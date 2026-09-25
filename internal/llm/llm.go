// Package llm defines provider-agnostic interfaces and data structures for language models.
//
// Nothing here names a vendor. The types are the neutral middle a local
// OpenAI-compatible endpoint and Bedrock's Converse API both map onto, so
// callers are written once and the provider is a wiring decision.
package llm

import "context"

// Message is one turn in a chat conversation: "system", "user" or "assistant".
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest asks a provider for one completion.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`

	// Stream, when set, receives the completion's text as it
	// is generated. The response still carries all of it.
	Stream func(delta string) `json:"-"`
}

// ChatResponse is one completion.
type ChatResponse struct {
	Content string `json:"content"`
}

// Client is a language-model completion provider.
type Client interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
