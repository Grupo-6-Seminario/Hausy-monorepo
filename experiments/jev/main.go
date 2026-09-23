// Command jev runs independently labeled smoke cases. It never changes inventory.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching/jev"
)

type expectation struct {
	Criterion  string `json:"criterion"`
	Assessment string `json:"assessment"`
}
type smokeCase struct {
	ID           string        `json:"id"`
	Kind         string        `json:"kind"`
	URL          string        `json:"source_url"`
	Description  string        `json:"description"`
	Expectations []expectation `json:"expectations"`
}
type outcome struct {
	ID           string           `json:"id"`
	Expected     []expectation    `json:"expected"`
	Result       *matching.Result `json:"result,omitempty"`
	Passed       bool             `json:"passed"`
	Error        string           `json:"error,omitempty"`
	Milliseconds int64            `json:"elapsed_ms"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("jev-smoke", flag.ContinueOnError)
	provider := flags.String("provider", "baseline", "baseline or jev; Jev makes billable Gateway requests")
	inventory := flags.String("inventory", "data/listings.parsed.jsonl", "frozen inventory JSONL")
	casesPath := flags.String("cases", "experiments/jev/cases.json", "independent smoke labels")
	gateway := flags.String("gateway", jev.GatewayURL, "Gateway base URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	var classifier matching.Classifier = matching.Baseline{}
	switch *provider {
	case "baseline":
	case "jev":
		key := os.Getenv("AI_GATEWAY_API_KEY")
		if key == "" {
			return fmt.Errorf("AI_GATEWAY_API_KEY is required in the process environment")
		}
		classifier = jev.New(*gateway, key, nil)
	default:
		return fmt.Errorf("unknown provider")
	}
	raw, err := os.ReadFile(*inventory)
	if err != nil {
		return err
	}
	rows := map[string]listing.Listing{}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		var row listing.Listing
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return err
		}
		rows[row.URL] = row
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	rawCases, err := os.ReadFile(*casesPath)
	if err != nil {
		return err
	}
	var fixtures struct {
		Cases []smokeCase `json:"cases"`
	}
	if err := json.Unmarshal(rawCases, &fixtures); err != nil {
		return err
	}
	report := struct {
		Provider string    `json:"provider"`
		Snapshot string    `json:"snapshot_sha256"`
		Cases    []outcome `json:"cases"`
	}{Provider: *provider, Snapshot: fmt.Sprintf("%x", sha256.Sum256(raw)), Cases: []outcome{}}
	evaluator := matching.New(classifier)
	for n, test := range fixtures.Cases {
		// Case IDs describe the expected answer; the provider only sees opaque IDs.
		id := fmt.Sprintf("c%d", n+1)
		candidate := matching.Candidate{ID: id, URL: "urn:hausy:" + id}
		description := test.Description
		if test.Kind == "real_listing" {
			row, ok := rows[test.URL]
			if !ok {
				return fmt.Errorf("fixture listing not found: %s", test.ID)
			}
			description = row.Description
			candidate.URL = row.URL
			for i, a := range row.Attributes {
				if a.Evidence != "" {
					candidate.Evidence = append(candidate.Evidence, matching.Evidence{ID: fmt.Sprintf("attr%d", i), Type: a.Type, Value: a.Value, Text: a.Evidence, Provenance: string(a.Provenance)})
				}
			}
		}
		for i, text := range strings.Split(description, "\n\n") {
			if strings.TrimSpace(text) != "" {
				candidate.Evidence = append(candidate.Evidence, matching.Evidence{ID: fmt.Sprintf("paragraph%d", i), Text: text, Provenance: "stated"})
			}
		}
		var criteria []matching.Criterion
		for i, e := range test.Expectations {
			q := matching.Criterion{ID: fmt.Sprintf("c%d", i), Text: e.Criterion, Priority: "primary"}
			switch e.Criterion {
			case "Buena luz natural":
				q.AttributeType = "natural_light"
				q.AttributeValue = "high"
			case "Permite mascotas":
				q.AttributeType = "pets_allowed"
				q.AttributeValue = "yes"
			case "Ambiente silencioso":
				q.AttributeType = "noise_level"
				q.AttributeValue = "quiet"
			case "Apto para uso profesional":
				q.AttributeType = "suitable_for"
				q.AttributeValue = "profesional"
			}
			criteria = append(criteria, q)
		}
		start := time.Now()
		result, err := evaluator.Evaluate(ctx, matching.Request{Criteria: criteria, Candidates: []matching.Candidate{candidate}})
		item := outcome{ID: test.ID, Expected: test.Expectations, Milliseconds: time.Since(start).Milliseconds(), Passed: err == nil}
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Result = &result
			for i, a := range result.Matches[0].Assessments {
				if a.Status != "evaluated" || a.Assessment != test.Expectations[i].Assessment {
					item.Passed = false
				}
			}
		}
		report.Cases = append(report.Cases, item)
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
