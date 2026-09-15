package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
)

type recordingAgent struct {
	sessionID string
	message   string
}

func (a *recordingAgent) HandleMessage(_ context.Context, sessionID, message string) (*buyer.TurnResponse, error) {
	a.sessionID = sessionID
	a.message = message
	return &buyer.TurnResponse{Reply: "Respuesta real del agente."}, nil
}

func TestHandler_PostMessageReturnsAgentReply(t *testing.T) {
	agent := &recordingAgent{}
	handler := httpapi.NewHandler(agent, auth.NewLocal(auth.NewMemoryStore()))
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
