// Package agentcore invokes the Bedrock AgentCore harness backing the agent prototype.
package agentcore

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcore"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcore/types"
)

const defaultRegion = "us-east-1"

// runtimeSessionID is shared by every run, so concurrent callers land in the same
// harness session. Generate one per invocation once sessions carry real state.
const runtimeSessionID = "550e8400-e29b-41d4-a716-446655440000"

// Invoke sends prompt to the configured harness and returns the streamed reply.
//
// The harness ARN is account-specific and is read from AGENTCORE_HARNESS_ARN so no
// account identifier is committed; AWS_REGION falls back to us-east-1.
func Invoke(ctx context.Context, prompt string) (string, error) {
	harnessARN := os.Getenv("AGENTCORE_HARNESS_ARN")
	if harnessARN == "" {
		return "", fmt.Errorf("AGENTCORE_HARNESS_ARN is not set; copy .env.example to .env and fill it in")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = defaultRegion
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", err
	}

	client := bedrockagentcore.NewFromConfig(cfg)

	params := &bedrockagentcore.InvokeHarnessInput{
		HarnessArn:       aws.String(harnessARN),
		Qualifier:        aws.String("DEFAULT"), // optional; DEFAULT is implied
		RuntimeSessionId: aws.String(runtimeSessionID),
		Messages: []types.HarnessMessage{
			{
				Role: types.HarnessConversationRoleUser,
				Content: []types.HarnessContentBlock{
					&types.HarnessContentBlockMemberText{Value: prompt},
				},
			},
		},
	}

	output, err := client.InvokeHarness(ctx, params)
	if err != nil {
		return "", err
	}
	defer output.GetStream().Close()

	var result strings.Builder
	for event := range output.GetStream().Events() {
		switch e := event.(type) {
		case *types.InvokeHarnessStreamOutputMemberContentBlockDelta:
			if text, ok := e.Value.Delta.(*types.HarnessContentBlockDeltaMemberText); ok {
				result.WriteString(text.Value)
			}
		}
	}

	if err := output.GetStream().Err(); err != nil {
		return "", err
	}

	return result.String(), nil
}
