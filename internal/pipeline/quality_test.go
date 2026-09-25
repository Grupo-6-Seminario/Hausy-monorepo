package pipeline_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/pipeline"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
)

type reviewSink struct{ got map[string]quality.Review }

func (s *reviewSink) SaveQualityRecord(_ context.Context, record quality.Record) error {
	s.got[record.URL] = record.Review
	return nil
}

func TestLoadQualityRestoresReviewedListingsWithoutAuditingAgain(t *testing.T) {
	input := strings.NewReader("{\"url\":\"a\",\"content_sha256\":\"hash-a\",\"review\":{\"status\":\"passed\"}}\n{\"url\":\"b\",\"content_sha256\":\"hash-b\",\"review\":{\"status\":\"withheld\",\"conflicts\":[{\"first\":\"luminoso\",\"second\":\"poca luz\",\"detail\":\"natural_light\"}]}}\n")
	sink := &reviewSink{got: map[string]quality.Review{}}
	report, err := pipeline.LoadQuality(context.Background(), input, sink)
	if err != nil || report.Loaded != 2 || sink.got["a"].Status != quality.Passed || len(sink.got["b"].Conflicts) != 1 {
		t.Fatalf("load review = %+v, report = %+v, err = %v", sink.got, report, err)
	}
}
