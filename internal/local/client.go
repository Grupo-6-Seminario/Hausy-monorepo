// Package local implements an LLM client for locally hosted OpenAI-compatible models.
package local

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

const (
	defaultBaseURL = "http://127.0.0.1:8000"
	defaultModel   = "Qwen3.5-9B-4bit"
)

// Client invokes an OpenAI-compatible API running locally (e.g. omlx, vLLM, llama.cpp).
type Client struct {
	baseURL      string
	token        string
	defaultModel string
	httpClient   *http.Client
}

// NewClient returns a new local LLM client.
func NewClient(baseURL, token, defaultModelName string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	if defaultModelName == "" {
		defaultModelName = defaultModel
	}

	return &Client{
		baseURL:      baseURL,
		token:        token,
		defaultModel: defaultModelName,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// The OpenAI wire shape is deliberately separate from llm.Message. The two
// diverge once tools enter: on the wire a call's arguments are a JSON *string*
// nested under "function", while the neutral type carries raw JSON. Mapping in
// one place keeps that quirk from leaking into the agent.
type chatCompletionRequest struct {
	Model              string         `json:"model"`
	Messages           []wireMessage  `json:"messages"`
	Temperature        float64        `json:"temperature"`
	MaxTokens          int            `json:"max_tokens,omitempty"`
	Tools              []wireTool     `json:"tools,omitempty"`
	Stream             bool           `json:"stream,omitempty"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

type wireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type wireTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Role      string         `json:"role"`
			Content   string         `json:"content"`
			ToolCalls []wireToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

func toWireMessages(messages []llm.Message) []wireMessage {
	wire := make([]wireMessage, 0, len(messages))
	for _, message := range messages {
		out := wireMessage{
			Role:       message.Role,
			Content:    message.Content,
			ToolCallID: message.ToolCallID,
			Name:       message.Name,
		}
		for _, call := range message.ToolCalls {
			var wireCall wireToolCall
			wireCall.ID = call.ID
			wireCall.Type = "function"
			wireCall.Function.Name = call.Name
			// An absent argument object is "{}", not "": a bare empty string
			// is not JSON and some servers reject the whole request over it.
			wireCall.Function.Arguments = "{}"
			if len(call.Arguments) > 0 {
				wireCall.Function.Arguments = string(call.Arguments)
			}
			out.ToolCalls = append(out.ToolCalls, wireCall)
		}
		wire = append(wire, out)
	}
	return wire
}

func toWireTools(definitions []llm.ToolDefinition) []wireTool {
	if len(definitions) == 0 {
		return nil
	}
	wire := make([]wireTool, 0, len(definitions))
	for _, definition := range definitions {
		var tool wireTool
		tool.Type = "function"
		tool.Function.Name = definition.Name
		tool.Function.Description = definition.Description
		tool.Function.Parameters = definition.InputSchema
		wire = append(wire, tool)
	}
	return wire
}

func fromWireToolCalls(calls []wireToolCall) []llm.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]llm.ToolCall, 0, len(calls))
	for _, call := range calls {
		arguments := strings.TrimSpace(call.Function.Arguments)
		if arguments == "" {
			arguments = "{}"
		}
		out = append(out, llm.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: json.RawMessage(arguments),
		})
	}
	return out
}

// Chat sends a completion request to the local OpenAI-compatible endpoint.
func (c *Client) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Tool calls arrive whole; only plain text is streamed.
	stream := req.Stream != nil && len(req.Tools) == 0
	payload := chatCompletionRequest{
		Model:       model,
		Messages:    toWireMessages(req.Messages),
		Temperature: req.Temperature,
		MaxTokens:   maxTokens,
		Tools:       toWireTools(req.Tools),
		Stream:      stream,
		ChatTemplateKwargs: map[string]any{
			"enable_thinking": false,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if stream && httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {
		return readStream(httpResp.Body, req.Stream)
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("api error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	var chatResp chatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("api returned 0 choices")
	}

	return &llm.ChatResponse{
		Content:   chatResp.Choices[0].Message.Content,
		ToolCalls: fromWireToolCalls(chatResp.Choices[0].Message.ToolCalls),
	}, nil
}

// readStream collects an OpenAI-style server-sent event stream, handing each
// piece of text to onDelta as it arrives. A stream cut before [DONE] is an
// error: the text so far is not the whole completion.
func readStream(body io.Reader, onDelta func(string)) (*llm.ChatResponse, error) {
	var content strings.Builder
	lines := bufio.NewScanner(body)
	lines.Buffer(make([]byte, 64<<10), 1<<20)
	for lines.Scan() {
		data, ok := strings.CutPrefix(lines.Text(), "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			return &llm.ChatResponse{Content: content.String()}, nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stream chunk: %w", err)
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				onDelta(choice.Delta.Content)
			}
		}
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response stream: %w", err)
	}
	return nil, fmt.Errorf("response stream ended before [DONE]")
}
