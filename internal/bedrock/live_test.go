package bedrock_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/bedrock"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

// TestChat_LiveBedrock streams one short reply from the real model. It costs
// a fraction of a cent, so it only runs when asked:
//
//	HAUSY_BEDROCK_LIVE=1 AWS_PROFILE=<your aws login profile> go test ./internal/bedrock -run Live
func TestChat_LiveBedrock(t *testing.T) {
	if os.Getenv("HAUSY_BEDROCK_LIVE") != "1" {
		t.Skip("set HAUSY_BEDROCK_LIVE=1 to call Bedrock")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		t.Fatalf("load AWS config: %v", err)
	}
	model := os.Getenv("BEDROCK_MODEL_ID")
	if model == "" {
		model = "us.anthropic.claude-sonnet-4-6"
	}
	client := bedrock.New(bedrockruntime.NewFromConfig(cfg), model)

	var streamed strings.Builder
	resp, err := client.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "Respondé en una sola palabra."},
			{Role: "user", Content: "¿En qué ciudad está el barrio de Palermo que busca Hausy?"},
		},
		Temperature: 0,
		MaxTokens:   20,
		Stream:      func(delta string) { streamed.WriteString(delta) },
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if strings.TrimSpace(resp.Content) == "" || streamed.String() != resp.Content {
		t.Fatalf("reply %q, streamed %q", resp.Content, streamed.String())
	}
	t.Logf("reply: %q", resp.Content)
}
