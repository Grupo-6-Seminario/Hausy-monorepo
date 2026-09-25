package buyer_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
)

type failingLLM struct{}

func (failingLLM) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, errors.New("private provider body")
}

func TestAgentWithoutStoreLogsExtractionFailureSafely(t *testing.T) {
	var logs bytes.Buffer
	ctx := logging.WithLogger(context.Background(), slog.New(slog.NewJSONHandler(&logs, nil)).With("request_id", "test-request"))
	_, err := buyer.NewAgent(failingLLM{}).HandleMessage(ctx, "private-session", "private search text", nil, buyer.Events{})
	if err == nil {
		t.Fatal("expected model failure")
	}
	if !strings.Contains(logs.String(), `"msg":"buyer_extraction"`) || !strings.Contains(logs.String(), `"outcome":"error"`) {
		t.Fatalf("missing extraction failure event: %s", logs.String())
	}
	if strings.Contains(logs.String(), "private provider body") || strings.Contains(logs.String(), "private search text") || strings.Contains(logs.String(), "private-session") {
		t.Fatalf("private input leaked: %s", logs.String())
	}
}

type mockLLMClient struct {
	responses []string
	calls     []llm.ChatRequest
}

func (m *mockLLMClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.calls = append(m.calls, req)
	idx := len(m.calls) - 1
	if idx < len(m.responses) {
		return &llm.ChatResponse{Content: m.responses[idx]}, nil
	}
	return &llm.ChatResponse{Content: `{"requirements":[]}`}, nil
}

func TestAgent_HandleMessage_ExtractsRequirements(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"requirements":[{"type":"property_type","value":"departamento"},{"type":"location","value":"Palermo"},{"type":"feature","value":"luminoso"}]}`,
		},
	}

	agent := buyer.NewAgent(mockClient)
	ctx := context.Background()

	resp, err := agent.HandleMessage(ctx, "session-123", "Busco un departamento luminoso en la zona de Palermo", nil, buyer.Events{})
	if err != nil {
		t.Fatalf("HandleMessage returned unexpected error: %v", err)
	}

	expectedReqs := []buyer.Requirement{
		{Type: "property_type", Value: "departamento"},
		{Type: "location", Value: "Palermo"},
		{Type: "feature", Value: "luminoso"},
	}

	if !reflect.DeepEqual(resp.Requirements, expectedReqs) {
		t.Errorf("expected requirements %+v, got %+v", expectedReqs, resp.Requirements)
	}

	if resp.Reply == "" {
		t.Errorf("expected non-empty agent reply message")
	}

	if len(mockClient.calls) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(mockClient.calls))
	}
}

func TestAgent_HandleMessage_AccumulatesRequirementsAcrossTurns(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{
			`{"requirements":[{"type":"property_type","value":"departamento"},{"type":"location","value":"Palermo"}]}`,
			`{"requirements":[{"type":"feature","value":"balcon"}]}`,
		},
	}

	agent := buyer.NewAgent(mockClient)
	ctx := context.Background()
	sessionID := "session-accumulate"

	// Turn 1
	resp1, err := agent.HandleMessage(ctx, sessionID, "Busco un departamento en Palermo", nil, buyer.Events{})
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}
	if len(resp1.Requirements) != 2 {
		t.Fatalf("expected 2 requirements in turn 1, got %d", len(resp1.Requirements))
	}

	// Turn 2
	resp2, err := agent.HandleMessage(ctx, sessionID, "Tambien quiero que tenga balcon", nil, buyer.Events{})
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}

	expectedReqs := []buyer.Requirement{
		{Type: "property_type", Value: "departamento"},
		{Type: "location", Value: "Palermo"},
		{Type: "feature", Value: "balcon"},
	}

	if !reflect.DeepEqual(resp2.Requirements, expectedReqs) {
		t.Errorf("expected accumulated requirements %+v, got %+v", expectedReqs, resp2.Requirements)
	}
}

func TestAgent_HandleMessage_HandlesMarkdownFences(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{
			"```json\n{\"requirements\":[{\"type\":\"property_type\",\"value\":\"ph\"}]}\n```",
		},
	}

	agent := buyer.NewAgent(mockClient)
	resp, err := agent.HandleMessage(context.Background(), "session-fence", "Busco un PH", nil, buyer.Events{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []buyer.Requirement{{Type: "property_type", Value: "ph"}}
	if !reflect.DeepEqual(resp.Requirements, expected) {
		t.Errorf("expected %+v, got %+v", expected, resp.Requirements)
	}
}
