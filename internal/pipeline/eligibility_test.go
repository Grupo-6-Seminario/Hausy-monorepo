package pipeline_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/pipeline"
)

func TestExtractEligibility_WritesOneRecordPerExaminedListing(t *testing.T) {
	input := strings.Join([]string{
		`{"url":"a","description":"Garantía propietaria"}`,
		`{"url":"b","description":"Luminoso"}`,
		`{"url":"c","description":"boom"}`,
		`{"url":"done","description":"Garantía propietaria"}`,
	}, "\n")
	extract := func(_ context.Context, description string) ([]eligibility.Rule, error) {
		switch description {
		case "Garantía propietaria":
			return []eligibility.Rule{{Fact: "guarantee", Operator: "one_of", Values: []string{"propietaria"}, Hardness: "hard"}}, nil
		case "boom":
			return nil, errors.New("gateway down")
		}
		return nil, nil
	}
	var out bytes.Buffer
	report, err := pipeline.ExtractEligibility(context.Background(), strings.NewReader(input), &out, extract, map[string]bool{"done": true})
	if err != nil {
		t.Fatal(err)
	}
	var records []pipeline.EligibilityRecord
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var r pipeline.EligibilityRecord
		json.Unmarshal([]byte(line), &r)
		records = append(records, r)
	}
	// "c" failed and is not written, so a resumed run retries it.
	if len(records) != 2 || records[0].URL != "a" || len(records[0].Rules) != 1 || records[1].URL != "b" || len(records[1].Rules) != 0 || records[0].ExtractedAt.IsZero() {
		t.Fatalf("got %s", out.String())
	}
	if report.Parsed != 2 || report.Failed != 1 || report.AlreadyDone != 1 {
		t.Fatalf("got %+v", report)
	}
}

type fakeEligibilitySink struct{ saved map[string][]eligibility.Rule }

func (f *fakeEligibilitySink) SaveEligibility(_ context.Context, url string, rules []eligibility.Rule) error {
	if url == "missing" {
		return errors.New("unknown listing")
	}
	f.saved[url] = rules
	return nil
}

func TestLoadEligibility_SavesEveryRecordAndReportsFailures(t *testing.T) {
	input := `{"url":"a","rules":[{"fact":"guarantee","operator":"one_of","values":["caucion"],"hardness":"hard"}]}
{"url":"b","rules":[]}
not json
{"url":"missing","rules":[]}`
	sink := &fakeEligibilitySink{saved: map[string][]eligibility.Rule{}}
	report, err := pipeline.LoadEligibility(context.Background(), strings.NewReader(input), sink)
	if err != nil {
		t.Fatal(err)
	}
	if report.Loaded != 2 || report.Failed != 2 || len(sink.saved["a"]) != 1 || sink.saved["b"] == nil {
		t.Fatalf("got %+v, saved %v", report, sink.saved)
	}
}
