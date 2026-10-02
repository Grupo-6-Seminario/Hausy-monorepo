package buyer_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

// neighborhoodStock serves every committed listing of the queried
// neighborhoods and applies no other filter, so the turn ranks the most that
// SQL could hand it, without a database.
type neighborhoodStock map[string][]eligibility.Candidate

func (s neighborhoodStock) Candidates(_ context.Context, q search.Query) ([]eligibility.Candidate, error) {
	var out []eligibility.Candidate
	for _, n := range q.Neighborhoods {
		out = append(out, s[n]...)
	}
	return out, nil
}

func (neighborhoodStock) Facts(context.Context) (eligibility.Catalog, error) {
	return eligibility.Catalog{"guarantee": {Admissible: true}, "income_band": {Admissible: true}, "caucion_quoted": {Admissible: true}}, nil
}

type cannedWriter struct{}

func (cannedWriter) Write(context.Context, buyer.Packet, func(string)) (string, error) {
	return "canned reply", nil
}

func committedStock(b *testing.B) neighborhoodStock {
	b.Helper()
	rules := map[string][]eligibility.Rule{}
	eachCommittedLine(b, "listings.eligibility.jsonl", func(line []byte) error {
		var record struct {
			URL   string             `json:"url"`
			Rules []eligibility.Rule `json:"rules"`
		}
		err := json.Unmarshal(line, &record)
		rules[record.URL] = record.Rules
		return err
	})
	stock := neighborhoodStock{}
	eachCommittedLine(b, "listings.parsed.jsonl", func(line []byte) error {
		var item listing.Listing
		err := json.Unmarshal(line, &item)
		stock[item.Neighborhood] = append(stock[item.Neighborhood], eligibility.Candidate{Listing: item, Rules: rules[item.URL]})
		return err
	})
	return stock
}

func eachCommittedLine(b *testing.B, name string, fn func([]byte) error) {
	b.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "data", name))
	if err != nil {
		b.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		if err := fn(scanner.Bytes()); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTurn is one buyer turn of a new conversation with the planner and
// the writer stubbed: everything between the two model calls, over the 200
// committed listings of palermo and congreso.
func BenchmarkTurn(b *testing.B) {
	stock := committedStock(b)
	plan := intake.Plan{Intent: "new_search", PlannedBy: "benchmark", Branches: []search.Query{{
		Neighborhoods: []string{"palermo", "congreso"},
		Operation:     "alquiler",
		PreferredAttributes: []search.AttributeFilter{
			{Type: "outdoor_space", Value: "balcon"},
			{Type: "noise_level", Value: "quiet"},
		},
	}}}
	ctx := logging.WithLogger(context.Background(), slog.New(slog.DiscardHandler))
	turn := func() *buyer.TurnResponse {
		agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, stock, cannedWriter{})
		response, err := agent.HandleMessage(ctx, "benchmark", "dos ambientes en palermo o congreso, con balcón y tranquilo",
			eligibility.Qualification{"guarantee": {"propietaria"}}, buyer.Events{})
		if err != nil {
			b.Fatal(err)
		}
		return response
	}
	if response := turn(); len(response.Listings) == 0 || len(response.Relaxations) == 0 {
		b.Fatalf("the committed inventory should rank listings and offer a relaxation: %+v", response)
	}
	for b.Loop() {
		turn()
	}
}
