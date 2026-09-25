package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
)

type QualitySink interface {
	SaveQualityRecord(context.Context, quality.Record) error
}

// LoadQuality restores the committed, non-reproducible review decisions.
func LoadQuality(ctx context.Context, input io.Reader, sink QualitySink) (Report, error) {
	var report Report
	scanner := scannerFor(input)
	for line := 1; scanner.Scan(); line++ {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var record quality.Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			report.fail(fmt.Errorf("quality line %d: %w", line, err))
			continue
		}
		if record.URL == "" || record.ContentSHA256 == "" {
			report.fail(fmt.Errorf("quality line %d: missing URL or fingerprint", line))
			continue
		}
		if err := sink.SaveQualityRecord(ctx, record); err != nil {
			report.fail(err)
			continue
		}
		report.Loaded++
	}
	return report, scanner.Err()
}
