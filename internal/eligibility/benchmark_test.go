package eligibility_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

// searcher offers a guarantee that the 27 committed listings taking caución
// only refuse, so both benchmarks walk ineligible verdicts too.
var searcher = eligibility.Qualification{"guarantee": {"propietaria"}, "income_band": {"1000000-2000000"}}

// committedCandidates pairs the committed listings with the rules extracted
// from them, as the store hands them to the evaluator.
func committedCandidates(b *testing.B) []eligibility.Candidate {
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
	var candidates []eligibility.Candidate
	eachCommittedLine(b, "listings.parsed.jsonl", func(line []byte) error {
		var item listing.Listing
		err := json.Unmarshal(line, &item)
		candidates = append(candidates, eligibility.Candidate{Listing: item, Rules: rules[item.URL]})
		return err
	})
	return candidates
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

// BenchmarkAssess evaluates the searcher against every committed listing.
func BenchmarkAssess(b *testing.B) {
	candidates := committedCandidates(b)
	ineligible := 0
	for _, c := range candidates {
		if eligibility.Assess(searcher, c.Listing.Price, c.Rules, admissible).State == eligibility.Ineligible {
			ineligible++
		}
	}
	if ineligible == 0 {
		b.Fatal("the searcher should be ineligible for some committed listings")
	}
	for b.Loop() {
		for _, c := range candidates {
			eligibility.Assess(searcher, c.Listing.Price, c.Rules, admissible)
		}
	}
}

// BenchmarkRelaxations runs the zero-results engine over the whole committed
// inventory: which undeclared instrument would bring listings back.
func BenchmarkRelaxations(b *testing.B) {
	candidates := committedCandidates(b)
	if len(eligibility.Relaxations(searcher, candidates, admissible)) == 0 {
		b.Fatal("the committed inventory should offer a relaxation")
	}
	for b.Loop() {
		eligibility.Relaxations(searcher, candidates, admissible)
	}
}
