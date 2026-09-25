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
	Stream             bool           `json:"stream,omitempty"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

type wireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func toWireMessages(messages []llm.Message) []wireMessage {
	wire := make([]wireMessage, 0, len(messages))
	for _, message := range messages {
		wire = append(wire, wireMessage{Role: message.Role, Content: message.Content})
	}
	return wire
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

	stream := req.Stream != nil
	payload := chatCompletionRequest{
		Model:       model,
		Messages:    toWireMessages(req.Messages),
		Temperature: req.Temperature,
		MaxTokens:   maxTokens,
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

	return &llm.ChatResponse{Content: chatResp.Choices[0].Message.Content}, nil
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
