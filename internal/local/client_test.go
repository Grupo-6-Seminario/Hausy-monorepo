package local_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
)

func TestClient_Chat_Success(t *testing.T) {
	expectedToken := "test-bearer-token"
	expectedPrompt := "Busco un departamento luminoso en Palermo"
	expectedReply := `{"requirements":[{"type":"property_type","value":"departamento"}]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer "+expectedToken {
			t.Errorf("expected Authorization Bearer %s, got %s", expectedToken, auth)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}

		var reqPayload struct {
			Model       string        `json:"model"`
			Messages    []llm.Message `json:"messages"`
			Temperature float64       `json:"temperature"`
			MaxTokens   int           `json:"max_tokens"`
		}
		if err := json.Unmarshal(body, &reqPayload); err != nil {
			t.Fatalf("failed unmarshaling request body: %v", err)
		}

		if len(reqPayload.Messages) == 0 || reqPayload.Messages[len(reqPayload.Messages)-1].Content != expectedPrompt {
			t.Errorf("unexpected messages: %+v", reqPayload.Messages)
		}

		w.Header().Set("Content-Type", "application/json")
		respPayload := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": expectedReply,
					},
					"finish_reason": "stop",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(respPayload)
	}))
	defer server.Close()

	client := local.NewClient(server.URL, expectedToken, "Qwen3.5-9B-4bit")
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: expectedPrompt},
		},
	}

	resp, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("client.Chat returned unexpected error: %v", err)
	}

	if resp.Content != expectedReply {
		t.Errorf("expected content %q, got %q", expectedReply, resp.Content)
	}
}

func TestClient_Chat_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal model engine error"))
	}))
	defer server.Close()

	client := local.NewClient(server.URL, "token", "")
	_, err := client.Chat(context.Background(), llm.ChatRequest{})
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected error to mention status 500, got: %v", err)
	}
}

func TestClient_Chat_SendsToolDefinitionsAndReadsToolCalls(t *testing.T) {
	var captured struct {
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("failed unmarshaling request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[
			{"id":"call_1","type":"function","function":{"name":"search_listings","arguments":"{\"neighborhoods\":[\"palermo\"]}"}}
		]},"finish_reason":"tool_calls"}]}`))
	}))
	defer server.Close()

	client := local.NewClient(server.URL, "", "")
	resp, err := client.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "Depto en Palermo"}},
		Tools: []llm.ToolDefinition{{
			Name:        "search_listings",
			Description: "Search the listing inventory.",
			InputSchema: map[string]any{"type": "object"},
		}},
	})
	if err != nil {
		t.Fatalf("Chat returned unexpected error: %v", err)
	}

	if len(captured.Tools) != 1 {
		t.Fatalf("expected 1 tool in the request payload, got %d", len(captured.Tools))
	}
	if captured.Tools[0].Type != "function" {
		t.Errorf("expected tool type %q, got %q", "function", captured.Tools[0].Type)
	}
	if captured.Tools[0].Function.Name != "search_listings" {
		t.Errorf("expected tool name %q, got %q", "search_listings", captured.Tools[0].Function.Name)
	}
	if captured.Tools[0].Function.Parameters["type"] != "object" {
		t.Errorf("expected the input schema to be sent as parameters, got %+v", captured.Tools[0].Function.Parameters)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	call := resp.ToolCalls[0]
	if call.ID != "call_1" || call.Name != "search_listings" {
		t.Errorf("unexpected tool call identity: %+v", call)
	}
	if string(call.Arguments) != `{"neighborhoods":["palermo"]}` {
		t.Errorf("expected arguments to be decoded from the JSON string, got %s", call.Arguments)
	}
}

func TestClient_Chat_SendsToolResultsInWireShape(t *testing.T) {
	var captured struct {
		Messages []struct {
			Role       string `json:"role"`
			Content    string `json:"content"`
			ToolCallID string `json:"tool_call_id"`
			ToolCalls  []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("failed unmarshaling request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Listo"}}]}`))
	}))
	defer server.Close()

	client := local.NewClient(server.URL, "", "")
	_, err := client.Chat(context.Background(), llm.ChatRequest{Messages: []llm.Message{
		{Role: "user", Content: "Depto en Palermo"},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{
			ID: "call_1", Name: "search_listings", Arguments: json.RawMessage(`{"neighborhoods":["palermo"]}`),
		}}},
		{Role: "tool", ToolCallID: "call_1", Content: `{"total_matches":3}`},
	}})
	if err != nil {
		t.Fatalf("Chat returned unexpected error: %v", err)
	}

	if len(captured.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(captured.Messages))
	}
	assistant := captured.Messages[1]
	if len(assistant.ToolCalls) != 1 {
		t.Fatalf("expected the assistant message to carry 1 tool call, got %d", len(assistant.ToolCalls))
	}
	if assistant.ToolCalls[0].Function.Arguments != `{"neighborhoods":["palermo"]}` {
		t.Errorf("expected arguments re-encoded as a JSON string, got %q", assistant.ToolCalls[0].Function.Arguments)
	}
	result := captured.Messages[2]
	if result.Role != "tool" || result.ToolCallID != "call_1" || result.Content != `{"total_matches":3}` {
		t.Errorf("unexpected tool result message: %+v", result)
	}
}

// The writer streams its reply so the searcher reads it as it is written. The
// chunks below are omlx's, keepalive and empty deltas included.
func TestClient_Chat_StreamsTextWhenAsked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Stream bool `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error(`want "stream": true`)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range []string{
			`{"model":"keepalive","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}`,
			`{"choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
			`{"choices":[{"index":0,"delta":{"content":"¡Hola! ¿En qué puedo"}}]}`,
			`{"choices":[{"index":0,"delta":{"content":" ayudarte"}}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		} {
			io.WriteString(w, "data: "+chunk+"\n\n")
		}
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	var deltas []string
	resp, err := local.NewClient(server.URL, "", "m").Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "Decí hola"}},
		Stream:   func(delta string) { deltas = append(deltas, delta) },
	})
	if err != nil || resp.Content != "¡Hola! ¿En qué puedo ayudarte" || strings.Join(deltas, "|") != "¡Hola! ¿En qué puedo| ayudarte" {
		t.Fatalf("got %q (deltas %q), %v", resp, deltas, err)
	}
}
