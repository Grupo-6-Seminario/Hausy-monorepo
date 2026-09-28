package pipeline_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/pipeline"
)

// Benchmarks read the committed inventory under data/, so every snapshot in
// docs/metrics measures the same 300 listings.
func committed(b *testing.B, name string) []byte {
	b.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "data", name))
	if err != nil {
		b.Fatal(err)
	}
	return data
}

type discardSink struct{}

func (discardSink) Save(context.Context, listing.Listing) error { return nil }

// BenchmarkParse normalizes the committed scrape with the model stubbed out:
// the deterministic half of `listings parse`, the part each new portal adds to.
func BenchmarkParse(b *testing.B) {
	raw := committed(b, "listings.jsonl")
	parse := func() pipeline.Report {
		report, err := pipeline.Parse(context.Background(), bytes.NewReader(raw), io.Discard, &fakeParser{}, nil)
		if err != nil {
			b.Fatal(err)
		}
		return report
	}
	if report := parse(); report.Parsed == 0 || report.Failed > 0 {
		b.Fatalf("the committed scrape should parse cleanly: %+v", report)
	}
	for b.Loop() {
		parse()
	}
}

// BenchmarkLoad reads the committed parsed listings the way `listings load`
// does, into a sink that keeps nothing, so no database time is counted.
func BenchmarkLoad(b *testing.B) {
	parsed := committed(b, "listings.parsed.jsonl")
	load := func() pipeline.Report {
		report, err := pipeline.Load(context.Background(), bytes.NewReader(parsed), discardSink{})
		if err != nil {
			b.Fatal(err)
		}
		return report
	}
	if report := load(); report.Loaded == 0 || report.Failed > 0 {
		b.Fatalf("the committed listings should load cleanly: %+v", report)
	}
	for b.Loop() {
		load()
	}
}
