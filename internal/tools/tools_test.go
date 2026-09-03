package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/tools"
)

type echoInput struct {
	Text  string `json:"text"`
	Times int    `json:"times"`
}

func echoTool(fn func(context.Context, echoInput) (any, error)) tools.Tool {
	return tools.New(tools.Spec{
		Name:        "echo",
		Description: "Repeat a phrase.",
		InputSchema: map[string]any{"type": "object"},
	}, fn)
}

func TestRegistry_DefinitionsAreExposedInRegistrationOrder(t *testing.T) {
	registry := tools.NewRegistry()
	mustRegister(t, registry, echoTool(nil))
	mustRegister(t, registry, tools.New(tools.Spec{
		Name: "reverse", Description: "Reverse a phrase.", InputSchema: map[string]any{"type": "object"},
	}, func(context.Context, echoInput) (any, error) { return nil, nil }))

	definitions := registry.Definitions()
	if len(definitions) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(definitions))
	}
	if definitions[0].Name != "echo" || definitions[1].Name != "reverse" {
		t.Errorf("expected registration order [echo reverse], got [%s %s]", definitions[0].Name, definitions[1].Name)
	}
	if definitions[0].Description != "Repeat a phrase." {
		t.Errorf("unexpected description %q", definitions[0].Description)
	}
}

func TestRegistry_RejectsDuplicateNames(t *testing.T) {
	registry := tools.NewRegistry()
	mustRegister(t, registry, echoTool(nil))

	if err := registry.Register(echoTool(nil)); err == nil {
		t.Fatal("expected registering a second tool named echo to fail")
	}
}

func TestRegistry_InvokeReturnsToolMessagePairedToTheCall(t *testing.T) {
	registry := tools.NewRegistry()
	mustRegister(t, registry, echoTool(func(_ context.Context, in echoInput) (any, error) {
		return map[string]any{"said": strings.Repeat(in.Text, in.Times)}, nil
	}))

	message := registry.Invoke(context.Background(), llm.ToolCall{
		ID: "call_7", Name: "echo", Arguments: json.RawMessage(`{"text":"ho","times":3}`),
	})

	if message.Role != "tool" {
		t.Errorf("expected role %q, got %q", "tool", message.Role)
	}
	if message.ToolCallID != "call_7" {
		t.Errorf("expected the result to be paired to call_7, got %q", message.ToolCallID)
	}
	if message.Name != "echo" {
		t.Errorf("expected the result to name the tool, got %q", message.Name)
	}
	if message.Content != `{"said":"hohoho"}` {
		t.Errorf("expected the tool output as JSON, got %s", message.Content)
	}
}

// A tool that fails is not a failure of the loop: the model has to read the
// reason to correct its next call, so every failure comes back as content.
func TestRegistry_InvokeReportsFailuresAsContentTheModelCanRead(t *testing.T) {
	registry := tools.NewRegistry()
	mustRegister(t, registry, echoTool(func(context.Context, echoInput) (any, error) {
		return nil, errors.New("neighborhood 'pallermo' is not in the inventory")
	}))

	cases := map[string]struct {
		call        llm.ToolCall
		wantFagment string
	}{
		"unknown tool": {
			call:        llm.ToolCall{ID: "c1", Name: "teleport", Arguments: json.RawMessage(`{}`)},
			wantFagment: "teleport",
		},
		"malformed arguments": {
			call:        llm.ToolCall{ID: "c2", Name: "echo", Arguments: json.RawMessage(`{"text":`)},
			wantFagment: "arguments",
		},
		"tool returned an error": {
			call:        llm.ToolCall{ID: "c3", Name: "echo", Arguments: json.RawMessage(`{"text":"hi"}`)},
			wantFagment: "pallermo",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			message := registry.Invoke(context.Background(), testCase.call)
			if message.Role != "tool" || message.ToolCallID != testCase.call.ID {
				t.Fatalf("expected a tool message paired to %s, got %+v", testCase.call.ID, message)
			}
			var payload struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal([]byte(message.Content), &payload); err != nil {
				t.Fatalf("expected JSON content, got %s", message.Content)
			}
			if payload.Error == "" {
				t.Fatalf("expected an error field, got %s", message.Content)
			}
			if !strings.Contains(payload.Error, testCase.wantFagment) {
				t.Errorf("expected the error to mention %q, got %q", testCase.wantFagment, payload.Error)
			}
		})
	}
}

// Small models emit stray fields. Ignoring them keeps an otherwise valid call
// from bouncing, while a wrongly *typed* field still has to fail loudly.
func TestRegistry_InvokeIgnoresUnknownArgumentFields(t *testing.T) {
	registry := tools.NewRegistry()
	mustRegister(t, registry, echoTool(func(_ context.Context, in echoInput) (any, error) {
		return map[string]any{"said": in.Text}, nil
	}))

	message := registry.Invoke(context.Background(), llm.ToolCall{
		ID: "c1", Name: "echo", Arguments: json.RawMessage(`{"text":"hi","vibe":"warm"}`),
	})
	if message.Content != `{"said":"hi"}` {
		t.Errorf("expected the unknown field to be ignored, got %s", message.Content)
	}
}

func mustRegister(t *testing.T, registry *tools.Registry, tool tools.Tool) {
	t.Helper()
	if err := registry.Register(tool); err != nil {
		t.Fatalf("Register(%s) failed: %v", tool.Name(), err)
	}
}
