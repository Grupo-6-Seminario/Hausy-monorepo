package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

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

// The reply writer can run on another provider than the planner (Bedrock for
// grounded replies) while local development keeps the local model for all.
func TestWriterClientFollowsTheModelUnlessSeparated(t *testing.T) {
	ctx := context.Background()
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("HAUSY_LLM", "local")
	t.Setenv("HAUSY_WRITER_LLM", "")
	config := serverConfigFromEnv()
	shared, err := newLLMClient(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if writer, err := writerClient(ctx, config, shared); err != nil || writer != shared {
		t.Fatalf("unset: the writer must share the model client, got %T %v", writer, err)
	}

	t.Setenv("HAUSY_WRITER_LLM", "bedrock")
	if writer, err := writerClient(ctx, serverConfigFromEnv(), shared); err != nil {
		t.Fatalf("bedrock writer: %v", err)
	} else if _, ok := writer.(*bedrock.Client); !ok {
		t.Fatalf("bedrock writer: got %T", writer)
	}

	t.Setenv("HAUSY_WRITER_LLM", "bedrok")
	if _, err := writerClient(ctx, serverConfigFromEnv(), shared); err == nil {
		t.Fatal("a typo must stop startup, not fall back to the local model")
	}
}

// Seen live 2026-09-25: with Postgres stopped the API kept serving a
// requirements-only chat whose replies looked real, so the clarification
// feature seemed broken while no turn logged an error.
func TestServerRefusesToStartWithoutItsDatabase(t *testing.T) {
	if os.Getenv("HAUSY_TEST_RUN_MAIN") == "1" {
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestServerRefusesToStartWithoutItsDatabase$")
	cmd.Env = append(os.Environ(), "HAUSY_TEST_RUN_MAIN=1", "HAUSY_LLM=local", "HAUSY_WRITER_LLM=local", "HAUSY_LOG_LEVEL=info",
		"HAUSY_API_ADDR=127.0.0.1:0", "DATABASE_URI=postgresql://hausy:hausy@127.0.0.1:1/hausy")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(out.String(), `"msg":"startup_failed"`) || !strings.Contains(out.String(), `"stage":"database_open"`) {
			t.Fatalf("want exit status 1 naming the database, got %v:\n%s", err, out.String())
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatalf("the server kept running without its database:\n%s", out.String())
	}
}

// Seen live 2026-09-26: the `aws login` session had expired hours before the
// API started, yet it served every search with the template reply because
// Bedrock credentials resolve on the first call, not at startup.
func TestServerRefusesToStartWithoutBedrockCredentials(t *testing.T) {
	if os.Getenv("HAUSY_TEST_RUN_MAIN") == "1" {
		main()
		return
	}
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "AWS_") {
			env = append(env, kv)
		}
	}
	nowhere := t.TempDir() + "/missing"
	cmd := exec.Command(os.Args[0], "-test.run=^TestServerRefusesToStartWithoutBedrockCredentials$")
	cmd.Env = append(env, "HAUSY_TEST_RUN_MAIN=1", "HAUSY_LLM=local", "HAUSY_WRITER_LLM=bedrock", "HAUSY_LOG_LEVEL=info",
		"HAUSY_API_ADDR=127.0.0.1:0", "DATABASE_URI=postgresql://hausy:hausy@127.0.0.1:1/hausy",
		"AWS_CONFIG_FILE="+nowhere, "AWS_SHARED_CREDENTIALS_FILE="+nowhere, "AWS_EC2_METADATA_DISABLED=true", "AWS_REGION=us-east-1")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(out), `"msg":"startup_failed"`) || !strings.Contains(string(out), `"stage":"aws_credentials"`) {
		t.Fatalf("want exit status 1 naming the AWS credentials, got %v:\n%s", err, out)
	}
}
