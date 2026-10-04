package comparison_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	comparison "github.com/Grupo-6-Seminario/proyecto-angus-back/experiments/property-matching"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
)

// Both paths run the same fixed cases, plans, inventory and recorded answers.
// No planner inference, SQL, provider inference or generated writer is timed.
func BenchmarkComparison(b *testing.B) {
	cases, _, err := comparison.Load("../..", "fixtures/cases.json")
	if err != nil {
		b.Fatal(err)
	}
	ctx := logging.WithLogger(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, id := range []string{"weighted-net", "mandatory-alternatives", "branch-specific-shared", "snapshot-direct-light"} {
		var c comparison.Case
		for _, item := range cases {
			if item.ID == id {
				c = item
			}
		}
		b.Run(id+"/baseline", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, _, err := comparison.RunBaseline(ctx, c); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(id+"/proposed", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				r, err := comparison.RunProposed(ctx, c)
				if err != nil {
					b.Fatal(err)
				}
				_ = r.Project()
				_ = r.Fallback("", "")
			}
		})
	}
}
