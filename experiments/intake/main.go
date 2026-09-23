// Command intake scores the planners in internal/intake against labeled
// conversations. It is a spike runner: it calls models but never writes data.
//
//	go run ./experiments/intake -system=jev    # needs AI_GATEWAY_API_KEY
//	go run ./experiments/intake -system=qwen   # needs the local model
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/jev"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

type testCase struct {
	ID       string     `json:"id"`
	Turns    []string   `json:"turns"`
	Intent   string     `json:"intent"`
	Expect   [][]string `json:"expect"`
	Optional []string   `json:"optional"`
}

type outcome struct {
	ID         string     `json:"id"`
	Plan       []string   `json:"plan"`
	Intent     string     `json:"intent"`
	IntentOK   bool       `json:"intent_ok"`
	Missing    [][]string `json:"missing"`
	Extra      []string   `json:"extra"`
	Exact      bool       `json:"exact"`
	TurnMillis []int64    `json:"turn_ms"`
	Error      string     `json:"error,omitempty"`
}

func main() {
	system := flag.String("system", "jev", "jev or qwen")
	casesPath := flag.String("cases", "experiments/intake/cases.json", "labeled cases")
	only := flag.String("only", "", "run a single case id")
	flag.Parse()
	raw, err := os.ReadFile(*casesPath)
	if err != nil {
		fail(err)
	}
	var file struct {
		Cases []testCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		fail(err)
	}

	var source intake.Source
	switch *system {
	case "jev":
		client := jev.New(jev.GatewayURL, os.Getenv("AI_GATEWAY_API_KEY"), nil)
		// Patient retries measure planning accuracy apart from Gateway rate limits;
		// the app bounds latency with a budget and falls back instead.
		client.Retries, client.Backoff = 6, time.Second
		source = intake.Jev{Evaluate: client.Evaluate}
	case "qwen":
		source = intake.Qwen{Client: local.NewClient(envOr("LOCAL_LLM_URL", "http://127.0.0.1:8000"), os.Getenv("LOCAL_LLM_TOKEN"), envOr("LOCAL_LLM_MODEL", "Qwen3.5-9B-4bit"))}
	default:
		fail(fmt.Errorf("unknown system %q", *system))
	}
	// The real wrapper, so turn rules (frozen plan, default rent) apply as in the app.
	planner := intake.Planner{Primary: source}

	var results []outcome
	for _, c := range file.Cases {
		if *only != "" && c.ID != *only {
			continue
		}
		o := outcome{ID: c.ID}
		var plan intake.Plan
		for i := range c.Turns {
			start := time.Now()
			next, err := planner.Plan(context.Background(), c.Turns[:i+1], plan)
			o.TurnMillis = append(o.TurnMillis, time.Since(start).Milliseconds())
			if err != nil {
				o.Error = err.Error()
				break
			}
			plan = next
		}
		o.Plan, o.Intent = canonical(plan), plan.Intent
		o.IntentOK = o.Intent == c.Intent
		o.Missing, o.Extra, o.Exact = score(o.Plan, c)
		o.Exact = o.Exact && o.Error == ""
		fmt.Fprintf(os.Stderr, "%-26s exact=%-5v intent=%-5v missing=%v extra=%v ms=%v %s\n", c.ID, o.Exact, o.IntentOK, o.Missing, o.Extra, o.TurnMillis, o.Error)
		results = append(results, o)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]any{"system": *system, "cases": results})
}

// canonical flattens a plan into the label keys. Items of a plan with more
// than one branch are prefixed with their branch's neighborhoods.
func canonical(p intake.Plan) []string {
	set := map[string]bool{}
	if p.Sort != "" && p.Sort != "relevance" {
		set["sort="+p.Sort] = true
	}
	for fact, values := range p.Qualification {
		for _, v := range values {
			set["has:"+fact+"="+v] = true
		}
	}
	for _, b := range p.Branches {
		prefix := ""
		if len(p.Branches) > 1 {
			prefix = "branch[" + strings.Join(b.Neighborhoods, ",") + "]:"
		}
		for _, h := range b.Neighborhoods {
			set["hood="+h] = true
		}
		if b.Operation != "" {
			set["op="+b.Operation] = true
		}
		for _, item := range branchItems(b) {
			set[prefix+item] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func branchItems(b search.Query) []string {
	var out []string
	num := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	if b.MaxPrice != nil {
		out = append(out, "max_price="+num(*b.MaxPrice)+" "+b.Currency)
	}
	if b.MinPrice != nil {
		out = append(out, "min_price="+num(*b.MinPrice)+" "+b.Currency)
	}
	if b.MaxExpensesARS != nil {
		out = append(out, "max_expenses="+num(*b.MaxExpensesARS))
	}
	if b.MinTotalAreaM2 != nil {
		out = append(out, "min_area="+num(*b.MinTotalAreaM2))
	}
	for name, v := range map[string]*int{"min_rooms": b.MinRooms, "max_rooms": b.MaxRooms, "min_bedrooms": b.MinBedrooms, "min_bathrooms": b.MinBathrooms} {
		if v != nil {
			out = append(out, name+"="+strconv.Itoa(*v))
		}
	}
	for prefix, filters := range map[string][]search.AttributeFilter{"req:": b.RequiredAttributes, "pref:": b.PreferredAttributes, "excl:": b.ExcludedAttributes} {
		for _, f := range filters {
			out = append(out, prefix+f.Type+"="+f.Value)
		}
	}
	return out
}

// score matches each expected slot against one accepted alternative. Items the
// labels neither expect nor list as optional are extras.
func score(plan []string, c testCase) (missing [][]string, extra []string, exact bool) {
	have := map[string]bool{}
	for _, p := range plan {
		have[p] = true
	}
	used := map[string]bool{}
	for _, slot := range c.Expect {
		hit := false
		for _, alt := range slot {
			if have[alt] {
				used[alt], hit = true, true
				break
			}
		}
		if !hit {
			missing = append(missing, slot)
		}
	}
	for _, o := range c.Optional {
		used[o] = true
	}
	for _, p := range plan {
		if !used[p] {
			extra = append(extra, p)
		}
	}
	return missing, extra, len(missing) == 0 && len(extra) == 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
