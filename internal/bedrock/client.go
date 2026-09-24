// Package bedrock implements llm.Client on Amazon Bedrock's Converse API.
//
// It is the hosted counterpart of internal/local: same neutral request, same
// plain-text reply, so which one serves the agent is a wiring decision in
// cmd/hausy. Credentials and region come from the AWS config the caller loads.
package bedrock

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

// Client sends chat requests to one Bedrock model or inference profile.
type Client struct {
	api   *bedrockruntime.Client
	model string
}

// New returns a Client that uses model unless a request names another.
func New(api *bedrockruntime.Client, model string) *Client {
	return &Client{api: api, model: model}
}

// Chat runs one completion. Only plain chat is supported: no caller uses
// tools, so a request with tool definitions, tool calls or tool results is
// refused before it reaches Bedrock rather than silently stripped.
func (c *Client) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if len(req.Tools) > 0 {
		return nil, fmt.Errorf("bedrock: tool use is not supported")
	}
	model := req.Model
	if model == "" {
		model = c.model
	}
	var system []types.SystemContentBlock
	var messages []types.Message
	for _, m := range req.Messages {
		if len(m.ToolCalls) > 0 {
			return nil, fmt.Errorf("bedrock: tool use is not supported")
		}
		text := &types.ContentBlockMemberText{Value: m.Content}
		switch m.Role {
		case "system":
			system = append(system, &types.SystemContentBlockMemberText{Value: m.Content})
		case "user":
			messages = append(messages, types.Message{Role: types.ConversationRoleUser, Content: []types.ContentBlock{text}})
		case "assistant":
			messages = append(messages, types.Message{Role: types.ConversationRoleAssistant, Content: []types.ContentBlock{text}})
		default:
			return nil, fmt.Errorf("bedrock: unsupported message role %q", m.Role)
		}
	}
	config := &types.InferenceConfiguration{Temperature: aws.Float32(float32(req.Temperature))}
	if req.MaxTokens > 0 {
		config.MaxTokens = aws.Int32(int32(req.MaxTokens))
	}

	if req.Stream != nil {
		return c.stream(ctx, &bedrockruntime.ConverseStreamInput{
			ModelId:         aws.String(model),
			System:          system,
			Messages:        messages,
			InferenceConfig: config,
		}, req.Stream)
	}
	out, err := c.api.Converse(ctx, &bedrockruntime.ConverseInput{
		ModelId:         aws.String(model),
		System:          system,
		Messages:        messages,
		InferenceConfig: config,
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock: converse: %w", err)
	}
	var content strings.Builder
	if message, ok := out.Output.(*types.ConverseOutputMemberMessage); ok {
		for _, block := range message.Value.Content {
			if text, ok := block.(*types.ContentBlockMemberText); ok {
				content.WriteString(text.Value)
			}
		}
	}
	return reply(content.String(), out.StopReason)
}

// reply turns collected text into a response. No text at all is an error that
// names why the model stopped (a content filter, a guardrail), which an empty
// string would hide from every caller.
func reply(content string, stop types.StopReason) (*llm.ChatResponse, error) {
	if content == "" {
		return nil, fmt.Errorf("bedrock: model returned no text (stop reason %q)", stop)
	}
	return &llm.ChatResponse{Content: content}, nil
}

// stream collects a ConverseStream reply, handing each piece of text to
// onDelta as it arrives. A stream that ends before messageStop is an error:
// the text so far is not the whole completion.
func (c *Client) stream(ctx context.Context, input *bedrockruntime.ConverseStreamInput, onDelta func(string)) (*llm.ChatResponse, error) {
	out, err := c.api.ConverseStream(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock: converse stream: %w", err)
	}
	events := out.GetStream()
	defer events.Close()
	var content strings.Builder
	var stop *types.StopReason
	for event := range events.Events() {
		switch e := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockDelta:
			if text, ok := e.Value.Delta.(*types.ContentBlockDeltaMemberText); ok && text.Value != "" {
				content.WriteString(text.Value)
				onDelta(text.Value)
			}
		case *types.ConverseStreamOutputMemberMessageStop:
			stop = &e.Value.StopReason
		}
	}
	if err := events.Err(); err != nil {
		return nil, fmt.Errorf("bedrock: converse stream: %w", err)
	}
	if stop == nil {
		return nil, fmt.Errorf("bedrock: converse stream ended before messageStop")
	}
	return reply(content.String(), *stop)
}
