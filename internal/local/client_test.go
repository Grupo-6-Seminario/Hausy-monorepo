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
