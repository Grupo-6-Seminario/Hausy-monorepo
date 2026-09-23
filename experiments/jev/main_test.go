package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeGateway records every request body and fails each one with a
// non-retryable HTTP 400, so each case makes exactly one request.
func fakeGateway(t *testing.T) (*httptest.Server, *[]string) {
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.WriteHeader(400)
	}))
	t.Cleanup(server.Close)
	t.Setenv("AI_GATEWAY_API_KEY", "test-key")
	return server, &bodies
}

func TestProviderRequestsNeverContainCaseLabels(t *testing.T) {
	server, bodies := fakeGateway(t)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"-provider=jev", "-gateway=" + server.URL, "-inventory=../../data/listings.parsed.jsonl", "-cases=cases.json"}, &out); err != nil {
		t.Fatal(err)
	}
	if len(*bodies) == 0 {
		t.Fatal("no provider request was made")
	}
	for _, body := range *bodies {
		for _, label := range []string{"synthetic", "real_congreso_explicit_and_missing", "synthetic_conflict", "synthetic_injection", "synthetic_disposition_is_not_noise"} {
			if strings.Contains(body, label) {
				t.Errorf("request leaks case label %q", label)
			}
		}
	}
}

func TestFrozenSmokeCasesRunWithoutNetworkOnBaseline(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), []string{"-provider=baseline", "-inventory=../../data/listings.parsed.jsonl", "-cases=cases.json"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Provider string            `json:"provider"`
		Cases    []json.RawMessage `json:"cases"`
		Snapshot string            `json:"snapshot_sha256"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Provider != "baseline" || len(report.Cases) != 4 || len(report.Snapshot) != 64 {
		t.Fatalf("incomplete report: %s", out.String())
	}
}

func TestProviderFailureIsReportedPerCaseWithoutStopping(t *testing.T) {
	server, bodies := fakeGateway(t)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"-provider=jev", "-gateway=" + server.URL, "-inventory=../../data/listings.parsed.jsonl", "-cases=cases.json"}, &out); err != nil {
		t.Fatal(err)
	}
	var report struct {
		Cases []struct {
			Passed bool   `json:"passed"`
			Error  string `json:"error"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(*bodies) != 4 || len(report.Cases) != 4 {
		t.Fatalf("want 4 requests and 4 reported cases, got %d and %d", len(*bodies), len(report.Cases))
	}
	for _, c := range report.Cases {
		if c.Passed || !strings.Contains(c.Error, "HTTP 400") {
			t.Fatalf("failure not reported per case: %+v", c)
		}
	}
}

func TestNeedsReviewNeverCountsAsAPassedCase(t *testing.T) {
	// Overall answers say "insufficient_evidence" (the label for two cases) while
	// every record "supports": each assessment needs review.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		answers := map[string]any{}
		for id := range req.Questions {
			if strings.Contains(id, "_e") {
				answers[id] = map[string]any{"type": "choice", "choice": "supports", "probabilities": map[string]float64{"supports": 1, "opposes": 0, "irrelevant": 0, "both": 0}}
			} else {
				answers[id] = map[string]any{"type": "choice", "choice": "insufficient_evidence", "probabilities": map[string]float64{"supported": 0, "contradicted": 0, "insufficient_evidence": 1, "conflicting_evidence": 0}}
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"answers": answers})
	}))
	defer server.Close()
	t.Setenv("AI_GATEWAY_API_KEY", "test-key")
	var out bytes.Buffer
	if err := run(context.Background(), []string{"-provider=jev", "-gateway=" + server.URL, "-inventory=../../data/listings.parsed.jsonl", "-cases=cases.json"}, &out); err != nil {
		t.Fatal(err)
	}
	var report struct {
		Cases []struct {
			Passed bool   `json:"passed"`
			Error  string `json:"error"`
		} `json:"cases"`
	}
	json.Unmarshal(out.Bytes(), &report)
	for _, c := range report.Cases {
		if c.Passed || c.Error != "" {
			t.Fatalf("needs_review case passed or errored: %s", out.String())
		}
	}
}
