package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
)

type recordingAgent struct {
	sessionID string
	message   string
}

func (a *recordingAgent) HandleMessage(_ context.Context, sessionID, message string, _ eligibility.Qualification, _ buyer.Events) (*buyer.TurnResponse, error) {
	a.sessionID = sessionID
	a.message = message
	return &buyer.TurnResponse{Reply: "Respuesta real del agente."}, nil
}

func TestHandler_PostMessageReturnsAgentReply(t *testing.T) {
	agent := &recordingAgent{}
	handler := httpapi.NewHandler(agent, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), nil)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/messages",
		strings.NewReader(`{"session_id":"browser-session","message":"Busco mucha luz natural"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if agent.sessionID != "browser-session" || agent.message != "Busco mucha luz natural" {
		t.Fatalf("agent received session %q and message %q", agent.sessionID, agent.message)
	}
	var payload buyer.TurnResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Reply != "Respuesta real del agente." {
		t.Fatalf("expected agent reply, got %q", payload.Reply)
	}
}

func TestHandlerLogsRequestWithCorrelationWithoutMessageContent(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	handler := httpapi.NewHandler(&recordingAgent{}, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), nil)
	request := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(`{"session_id":"private-session","message":"private search text"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	id := response.Header().Get("X-Request-ID")
	if len(id) != 32 {
		t.Fatalf("expected a 32-character request ID, got %q", id)
	}
	var event map[string]any
	if err := json.Unmarshal(logs.Bytes(), &event); err != nil {
		t.Fatalf("expected one JSON request log: %v: %s", err, logs.String())
	}
	if event["msg"] != "http_request" || event["request_id"] != id || event["status"] != float64(200) || event["outcome"] != "success" {
		t.Fatalf("unexpected request log: %v", event)
	}
	if strings.Contains(logs.String(), "private search text") || strings.Contains(logs.String(), "private-session") {
		t.Fatalf("request log contains private input: %s", logs.String())
	}
}

// streamingAgent reports its ranking, streams two deltas, then fails if err is set.
type streamingAgent struct{ err error }

func (a streamingAgent) HandleMessage(_ context.Context, _, _ string, _ eligibility.Qualification, events buyer.Events) (*buyer.TurnResponse, error) {
	ranked := []buyer.Requirement{{Type: "neighborhood", Value: "palermo"}}
	events.Results(buyer.TurnResponse{Requirements: ranked})
	events.Reply("Hola ")
	events.Reply("mundo")
	if a.err != nil {
		return nil, a.err
	}
	return &buyer.TurnResponse{Reply: "Hola mundo", Requirements: ranked}, nil
}

func postStreaming(handler http.Handler) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(`{"session_id":"s","message":"Palermo"}`))
	request.Header.Set("Accept", "application/x-ndjson")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func streamEvents(t *testing.T, body string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("not one JSON event per line: %q", line)
		}
		out = append(out, event)
	}
	return out
}

func TestHandler_StreamsTheTurnAsNDJSONWhenAsked(t *testing.T) {
	handler := httpapi.NewHandler(streamingAgent{}, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), nil)
	response := postStreaming(handler)
	if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/x-ndjson") {
		t.Fatalf("got %d %q", response.Code, response.Header().Get("Content-Type"))
	}
	if !response.Flushed {
		t.Fatal("streamed events must still flush through request logging middleware")
	}
	var got []string
	for _, e := range streamEvents(t, response.Body.String()) {
		requirements, _ := e["requirements"].([]any)
		got = append(got, fmt.Sprintf("%v|%v|%v|%d", e["type"], e["delta"], e["reply"], len(requirements)))
	}
	want := []string{"results|<nil>||1", "reply|Hola |<nil>|0", "reply|mundo|<nil>|0", "done|<nil>|Hola mundo|1"}
	if !slices.Equal(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}

	// Once the stream has started, a failure can only be reported in it.
	failed := streamEvents(t, postStreaming(httpapi.NewHandler(streamingAgent{err: errors.New("model down")}, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), nil)).Body.String())
	if last := failed[len(failed)-1]; last["type"] != "error" || last["error"] == "" || strings.Contains(fmt.Sprint(last["error"]), "model down") {
		t.Fatalf("want a sanitized error event last, got %v", last)
	}
}

func TestHandlerLogsLateStreamFailureEvenAfterHTTP200(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	handler := httpapi.NewHandler(streamingAgent{err: errors.New("private provider response")}, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), nil)
	response := postStreaming(handler)
	if response.Code != http.StatusOK {
		t.Fatalf("stream must keep its committed HTTP status, got %d", response.Code)
	}
	var event map[string]any
	if err := json.Unmarshal(logs.Bytes(), &event); err != nil {
		t.Fatalf("decode request log: %v: %s", err, logs.String())
	}
	if event["status"] != float64(200) || event["outcome"] != "error" {
		t.Fatalf("late stream error must be visible in logs: %v", event)
	}
	if strings.Contains(logs.String(), "private provider response") {
		t.Fatalf("provider response leaked into logs: %s", logs.String())
	}
}
