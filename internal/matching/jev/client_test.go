package jev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching/jev"
)

func TestGatewayClassifiesWithReferencesToSuppliedEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/evaluate" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("incorrect gateway request")
		}
		var request struct {
			Model     string                     `json:"model"`
			Questions map[string]json.RawMessage `json:"questions"`
			State     json.RawMessage            `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Model != "typesafe-ai/jev" || len(request.Questions) != 2 || !strings.Contains(string(request.State), "Muy luminoso") {
			t.Errorf("incomplete evaluation request: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"answers":{"q0":{"type":"choice","choice":"supported","probabilities":{"supported":0.95,"contradicted":0.01,"insufficient_evidence":0.03,"conflicting_evidence":0.01}},"q0_e0":{"type":"choice","choice":"supports","probabilities":{"supports":1,"opposes":0,"irrelevant":0,"both":0}}}}`))
	}))
	defer server.Close()
	evaluator := matching.New(jev.New(server.URL, "test-key", server.Client()))
	result, err := evaluator.Evaluate(context.Background(), matching.Request{Criteria: []matching.Criterion{{ID: "light", Text: "Buena luz natural"}}, Candidates: []matching.Candidate{{ID: "p", URL: "url", Evidence: []matching.Evidence{{ID: "e", Text: "Muy luminoso", Provenance: "stated"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	a := result.Matches[0].Assessments[0]
	if a.Assessment != "supported" || len(a.EvidenceRefs) != 1 || a.EvidenceRefs[0] != "e" {
		t.Fatalf("wrong grounded assessment: %+v", a)
	}
}

func TestOverallAnswerDisagreeingWithEvidenceNeedsReview(t *testing.T) {
	// Overall "supported", but the only record opposes: evidence implies "contradicted".
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"answers":{"q0":{"type":"choice","choice":"supported","probabilities":{"supported":0.8,"contradicted":0.1,"insufficient_evidence":0.05,"conflicting_evidence":0.05}},"q0_e0":{"type":"choice","choice":"opposes","probabilities":{"supports":0.1,"opposes":0.9,"irrelevant":0,"both":0}}}}`))
	}))
	defer server.Close()
	result, err := matching.New(jev.New(server.URL, "key", server.Client())).Evaluate(context.Background(), matching.Request{
		Criteria:   []matching.Criterion{{ID: "light", Text: "Buena luz natural"}},
		Candidates: []matching.Candidate{{ID: "p", URL: "url", Evidence: []matching.Evidence{{ID: "e", Text: "Interno", Provenance: "stated"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	a := result.Matches[0].Assessments[0]
	if a.Status != "needs_review" || a.Assessment != "supported" || a.EvidenceAssessment != "contradicted" || result.Matches[0].Supported != 0 || result.Matches[0].NeedsReview != 1 {
		t.Fatalf("disagreement must be kept for review, not counted: %+v", result.Matches[0])
	}
}

func TestSupportedWithoutEvidenceNeedsReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"answers":{"q0":{"type":"choice","choice":"supported","probabilities":{"supported":1,"contradicted":0,"insufficient_evidence":0,"conflicting_evidence":0}}}}`))
	}))
	defer server.Close()
	got, err := jev.New(server.URL, "key", server.Client()).Classify(context.Background(), matching.Candidate{ID: "p", URL: "url"}, []matching.Criterion{{ID: "q", Text: "Quiet"}})
	if err != nil || got[0].Status != "needs_review" || got[0].EvidenceAssessment != "insufficient_evidence" {
		t.Fatalf("ungrounded support must need review: %+v, %v", got, err)
	}
}

func TestAcceptsProbabilitiesRoundedToTwoDecimals(t *testing.T) {
	// Sums of 1.01 and 0.99 are what two-decimal rounding of four options produces.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"answers":{"q0":{"type":"choice","choice":"supported","probabilities":{"supported":0.54,"contradicted":0.44,"insufficient_evidence":0.02,"conflicting_evidence":0.01}},"q0_e0":{"type":"choice","choice":"supports","probabilities":{"supports":0.33,"opposes":0.33,"irrelevant":0.33,"both":0}}}}`))
	}))
	defer server.Close()
	result, err := matching.New(jev.New(server.URL, "key", server.Client())).Evaluate(context.Background(), matching.Request{
		Criteria:   []matching.Criterion{{ID: "light", Text: "Buena luz natural"}},
		Candidates: []matching.Candidate{{ID: "p", URL: "url", Evidence: []matching.Evidence{{ID: "e", Text: "Luminoso", Provenance: "stated"}}}},
	})
	if err != nil {
		t.Fatalf("rounded distribution rejected: %v", err)
	}
	if a := result.Matches[0].Assessments[0]; a.Status != "evaluated" || a.Assessment != "supported" {
		t.Fatalf("wrong assessment: %+v", a)
	}
}

func TestGatewayRejectsMissingAnswersAndInvalidDistributions(t *testing.T) {
	for _, body := range []string{
		`{"answers":{}}`,
		`{"answers":{"q0":{"type":"choice","choice":"insufficient_evidence","probabilities":{"supported":0,"contradicted":0,"insufficient_evidence":0.4,"conflicting_evidence":0}}}}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer server.Close()
			_, err := jev.New(server.URL, "key", server.Client()).Classify(context.Background(), matching.Candidate{ID: "p", URL: "url"}, []matching.Criterion{{ID: "q", Text: "Quiet"}})
			if err == nil {
				t.Fatal("accepted invalid response")
			}
		})
	}
}
