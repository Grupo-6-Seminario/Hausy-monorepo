package bedrock_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/smithy-go/eventstream"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/bedrock"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

const model = "us.anthropic.claude-sonnet-4-6"

// newClient points a real Bedrock runtime client at a test server, so the
// tests cross the SDK's own encoding and decoding rather than a fake of it.
func newClient(t *testing.T, handler http.HandlerFunc) *bedrock.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	api := bedrockruntime.New(bedrockruntime.Options{
		Region:           "us-east-1",
		BaseEndpoint:     aws.String(server.URL),
		Credentials:      credentials.NewStaticCredentialsProvider("AKIDTEST", "secret", ""),
		RetryMaxAttempts: 1,
	})
	return bedrock.New(api, model)
}

func TestChat_SendsSystemPromptApartAndReturnsTheReply(t *testing.T) {
	var path string
	var body map[string]any
	client := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("request body is not JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"output":{"message":{"role":"assistant","content":[{"text":"## Mi lectura\nHay dos opciones."}]}},"stopReason":"end_turn","usage":{"inputTokens":20,"outputTokens":8,"totalTokens":28},"metrics":{"latencyMs":5}}`)
	})

	resp, err := client.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "Sos el agente de Hausy."},
			{Role: "user", Content: `{"intent":"new_search"}`},
		},
		Temperature: 0,
		MaxTokens:   900,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if resp.Content != "## Mi lectura\nHay dos opciones." {
		t.Errorf("content = %q", resp.Content)
	}
	if path != "/model/us.anthropic.claude-sonnet-4-6/converse" {
		t.Errorf("path = %q", path)
	}
	want := map[string]any{
		"system":          []any{map[string]any{"text": "Sos el agente de Hausy."}},
		"messages":        []any{map[string]any{"role": "user", "content": []any{map[string]any{"text": `{"intent":"new_search"}`}}}},
		"inferenceConfig": map[string]any{"maxTokens": float64(900), "temperature": float64(0)},
	}
	for key, value := range want {
		got, _ := json.Marshal(body[key])
		expected, _ := json.Marshal(value)
		if string(got) != string(expected) {
			t.Errorf("%s = %s, want %s", key, got, expected)
		}
	}
}

// streamEvents answers a ConverseStream call with the given events, encoded
// the way Bedrock sends them: one event-stream frame per event.
func streamEvents(t *testing.T, events ...[2]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/model/us.anthropic.claude-sonnet-4-6/converse-stream" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		encoder := eventstream.NewEncoder()
		for _, event := range events {
			var headers eventstream.Headers
			headers.Set(eventstream.MessageTypeHeader, eventstream.StringValue(eventstream.EventMessageType))
			headers.Set(eventstream.EventTypeHeader, eventstream.StringValue(event[0]))
			headers.Set(eventstream.ContentTypeHeader, eventstream.StringValue("application/json"))
			if err := encoder.Encode(w, eventstream.Message{Headers: headers, Payload: []byte(event[1])}); err != nil {
				t.Errorf("encode %s: %v", event[0], err)
			}
		}
	}
}

func TestChat_StreamsTheReplyAsItArrives(t *testing.T) {
	client := newClient(t, streamEvents(t,
		[2]string{"messageStart", `{"role":"assistant"}`},
		[2]string{"contentBlockDelta", `{"contentBlockIndex":0,"delta":{"text":"## Mi "}}`},
		[2]string{"contentBlockDelta", `{"contentBlockIndex":0,"delta":{"text":"lectura"}}`},
		[2]string{"contentBlockStop", `{"contentBlockIndex":0}`},
		[2]string{"messageStop", `{"stopReason":"end_turn"}`},
	))

	var deltas []string
	resp, err := client.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "hola"}},
		Stream:   func(delta string) { deltas = append(deltas, delta) },
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if resp.Content != "## Mi lectura" {
		t.Errorf("content = %q", resp.Content)
	}
	if len(deltas) != 2 || deltas[0] != "## Mi " || deltas[1] != "lectura" {
		t.Errorf("deltas = %q", deltas)
	}
}

func TestChat_AStreamCutBeforeMessageStopIsAnError(t *testing.T) {
	client := newClient(t, streamEvents(t,
		[2]string{"messageStart", `{"role":"assistant"}`},
		[2]string{"contentBlockDelta", `{"contentBlockIndex":0,"delta":{"text":"## Mi "}}`},
	))

	_, err := client.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "hola"}},
		Stream:   func(string) {},
	})
	if err == nil {
		t.Fatal("expected an error for a stream that ended before messageStop")
	}
}

func TestChat_RejectsToolUseWithoutCallingBedrock(t *testing.T) {
	cases := map[string]llm.ChatRequest{
		"tool definitions": {
			Messages: []llm.Message{{Role: "user", Content: "hola"}},
			Tools:    []llm.ToolDefinition{{Name: "search_listings"}},
		},
		"a tool result": {
			Messages: []llm.Message{{Role: "tool", Content: "[]", ToolCallID: "call_1"}},
		},
		"an assistant tool call": {
			Messages: []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call_1", Name: "search_listings"}}}},
		},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			called := false
			client := newClient(t, func(w http.ResponseWriter, r *http.Request) { called = true })

			if _, err := client.Chat(context.Background(), req); err == nil {
				t.Error("expected an error")
			}
			if called {
				t.Error("Bedrock was called")
			}
		})
	}
}

func TestChat_AReplyWithoutTextIsAnErrorNamingTheStopReason(t *testing.T) {
	t.Run("converse", func(t *testing.T) {
		client := newClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"output":{"message":{"role":"assistant","content":[]}},"stopReason":"content_filtered","usage":{"inputTokens":20,"outputTokens":0,"totalTokens":20},"metrics":{"latencyMs":5}}`)
		})

		_, err := client.Chat(context.Background(), llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: "hola"}}})
		if err == nil || !strings.Contains(err.Error(), "content_filtered") {
			t.Fatalf("err = %v, want one naming content_filtered", err)
		}
	})
	t.Run("converse stream", func(t *testing.T) {
		client := newClient(t, streamEvents(t,
			[2]string{"messageStart", `{"role":"assistant"}`},
			[2]string{"messageStop", `{"stopReason":"content_filtered"}`},
		))

		_, err := client.Chat(context.Background(), llm.ChatRequest{
			Messages: []llm.Message{{Role: "user", Content: "hola"}},
			Stream:   func(string) {},
		})
		if err == nil || !strings.Contains(err.Error(), "content_filtered") {
			t.Fatalf("err = %v, want one naming content_filtered", err)
		}
	})
}
