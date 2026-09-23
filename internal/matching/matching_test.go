package matching_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
)

type classifierFunc func(context.Context, matching.Candidate, []matching.Criterion) ([]matching.Assessment, error)

func (f classifierFunc) Classify(ctx context.Context, c matching.Candidate, q []matching.Criterion) ([]matching.Assessment, error) {
	return f(ctx, c, q)
}

func TestRejectsIncompleteOrUngroundedProviderAnswers(t *testing.T) {
	for _, answers := range [][]matching.Assessment{
		{},
		{{CriterionID: "quiet", Assessment: "supported", Status: "evaluated", EvidenceRefs: []string{"invented"}}},
		{{CriterionID: "quiet", Assessment: "supported", Status: "evaluated"}},
	} {
		classifier := classifierFunc(func(context.Context, matching.Candidate, []matching.Criterion) ([]matching.Assessment, error) {
			return answers, nil
		})
		_, err := matching.New(classifier).Evaluate(context.Background(), matching.Request{Criteria: []matching.Criterion{{ID: "quiet", Priority: "primary"}}, Candidates: []matching.Candidate{{ID: "p", URL: "url"}}})
		if err == nil {
			t.Fatalf("accepted invalid answers: %+v", answers)
		}
	}
}

func TestProviderFailureCannotProduceACompleteRanking(t *testing.T) {
	classifier := classifierFunc(func(context.Context, matching.Candidate, []matching.Criterion) ([]matching.Assessment, error) {
		return nil, errors.New("unavailable")
	})
	_, err := matching.New(classifier).Evaluate(context.Background(), matching.Request{Criteria: []matching.Criterion{{ID: "quiet"}}, Candidates: []matching.Candidate{{ID: "p", URL: "url"}}})
	if err == nil {
		t.Fatal("provider failure was hidden")
	}
}

func TestBaselineDoesNotPromoteInferredAttributeOverPublishedConflict(t *testing.T) {
	result, err := matching.New(matching.Baseline{}).Evaluate(context.Background(), matching.Request{
		Criteria: []matching.Criterion{{ID: "pets", AttributeType: "pets_allowed", AttributeValue: "yes"}},
		Candidates: []matching.Candidate{{ID: "p", URL: "url", Evidence: []matching.Evidence{
			{ID: "claim", Text: "No mascotas", Provenance: "published", Type: "pets_allowed", Value: "no"},
			{ID: "inference", Text: "Mascotas inferidas por parser", Provenance: "inferred", Type: "pets_allowed", Value: "yes"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Matches[0].Assessments[0].Assessment != "contradicted" {
		t.Fatalf("published fact must dominate: %+v", result)
	}
}

func TestNoPreferencesUsesStableOrderWithoutCallingClassifier(t *testing.T) {
	c := classifierFunc(func(context.Context, matching.Candidate, []matching.Criterion) ([]matching.Assessment, error) {
		t.Fatal("no classification needed")
		return nil, nil
	})
	result, err := matching.New(c).Evaluate(context.Background(), matching.Request{Candidates: []matching.Candidate{{ID: "z", URL: "z"}, {ID: "a", URL: "a"}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Matches[0].CandidateID != "a" {
		t.Fatal("unstable tie ordering")
	}
}

func TestPrimaryEvidenceOutranksSecondaryEvidenceAndKeepsUnknowns(t *testing.T) {
	evaluator := matching.New(matching.Baseline{})
	result, err := evaluator.Evaluate(context.Background(), matching.Request{
		Criteria: []matching.Criterion{
			{ID: "quiet", Priority: "primary", AttributeType: "noise_level", AttributeValue: "quiet"},
			{ID: "light", Priority: "secondary", AttributeType: "natural_light", AttributeValue: "high"},
		},
		Candidates: []matching.Candidate{
			{ID: "bright", URL: "https://example.com/a", Evidence: []matching.Evidence{{ID: "e1", Type: "natural_light", Value: "high", Provenance: "stated", Text: "Muy luminoso"}}},
			{ID: "quiet", URL: "https://example.com/z", Evidence: []matching.Evidence{{ID: "e2", Type: "noise_level", Value: "quiet", Provenance: "stated", Text: "Silencioso"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) != 2 || result.Matches[0].CandidateID != "quiet" {
		t.Fatalf("primary preference must dominate: %+v", result)
	}
	got := result.Matches[0]
	if got.Rank != 1 || got.Assessments[1].Assessment != "insufficient_evidence" || got.Supported != 1 || got.Unknown != 1 {
		t.Fatalf("missing evidence must stay visible: %+v", got)
	}
}

func TestNeedsReviewSupportDoesNotOutrankEvaluatedSupport(t *testing.T) {
	classifier := classifierFunc(func(_ context.Context, c matching.Candidate, _ []matching.Criterion) ([]matching.Assessment, error) {
		status := "evaluated"
		if c.ID == "disputed" {
			status = "needs_review"
		}
		return []matching.Assessment{{CriterionID: "quiet", Assessment: "supported", Status: status, EvidenceRefs: []string{"e"}}}, nil
	})
	evidence := []matching.Evidence{{ID: "e", Text: "Silencioso", Provenance: "stated"}}
	result, err := matching.New(classifier).Evaluate(context.Background(), matching.Request{
		Criteria: []matching.Criterion{{ID: "quiet", Priority: "primary"}},
		// "disputed" sorts first by URL, so only support can move "clean" above it.
		Candidates: []matching.Candidate{{ID: "disputed", URL: "a", Evidence: evidence}, {ID: "clean", URL: "z", Evidence: evidence}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Matches[0].CandidateID != "clean" || result.Matches[1].Supported != 0 || result.Matches[1].NeedsReview != 1 {
		t.Fatalf("needs_review must add no support: %+v", result.Matches)
	}
}
