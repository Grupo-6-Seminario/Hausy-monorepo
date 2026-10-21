// Command clarification evaluates the live proposer against labeled search
// turns. It reads no database and writes only the report to stdout.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/clarification"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
)

type labeledCase struct {
	ID     string                    `json:"id"`
	Turns  []string                  `json:"turns"`
	Known  eligibility.Qualification `json:"known_qualification,omitempty"`
	Kind   string                    `json:"kind"`
	Source string                    `json:"source,omitempty"`
}

type result struct {
	ID            string `json:"id"`
	Expected      string `json:"expected"`
	Actual        string `json:"actual"`
	SourceMatched bool   `json:"source_matched"`
	Pass          bool   `json:"pass"`
	PlannerMS     int64  `json:"planner_ms"`
	ProposalMS    int64  `json:"proposal_ms"`
	Error         string `json:"error,omitempty"`
}

func main() {
	casesPath := flag.String("cases", "experiments/clarification/cases.json", "labeled cases")
	only := flag.String("only", "", "one case ID")
	debug := flag.Bool("debug", false, "print proposed questions for labeled fixtures")
	flag.Parse()
	raw, err := os.ReadFile(*casesPath)
	if err != nil {
		fail(err)
	}
	var file struct {
		Cases []labeledCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		fail(err)
	}
	client := local.NewClient(envOr("LOCAL_LLM_URL", "http://127.0.0.1:8000"), os.Getenv("LOCAL_LLM_TOKEN"), envOr("LOCAL_LLM_MODEL", "Qwen3.5-9B-4bit"))
	planner := intake.Planner{Primary: intake.Qwen{Client: client}}
	catalog := readCatalog()
	proposer := clarification.Model{Client: client}
	var results []result
	for _, c := range file.Cases {
		if *only != "" && c.ID != *only {
			continue
		}
		r := result{ID: c.ID, Expected: c.Kind}
		started := time.Now()
		plan, planErr := planner.Plan(context.Background(), c.Turns, intake.Plan{}, catalog)
		r.PlannerMS = time.Since(started).Milliseconds()
		problem := ""
		if planErr != nil {
			problem = planErr.Error()
		}
		started = time.Now()
		questions, err := proposer.Propose(context.Background(), c.Turns, plan, problem, c.Known, catalog)
		r.ProposalMS = time.Since(started).Milliseconds()
		if err != nil {
			r.Error = "proposal_error"
		} else {
			if *debug {
				fmt.Fprintf(os.Stderr, "plan=%+v problem=%q questions=%+v\n", plan, problem, questions)
			}
			r.Actual = "none"
			for _, question := range questions {
				if clarification.Validate(question, c.Turns, catalog) != nil {
					continue
				}
				r.Actual = question.Kind
				r.SourceMatched = c.Source == "" || strings.EqualFold(c.Source, question.Source)
				break
			}
			r.Pass = r.Actual == c.Kind && (c.Source == "" || r.SourceMatched)
		}
		fmt.Fprintf(os.Stderr, "%s expected=%s actual=%s pass=%v plan_ms=%d proposal_ms=%d\n", r.ID, r.Expected, r.Actual, r.Pass, r.PlannerMS, r.ProposalMS)
		results = append(results, r)
	}
	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": passed, "total": len(results), "cases": results})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

// readCatalog reads the qualification catalog the app serves, so the spike
// declares against the same facts and choices.
func readCatalog() eligibility.Catalog {
	ctx := context.Background()
	store, err := postgres.Open(ctx, envOr("DATABASE_URI", "postgresql://hausy:hausy@localhost:5432/hausy"))
	if err != nil {
		fail(err)
	}
	defer store.Close()
	catalog, err := store.Facts(ctx)
	if err != nil {
		fail(err)
	}
	return catalog
}
