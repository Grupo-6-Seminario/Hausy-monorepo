package matching_test

import (
	"context"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
)

type recordedFit struct{}

func (recordedFit) Classify(_ context.Context, _ matching.Candidate, qs []matching.Criterion) ([]matching.Assessment, error) {
	answers := map[string]string{"light": "supported", "quiet": "contradicted", "balcony": "insufficient_evidence"}
	var out []matching.Assessment
	for _, q := range qs {
		a := matching.Assessment{CriterionID: q.ID, Assessment: answers[q.ID], Status: "evaluated"}
		if a.Assessment != "insufficient_evidence" {
			a.EvidenceRefs = []string{"description"}
		}
		out = append(out, a)
	}
	return out, nil
}

func TestFitRetainsWeightedNetCoverageAndRequiredContradiction(t *testing.T) {
	req := matching.FitRequest{Criteria: []matching.FitCriterion{
		{Criterion: matching.Criterion{ID: "light", Text: "luz"}, Strength: "preference", Weight: 2, SourceQuote: "priorizo luz", PrioritySource: "priorizo luz"},
		{Criterion: matching.Criterion{ID: "quiet", Text: "silencio"}, Strength: "preference"},
		{Criterion: matching.Criterion{ID: "balcony", Text: "balcón"}, Strength: "preference"},
	}, Candidates: []matching.Candidate{{ID: "a", URL: "a", Evidence: []matching.Evidence{{ID: "description", Text: "Luminoso y ruidoso", Provenance: "published"}}}}}
	result, err := matching.New(recordedFit{}).AssessFit(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	got := result.Matches[0]
	if got.Numerator != 1 || got.Denominator != 4 || got.Score != 0.25 || got.RequiredFit != "confirmed" || got.Contributions[0].Points != 2 || got.Contributions[1].Points != -1 || len(got.Contributions[0].Evidence) != 1 {
		t.Fatalf("fit=%+v", got)
	}
	req.Criteria[1].Strength = "requirement"
	result, err = matching.New(recordedFit{}).AssessFit(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Matches[0].RequiredFit != "contradicted" {
		t.Fatalf("fit=%+v", result.Matches[0])
	}
}

type fitProvider struct {
	answer matching.Assessment
	err    error
}

func (f fitProvider) Classify(_ context.Context, _ matching.Candidate, qs []matching.Criterion) ([]matching.Assessment, error) {
	a := f.answer
	a.CriterionID = qs[0].ID
	return []matching.Assessment{a}, f.err
}

func TestFitKeepsHintsConflictsReviewAndProviderFailuresDistinct(t *testing.T) {
	for _, tc := range []struct {
		name, provenance, assessment, status, want, required, failure string
		points                                                        int
		err                                                           error
	}{
		{name: "inferred support", provenance: "inferred", assessment: "supported", status: "evaluated", want: "hint", required: "unconfirmed"},
		{name: "missing", provenance: "published", assessment: "insufficient_evidence", status: "evaluated", want: "insufficient_evidence", required: "unconfirmed"},
		{name: "conflict", provenance: "published", assessment: "conflicting_evidence", status: "evaluated", want: "conflicting_evidence", required: "unconfirmed"},
		{name: "review", provenance: "published", assessment: "supported", status: "needs_review", want: "needs_review", required: "unconfirmed"},
		{name: "failed provider", provenance: "published", want: "unavailable", required: "unconfirmed", failure: "provider_error", err: context.DeadlineExceeded},
		{name: "invalid answer", provenance: "published", assessment: "invented", status: "evaluated", want: "unavailable", required: "unconfirmed", failure: "invalid_assessment"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := fitProvider{answer: matching.Assessment{Assessment: tc.assessment, Status: tc.status, EvidenceRefs: []string{"e"}}, err: tc.err}
			req := matching.FitRequest{Criteria: []matching.FitCriterion{{Criterion: matching.Criterion{ID: "light"}, Strength: "requirement"}}, Candidates: []matching.Candidate{{ID: "a", URL: "a", Evidence: []matching.Evidence{{ID: "e", Text: "Listing evidence", Provenance: tc.provenance}}}}}
			got, err := matching.New(provider).AssessFit(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			m := got.Matches[0]
			if m.RequiredFit != tc.required || m.Failure != tc.failure || m.Contributions[0].EffectiveAssessment != tc.want || m.Contributions[0].Points != tc.points {
				t.Fatalf("fit=%+v", m)
			}
		})
	}
}

func TestFitRejectsUntraceablePriorityAndPreservesNoPreferenceScore(t *testing.T) {
	candidate := matching.Candidate{ID: "a", URL: "a"}
	req := matching.FitRequest{Criteria: []matching.FitCriterion{{Criterion: matching.Criterion{ID: "light"}, Strength: "preference", Weight: 2}}, Candidates: []matching.Candidate{candidate}}
	if _, err := matching.New(nil).AssessFit(context.Background(), req); err == nil {
		t.Fatal("untraceable priority accepted")
	}
	req.Criteria = nil
	got, err := matching.New(nil).AssessFit(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	m := got.Matches[0]
	if m.Score != 0 || m.Numerator != 0 || m.Denominator != 0 || !m.NoPreferenceAdvantage || m.Failure != "" {
		t.Fatalf("fit=%+v", m)
	}
}

func TestFitUsesDirectExactAttributesWithoutAProvider(t *testing.T) {
	req := matching.FitRequest{Criteria: []matching.FitCriterion{{Criterion: matching.Criterion{ID: "furnished", AttributeType: "furnished", AttributeValue: "yes"}, Strength: "preference"}, {Criterion: matching.Criterion{ID: "balcony", AttributeType: "outdoor_space", AttributeValue: "balcon"}, Strength: "preference"}}, Candidates: []matching.Candidate{{ID: "a", URL: "a", Evidence: []matching.Evidence{{ID: "f", Text: "Amoblado", Type: "furnished", Value: "yes", Provenance: "stated"}, {ID: "b", Text: "Parece tener balcón", Type: "outdoor_space", Value: "balcon", Provenance: "inferred"}}}}}
	result, err := matching.New(nil).AssessFit(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	got := result.Matches[0]
	if got.Score != 0.5 || got.Numerator != 1 || got.Denominator != 2 || got.Failure != "" || got.Contributions[1].Points != 0 {
		t.Fatalf("fit=%+v", got)
	}
}

func TestFitRejectsQualitiesOutsideThePrototypeRubric(t *testing.T) {
	for _, q := range []matching.Criterion{{ID: "light", AttributeType: "natural_light", AttributeValue: "low"}, {ID: "noise", AttributeType: "noise_level", AttributeValue: "noisy"}} {
		if _, err := matching.New(nil).AssessFit(context.Background(), matching.FitRequest{Criteria: []matching.FitCriterion{{Criterion: q, Strength: "preference"}}}); err == nil {
			t.Fatalf("unsupported qualitative value accepted: %+v", q)
		}
	}
}
func TestFitDirectBooleanClaimOutranksInferredOpposite(t *testing.T) {
	req := matching.FitRequest{Criteria: []matching.FitCriterion{{Criterion: matching.Criterion{ID: "f", AttributeType: "furnished", AttributeValue: "yes"}, Strength: "preference"}}, Candidates: []matching.Candidate{{ID: "a", URL: "a", Evidence: []matching.Evidence{{ID: "direct", Text: "Amoblado", Type: "furnished", Value: "yes", Provenance: "stated"}, {ID: "inferred", Text: "Parece sin muebles", Type: "furnished", Value: "no", Provenance: "inferred"}}}}}
	got, err := matching.New(nil).AssessFit(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Matches[0].Score != 1 || got.Matches[0].Contributions[0].Assessment.Assessment != "supported" {
		t.Fatalf("direct fact lost: %+v", got.Matches[0])
	}
}
