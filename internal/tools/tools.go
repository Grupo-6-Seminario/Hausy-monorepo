// Package tools turns Go functions into capabilities a language model can
// invoke, and dispatches the calls it makes.
//
// The package knows nothing about property listings or about any particular
// provider. A tool is a name, a description, a JSON Schema and a function --
// the intersection of what every provider we target accepts -- so the same
// registry backs the local OpenAI-compatible endpoint today and a Bedrock
// agent later without the tools themselves changing.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

// Tool is one capability the model may invoke.
//
// Invoke returns a value that will be JSON-encoded for the model, so it should
// be a struct or map whose field names read as documentation. An error it
// returns is shown to the model verbatim: write it as an instruction the model
// can act on ("neighborhood 'pallermo' is not in the inventory; known
// neighborhoods are ..."), not as a stack trace.
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Invoke(ctx context.Context, arguments json.RawMessage) (any, error)
}

// Registry holds the tools available for a conversation and dispatches calls
// to them. The zero value is not usable; call NewRegistry.
type Registry struct {
	order  []string
	byName map[string]Tool
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]Tool)}
}

// Register adds a tool. Names must be unique: the model addresses a tool by
// name and nothing else, so a duplicate is an ambiguity no caller can resolve.
func (r *Registry) Register(tool Tool) error {
	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tools: a tool must have a name")
	}
	if _, exists := r.byName[name]; exists {
		return fmt.Errorf("tools: a tool named %q is already registered", name)
	}
	r.byName[name] = tool
	r.order = append(r.order, name)
	return nil
}

// MustRegister adds every tool or panics. It is for wiring at startup, where a
// duplicate name is a programming error and there is no caller to report it to.
func (r *Registry) MustRegister(list ...Tool) {
	for _, tool := range list {
		if err := r.Register(tool); err != nil {
			panic(err)
		}
	}
}

// Definitions describes the registered tools for a provider, in registration
// order. Order is stable so the serialized prompt is stable and cacheable.
func (r *Registry) Definitions() []llm.ToolDefinition {
	definitions := make([]llm.ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		tool := r.byName[name]
		definitions = append(definitions, llm.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return definitions
}

// Invoke runs one call and returns the message to send back.
//
// It never returns an error. Every failure -- an unknown tool, unparseable
// arguments, a tool that refused -- comes back as the tool's result so the
// model reads the reason and corrects itself on the next turn. Aborting the
// conversation instead would hide the one thing that could fix it.
func (r *Registry) Invoke(ctx context.Context, call llm.ToolCall) llm.Message {
	tool, known := r.byName[call.Name]
	if !known {
		return r.errorMessage(call, fmt.Errorf("no tool named %q is available; call one of %v", call.Name, r.order))
	}

	output, err := tool.Invoke(ctx, call.Arguments)
	if err != nil {
		return r.errorMessage(call, err)
	}

	encoded, err := json.Marshal(output)
	if err != nil {
		return r.errorMessage(call, fmt.Errorf("tool %q produced a result that could not be encoded: %w", call.Name, err))
	}

	return llm.Message{
		Role:       "tool",
		Name:       call.Name,
		ToolCallID: call.ID,
		Content:    string(encoded),
	}
}

func (r *Registry) errorMessage(call llm.ToolCall, cause error) llm.Message {
	// Hand-built rather than marshalled: this path is what runs when encoding
	// has already gone wrong, and it must not be able to fail in turn.
	encoded, err := json.Marshal(struct {
		Error string `json:"error"`
	}{Error: cause.Error()})
	if err != nil {
		encoded = []byte(`{"error":"the tool failed and the reason could not be encoded"}`)
	}
	return llm.Message{
		Role:       "tool",
		Name:       call.Name,
		ToolCallID: call.ID,
		Content:    string(encoded),
	}
}
