// Package llm defines provider-agnostic interfaces and data structures for language models.
package llm

import "context"

// Message represents a single message in a chat conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a request to generate a chat completion.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

// ChatResponse represents the result of a chat completion.
type ChatResponse struct {
	Content string `json:"content"`
}

// Client defines the interface for language model completion providers.
type Client interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
