package tools

import (
	"context"
	"fmt"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

// DefaultMaxSteps bounds how many times a single user turn may go back to the
// model. Four is room to look up the vocabulary, search, widen a search that
// came back empty, and answer -- past that a model is looping rather than
// working.
const DefaultMaxSteps = 4

// Step is one tool call and what it returned. The trace is kept so a caller
// can show the user what was actually looked up, which is the difference
// between an answer and an answer you can check.
type Step struct {
	Call   llm.ToolCall `json:"call"`
	Result string       `json:"result"`
}

// Result is the outcome of one user turn.
type Result struct {
	// Reply is the model's final message to the user.
	Reply string `json:"reply"`
	// Messages is the full conversation including tool traffic, ready to be
	// carried into the next turn.
	Messages []llm.Message `json:"messages"`
	// Steps are the tool calls made, in the order they ran.
	Steps []Step `json:"steps"`
}

// Runner drives the call-tool-then-answer loop against a provider.
//
// It is the only place that knows a turn can take several round trips, which
// keeps that concern out of both the provider clients and the agent.
type Runner struct {
	client   llm.Client
	registry *Registry
	maxSteps int
}

// NewRunner returns a runner. A maxSteps of zero or less means DefaultMaxSteps.
func NewRunner(client llm.Client, registry *Registry, maxSteps int) *Runner {
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	return &Runner{client: client, registry: registry, maxSteps: maxSteps}
}

// Run completes one user turn, executing any tools the model asks for.
//
// The tools offered come from the registry; a Tools field already set on req
// is overwritten, so there is exactly one answer to "what can this agent do".
func (r *Runner) Run(ctx context.Context, req llm.ChatRequest) (*Result, error) {
	req.Tools = r.registry.Definitions()
	messages := append([]llm.Message(nil), req.Messages...)
	var steps []Step

	for step := 0; step < r.maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req.Messages = messages
		response, err := r.client.Chat(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("tools: model call failed: %w", err)
		}

		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})

		if len(response.ToolCalls) == 0 {
			return &Result{Reply: response.Content, Messages: messages, Steps: steps}, nil
		}

		// Every call in the turn is answered before going back to the model:
		// a provider that asked for two lookups rejects a reply that resolves
		// only one of them.
		for _, call := range response.ToolCalls {
			result := r.registry.Invoke(ctx, call)
			messages = append(messages, result)
			steps = append(steps, Step{Call: call, Result: result.Content})
		}
	}

	return nil, fmt.Errorf("tools: the model was still calling tools after %d steps", r.maxSteps)
}
