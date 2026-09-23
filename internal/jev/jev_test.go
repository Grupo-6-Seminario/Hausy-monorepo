package jev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/jev"
)

var oneQuestion = map[string]jev.Question{"refund": {Type: "boolean", Instructions: "Is a refund requested?"}}

func client(url string) *jev.Client {
	c := jev.New(url, "secret-key", nil)
	c.Backoff = time.Millisecond
	return c
}

func TestEvaluateSendsTheGatewayRequestAndReturnsAnswers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model     string                     `json:"model"`
			State     string                     `json:"state"`
			Questions map[string]json.RawMessage `json:"questions"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if r.URL.Path != "/v1/evaluate" || r.Header.Get("Authorization") != "Bearer secret-key" || req.Model != "typesafe-ai/jev" || req.State != "charged twice" || len(req.Questions) != 1 {
			t.Errorf("wrong gateway request: %s %+v", r.URL.Path, req)
		}
		w.Write([]byte(`{"answers":{"refund":{"type":"boolean","probability":0.98}},"usage":{"inputTokens":275}}`))
	}))
	defer server.Close()
	got, err := client(server.URL).Evaluate(context.Background(), "charged twice", oneQuestion)
	if err != nil || got["refund"].Probability != 0.98 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestTransientFailuresAreRetriedThenSucceed(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader([]int{503, 429}[calls.Load()-1])
			return
		}
		w.Write([]byte(`{"answers":{"refund":{"type":"boolean","probability":0.5}}}`))
	}))
	defer server.Close()
	if _, err := client(server.URL).Evaluate(context.Background(), "s", oneQuestion); err != nil || calls.Load() != 3 {
		t.Fatalf("want success on the third attempt, got %v after %d calls", err, calls.Load())
	}
}

func TestClientErrorsAreNotRetriedAndNameTypeAndRequestIDOnly(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("X-Vercel-Id", "gru1::abc123")
		w.WriteHeader(403)
		w.Write([]byte(`{"error":{"message":"secret diagnostic","type":"customer_verification_required"}}`))
	}))
	defer server.Close()
	_, err := client(server.URL).Evaluate(context.Background(), "s", oneQuestion)
	if calls.Load() != 1 || err == nil || !strings.Contains(err.Error(), "customer_verification_required") || !strings.Contains(err.Error(), "gru1::abc123") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("want one call and a sanitized error, got %d calls: %v", calls.Load(), err)
	}
}

func TestFreeTextErrorTypesAreDropped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"type":"secret text with spaces"}}`))
	}))
	defer server.Close()
	if _, err := client(server.URL).Evaluate(context.Background(), "s", oneQuestion); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("free text leaked: %v", err)
	}
}

func TestMissingAnswersAreAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"answers":{}}`)) }))
	defer server.Close()
	if _, err := client(server.URL).Evaluate(context.Background(), "s", oneQuestion); err == nil {
		t.Fatal("accepted a response without every answer")
	}
}

func TestTransportErrorsKeepTheirCauseAndCancellationWins(t *testing.T) {
	_, err := client("http://127.0.0.1:1").Evaluate(context.Background(), "s", oneQuestion)
	if err == nil || !strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("transport cause lost or leaked: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client("http://127.0.0.1:1").Evaluate(ctx, "s", oneQuestion); err != context.Canceled {
		t.Fatalf("cancellation lost: %v", err)
	}
}
