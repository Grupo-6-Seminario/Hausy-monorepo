package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Spec is the model-facing half of a tool: what it is called, what it is for,
// and the shape of its arguments.
//
// Description is prompt text, not documentation for us. It is the only thing
// telling the model when to reach for this tool rather than another, so say
// what question the tool answers and when it is the wrong one.
type Spec struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// New adapts a typed Go function into a Tool, decoding the model's arguments
// into In. It exists so a tool implementation is a function over a struct --
// the JSON boundary is handled once, here, rather than in every tool.
func New[In any](spec Spec, fn func(ctx context.Context, input In) (any, error)) Tool {
	return &typedTool{
		spec: spec,
		invoke: func(ctx context.Context, arguments json.RawMessage) (any, error) {
			var input In
			if len(arguments) > 0 {
				// Unknown fields are ignored on purpose. Small models garnish
				// calls with fields that are not in the schema; bouncing an
				// otherwise valid call over one costs a round trip and often
				// produces the same garnish again. A wrongly *typed* field
				// still fails here, which is the error worth reporting.
				if err := json.Unmarshal(arguments, &input); err != nil {
					return nil, fmt.Errorf("the arguments for %q are not valid for its schema: %w", spec.Name, err)
				}
			}
			if fn == nil {
				return nil, fmt.Errorf("tool %q has no implementation", spec.Name)
			}
			return fn(ctx, input)
		},
	}
}

type typedTool struct {
	spec   Spec
	invoke func(ctx context.Context, arguments json.RawMessage) (any, error)
}

func (t *typedTool) Name() string                { return t.spec.Name }
func (t *typedTool) Description() string         { return t.spec.Description }
func (t *typedTool) InputSchema() map[string]any { return t.spec.InputSchema }

func (t *typedTool) Invoke(ctx context.Context, arguments json.RawMessage) (any, error) {
	return t.invoke(ctx, arguments)
}
