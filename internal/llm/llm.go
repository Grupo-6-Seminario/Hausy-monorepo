// Package llm defines provider-agnostic interfaces and data structures for language models.
//
// Nothing here names a vendor. The types are the neutral middle a local
// OpenAI-compatible endpoint and Bedrock's Converse API both map onto, so the
// agent and its tools are written once and the provider is a wiring decision.
package llm

import (
	"context"
	"encoding/json"
)

// Message is one turn in a chat conversation.
//
// Four roles carry meaning: "system", "user", "assistant", and "tool". An
// assistant message may carry ToolCalls instead of (or alongside) Content; a
// tool message answers exactly one of them and must set ToolCallID.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`

	// ToolCalls are the invocations an assistant message asked for.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID names the call a "tool" message is answering. Providers pair
	// results to calls by this id, so a result without it is unattributable.
	ToolCallID string `json:"tool_call_id,omitempty"`

	// Name is the tool that produced a "tool" message, carried for readability
	// in traces and logs; providers do not require it.
	Name string `json:"name,omitempty"`
}

// ToolDefinition describes a capability the model may invoke. InputSchema is a
// JSON Schema object; every provider we target accepts one, which is why the
// definition can stay this small.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ToolCall is one invocation the model asked for. Arguments is raw JSON rather
// than a decoded map so the tool that owns the schema does the decoding, and
// malformed JSON surfaces as that tool's error instead of a transport error.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ChatRequest asks a provider for one completion.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`

	// Tools the model may call on this turn. Empty means plain chat.
	Tools []ToolDefinition `json:"tools,omitempty"`
}

// ChatResponse is one completion. Content and ToolCalls are not exclusive: a
// model may narrate what it is about to look up and then look it up.
type ChatResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Client is a language-model completion provider.
type Client interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
