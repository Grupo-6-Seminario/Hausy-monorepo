package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/pipeline"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

// BenchmarkCandidates is the hard-filter query of a buyer turn over the
// committed inventory, loaded the way `listings load` loads it. Like every
// Postgres test it needs HAUSY_TEST_DATABASE_URI and skips without it.
func BenchmarkCandidates(b *testing.B) {
	store := openTestStore(b)
	ctx := context.Background()
	for _, step := range []struct {
		file string
		load func(*os.File) (pipeline.Report, error)
	}{
		{"listings.parsed.jsonl", func(f *os.File) (pipeline.Report, error) { return pipeline.Load(ctx, f, store) }},
		{"listings.eligibility.jsonl", func(f *os.File) (pipeline.Report, error) { return pipeline.LoadEligibility(ctx, f, store) }},
		{"listings.quality.jsonl", func(f *os.File) (pipeline.Report, error) { return pipeline.LoadQuality(ctx, f, store) }},
	} {
		file, err := os.Open(filepath.Join("..", "..", "..", "data", step.file))
		if err != nil {
			b.Fatal(err)
		}
		report, err := step.load(file)
		file.Close()
		if err != nil || report.Loaded == 0 {
			b.Fatalf("load %s: %v %+v", step.file, err, report)
		}
	}
	maxPrice := 900000.0
	query := search.Query{Neighborhoods: []string{"palermo", "congreso"}, Operation: "alquiler", Currency: "ARS", MaxPrice: &maxPrice}
	candidates, err := store.Candidates(ctx, query)
	if err != nil || len(candidates) == 0 {
		b.Fatalf("the committed inventory should match the query: %v, %d candidates", err, len(candidates))
	}
	for b.Loop() {
		if _, err := store.Candidates(ctx, query); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(candidates)), "candidates")
}
