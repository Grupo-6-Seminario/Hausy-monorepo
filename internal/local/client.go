// Package local implements an LLM client for locally hosted OpenAI-compatible models.
package local

import (
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

type chatCompletionRequest struct {
	Model              string         `json:"model"`
	Messages           []llm.Message  `json:"messages"`
	Temperature        float64        `json:"temperature"`
	MaxTokens          int            `json:"max_tokens,omitempty"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Chat sends a completion request to the local OpenAI-compatible endpoint.
func (c *Client) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 512
	}

	payload := chatCompletionRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   maxTokens,
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
		Content: chatResp.Choices[0].Message.Content,
	}, nil
}
