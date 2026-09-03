package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/tools"
)

type scriptedClient struct {
	replies  []llm.ChatResponse
	requests []llm.ChatRequest
}

func (c *scriptedClient) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.requests = append(c.requests, req)
	index := len(c.requests) - 1
	if index >= len(c.replies) {
		return &llm.ChatResponse{Content: "sin más que agregar"}, nil
	}
	reply := c.replies[index]
	return &reply, nil
}

func searchRegistry(t *testing.T, result string) *tools.Registry {
	t.Helper()
	registry := tools.NewRegistry()
	mustRegister(t, registry, tools.New(tools.Spec{
		Name: "search_listings", Description: "Search.", InputSchema: map[string]any{"type": "object"},
	}, func(context.Context, echoInput) (any, error) {
		return json.RawMessage(result), nil
	}))
	return registry
}

func TestRunner_ReturnsThePlainReplyWhenNoToolIsCalled(t *testing.T) {
	client := &scriptedClient{replies: []llm.ChatResponse{{Content: "Contame tu presupuesto."}}}
	runner := tools.NewRunner(client, searchRegistry(t, `{"total_matches":0}`), 5)

	result, err := runner.Run(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "Hola"}},
	})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if result.Reply != "Contame tu presupuesto." {
		t.Errorf("expected the model's reply, got %q", result.Reply)
	}
	if len(result.Steps) != 0 {
		t.Errorf("expected no tool steps, got %d", len(result.Steps))
	}
	if len(client.requests) != 1 {
		t.Fatalf("expected 1 model call, got %d", len(client.requests))
	}
	if len(client.requests[0].Tools) != 1 || client.requests[0].Tools[0].Name != "search_listings" {
		t.Errorf("expected the registry's tools to be offered, got %+v", client.requests[0].Tools)
	}
}

func TestRunner_FeedsToolResultsBackAndReturnsTheFollowUpReply(t *testing.T) {
	client := &scriptedClient{replies: []llm.ChatResponse{
		{ToolCalls: []llm.ToolCall{{ID: "call_1", Name: "search_listings", Arguments: json.RawMessage(`{"text":"palermo"}`)}}},
		{Content: "Encontré 2 opciones."},
	}}
	runner := tools.NewRunner(client, searchRegistry(t, `{"total_matches":2}`), 5)

	result, err := runner.Run(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "Depto en Palermo"}},
	})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if result.Reply != "Encontré 2 opciones." {
		t.Errorf("expected the reply after the tool ran, got %q", result.Reply)
	}

	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 tool step in the trace, got %d", len(result.Steps))
	}
	if result.Steps[0].Call.Name != "search_listings" || result.Steps[0].Result != `{"total_matches":2}` {
		t.Errorf("unexpected step: %+v", result.Steps[0])
	}

	if len(client.requests) != 2 {
		t.Fatalf("expected 2 model calls, got %d", len(client.requests))
	}
	second := client.requests[1].Messages
	if len(second) != 3 {
		t.Fatalf("expected user + assistant + tool messages on the second call, got %d", len(second))
	}
	if len(second[1].ToolCalls) != 1 {
		t.Errorf("expected the assistant's tool call to be replayed, got %+v", second[1])
	}
	if second[2].Role != "tool" || second[2].ToolCallID != "call_1" {
		t.Errorf("expected the tool result paired to call_1, got %+v", second[2])
	}
}

func TestRunner_StopsAModelThatKeepsCallingToolsForever(t *testing.T) {
	looping := llm.ChatResponse{ToolCalls: []llm.ToolCall{
		{ID: "call_n", Name: "search_listings", Arguments: json.RawMessage(`{}`)},
	}}
	client := &scriptedClient{replies: []llm.ChatResponse{looping, looping, looping, looping}}
	runner := tools.NewRunner(client, searchRegistry(t, `{"total_matches":0}`), 2)

	_, err := runner.Run(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "Depto"}},
	})
	if err == nil {
		t.Fatal("expected an error once the step budget is spent")
	}
	if len(client.requests) != 2 {
		t.Errorf("expected the runner to stop after 2 model calls, got %d", len(client.requests))
	}
}

func TestRunner_RunsEveryToolCallInOneAssistantTurn(t *testing.T) {
	client := &scriptedClient{replies: []llm.ChatResponse{
		{ToolCalls: []llm.ToolCall{
			{ID: "call_1", Name: "search_listings", Arguments: json.RawMessage(`{}`)},
			{ID: "call_2", Name: "search_listings", Arguments: json.RawMessage(`{}`)},
		}},
		{Content: "Comparé las dos zonas."},
	}}
	runner := tools.NewRunner(client, searchRegistry(t, `{"total_matches":1}`), 5)

	result, err := runner.Run(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "Palermo o Belgrano"}},
	})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected both calls to run, got %d steps", len(result.Steps))
	}
	if got := len(client.requests[1].Messages); got != 4 {
		t.Errorf("expected user + assistant + 2 tool messages, got %d", got)
	}
}
