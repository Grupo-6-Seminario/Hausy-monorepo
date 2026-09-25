package main

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/bedrock"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
)

func TestLogLevelFromEnv(t *testing.T) {
	t.Setenv("HAUSY_LOG_LEVEL", "debug")
	if level, err := logLevelFromEnv(); err != nil || level != slog.LevelDebug {
		t.Fatalf("debug level: got %v, %v", level, err)
	}
	t.Setenv("HAUSY_LOG_LEVEL", "unknown")
	if _, err := logLevelFromEnv(); err == nil {
		t.Fatal("unknown log level must fail configuration")
	}
}

func TestServerConfigFromEnvUsesLocalDefaults(t *testing.T) {
	t.Setenv("HAUSY_API_ADDR", "")
	t.Setenv("LOCAL_LLM_URL", "")
	t.Setenv("LOCAL_LLM_MODEL", "")

	config := serverConfigFromEnv()

	if config.address != "127.0.0.1:8080" {
		t.Fatalf("expected local API address, got %q", config.address)
	}
	if config.llmURL != "http://127.0.0.1:8000" {
		t.Fatalf("expected local LLM URL, got %q", config.llmURL)
	}
	if config.llmModel != "Qwen3.5-9B-4bit" {
		t.Fatalf("expected local model, got %q", config.llmModel)
	}
}

func TestServerConfigFromEnvDefaultsToTheLocalDatabase(t *testing.T) {
	t.Setenv("DATABASE_URI", "")

	if got := serverConfigFromEnv().databaseURI; got != "postgresql://hausy:hausy@localhost:5432/hausy" {
		t.Fatalf("expected the local development database, got %q", got)
	}
}

func TestServerConfigFromEnvDefaultsToTheLocalModel(t *testing.T) {
	t.Setenv("HAUSY_LLM", "")
	t.Setenv("BEDROCK_MODEL_ID", "")

	config := serverConfigFromEnv()

	if config.llmProvider != "local" {
		t.Fatalf("expected the local model provider, got %q", config.llmProvider)
	}
	if config.bedrockModel != "us.anthropic.claude-sonnet-4-6" {
		t.Fatalf("expected the approved Sonnet 4.6 inference profile, got %q", config.bedrockModel)
	}
}

func TestNewLLMClientPicksTheConfiguredProvider(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-1")
	config := serverConfigFromEnv()

	config.llmProvider = "local"
	if client, err := newLLMClient(context.Background(), config); err != nil {
		t.Fatalf("local: %v", err)
	} else if _, ok := client.(*local.Client); !ok {
		t.Fatalf("local: got %T", client)
	}

	config.llmProvider = "bedrock"
	if client, err := newLLMClient(context.Background(), config); err != nil {
		t.Fatalf("bedrock: %v", err)
	} else if _, ok := client.(*bedrock.Client); !ok {
		t.Fatalf("bedrock: got %T", client)
	}
}

// A typo must not quietly fall back to the local model: a benchmark or a demo
// would then measure the wrong provider.
func TestNewLLMClientRejectsAnUnknownProvider(t *testing.T) {
	config := serverConfigFromEnv()
	config.llmProvider = "bedrok"

	if _, err := newLLMClient(context.Background(), config); err == nil {
		t.Fatal("expected an error for an unknown provider")
	}
}
