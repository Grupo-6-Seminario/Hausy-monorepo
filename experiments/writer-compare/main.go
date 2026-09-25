// Command writer-compare runs real search turns (local planner, Postgres,
// eligibility) and gives the same writer packet to three writers: the template,
// the local model and Bedrock. It flags amenity and guarantee mentions the
// packet does not support, and prints replies and latency. It writes nothing.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/bedrock"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
)

var searches = []string{
	"Busco un dos ambientes en Palermo hasta 900 mil pesos",
	"Departamento con pileta en Palermo para alquilar",
	"Monoambiente en Congreso, tengo garantía propietaria",
	"Tres ambientes en Monserrat con balcón, hasta un millón",
	"Busco alquilar en Congreso, tengo seguro de caución",
	"Dos ambientes luminoso en Palermo con gimnasio",
	"Algo barato en Monserrat para un estudiante",
	"Departamento con cochera en Palermo",
}

var guaranteeTerm = regexp.MustCompile(`(?i)garant|cauci[oó]n|finaer|hoggax|propietaria`)

// capture keeps the packet and fails, so the pipeline answers with its
// template: the template reply comes back on the turn response.
type capture struct{ packet *buyer.Packet }

func (c *capture) Write(_ context.Context, p buyer.Packet, _ func(string)) (string, error) {
	c.packet = &p
	return "", errors.New("captured")
}

type reply struct {
	Writer     string   `json:"writer"`
	MS         int64    `json:"ms"`
	Text       string   `json:"text"`
	Ungrounded []string `json:"ungrounded,omitempty"`
	Error      string   `json:"error,omitempty"`
}

func main() {
	only := flag.Int("only", -1, "run one search by index")
	flag.Parse()
	ctx := context.Background()
	model := envOr("LOCAL_LLM_MODEL", "Qwen3.5-9B-4bit")
	localClient := local.NewClient(envOr("LOCAL_LLM_URL", "http://127.0.0.1:8000"), os.Getenv("LOCAL_LLM_TOKEN"), model)
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		fail(err)
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	writers := []struct {
		name   string
		writer buyer.Writer
	}{
		{"local", buyer.LocalWriter{Client: localClient}},
		{"bedrock", buyer.LocalWriter{Client: bedrock.New(bedrockruntime.NewFromConfig(cfg), envOr("BEDROCK_MODEL_ID", "us.anthropic.claude-sonnet-4-6"))}},
	}
	store, err := postgres.Open(ctx, envOr("DATABASE_URI", "postgresql://hausy:hausy@localhost:5432/hausy"))
	if err != nil {
		fail(err)
	}
	defer store.Close()

	out := json.NewEncoder(os.Stdout)
	out.SetEscapeHTML(false)
	for i, message := range searches {
		if *only >= 0 && i != *only {
			continue
		}
		c := &capture{}
		agent := buyer.NewAgent(nil, buyer.WithPipeline(intake.Planner{Primary: intake.Qwen{Client: localClient}}, store, c))
		turn, err := agent.HandleMessage(ctx, fmt.Sprintf("writer-compare-%d", i), message, nil, buyer.Events{})
		if err != nil || c.packet == nil {
			fmt.Fprintf(os.Stderr, "%d: no packet: %v\n", i, err)
			continue
		}
		replies := []reply{{Writer: "template", Text: turn.Reply, Ungrounded: ungrounded(turn.Reply, *c.packet)}}
		for _, w := range writers {
			started := time.Now()
			text, err := w.writer.Write(ctx, *c.packet, nil)
			r := reply{Writer: w.name, MS: time.Since(started).Milliseconds(), Text: text, Ungrounded: ungrounded(text, *c.packet)}
			if err != nil {
				r.Error = err.Error()
			}
			replies = append(replies, r)
		}
		var shown []string
		for _, s := range c.packet.Shown {
			var amenities []string
			for _, a := range s.Attributes {
				if a.Type == "amenity" {
					amenities = append(amenities, a.Value)
				}
			}
			price := 0.0
			if s.Price.Amount != nil {
				price = *s.Price.Amount
			}
			state := ""
			if s.Eligibility != nil {
				state = string(s.Eligibility.State)
			}
			rooms := 0
			if s.Rooms != nil {
				rooms = *s.Rooms
			}
			shown = append(shown, fmt.Sprintf("#%d %s %.0f %s, %d amb, amenities=%v, %s", s.Rank, s.Address, price, s.Price.Currency, rooms, amenities, state))
		}
		_ = out.Encode(map[string]any{"search": message, "shown": shown, "replies": replies})
		for _, r := range replies {
			fmt.Fprintf(os.Stderr, "%d %-8s ms=%-6d ungrounded=%v err=%v\n", i, r.Writer, r.MS, r.Ungrounded, r.Error != "")
		}
	}
}

// ungrounded lists amenity and guarantee mentions that neither the search nor
// a shown listing supports.
func ungrounded(text string, p buyer.Packet) []string {
	var evidence strings.Builder
	evidence.WriteString(p.Question)
	for _, r := range p.Requirements {
		fmt.Fprintf(&evidence, " %s=%s", r.Type, r.Value)
	}
	for _, s := range p.Shown {
		evidence.WriteString(" " + s.Description)
		for _, a := range s.Attributes {
			fmt.Fprintf(&evidence, " %s %s", a.Value, a.Evidence)
		}
		if s.Eligibility != nil {
			for _, c := range s.Eligibility.Conditions {
				evidence.WriteString(" guarantee " + c.Rule.Evidence)
			}
			for _, m := range s.Eligibility.Met {
				evidence.WriteString(" guarantee " + m.Evidence)
			}
		}
	}
	grounds := evidence.String()
	var out []string
	for _, value := range listing.Vocabulary["amenity"] {
		if listing.NamesAmenity(value, text) && !listing.NamesAmenity(value, grounds) && !strings.Contains(grounds, value) {
			out = append(out, "amenity:"+value)
		}
	}
	if guaranteeTerm.MatchString(text) && !guaranteeTerm.MatchString(grounds) {
		out = append(out, "guarantee")
	}
	// Every amount must be a shown price, expensas, their sum, or the
	// searcher's own number.
	known := map[string]bool{}
	for _, m := range amount.FindAllStringSubmatch(p.Question, -1) {
		known[digitsOnly(m[2])] = true
	}
	for _, s := range p.Shown {
		price, expenses := 0.0, 0.0
		if s.Price.Amount != nil {
			price = *s.Price.Amount
		}
		if s.Expenses.Amount != nil {
			expenses = *s.Expenses.Amount
		}
		for _, v := range []float64{price, expenses, price + expenses} {
			known[fmt.Sprintf("%.0f", v)] = true
		}
	}
	for _, m := range amount.FindAllStringSubmatch(text, -1) {
		if d := digitsOnly(m[2]); d != "" && !known[d] {
			out = append(out, "amount:"+m[0])
		}
	}
	return out
}

var amount = regexp.MustCompile(`(\$|USD|u\$s)\s?(\d[\d.]*)`)

func digitsOnly(s string) string { return strings.NewReplacer(".", "", ",", "").Replace(s) }

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
