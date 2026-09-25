// Command hausy serves the buyer agent to the local web application.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/bedrock"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/clarification"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/jev"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
	matchingjev "github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching/jev"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
)

type serverConfig struct {
	address     string
	llmProvider string
	// writerProvider runs the reply writer; it defaults to llmProvider.
	writerProvider string
	llmURL         string
	llmToken       string
	llmModel       string
	bedrockModel   string
	databaseURI    string
}

func serverConfigFromEnv() serverConfig {
	return serverConfig{
		address:        envOrDefault("HAUSY_API_ADDR", "127.0.0.1:8080"),
		llmProvider:    envOrDefault("HAUSY_LLM", "local"),
		writerProvider: envOrDefault("HAUSY_WRITER_LLM", envOrDefault("HAUSY_LLM", "local")),
		llmURL:         envOrDefault("LOCAL_LLM_URL", "http://127.0.0.1:8000"),
		llmToken:       os.Getenv("LOCAL_LLM_TOKEN"),
		llmModel:       envOrDefault("LOCAL_LLM_MODEL", "Qwen3.5-9B-4bit"),
		bedrockModel:   envOrDefault("BEDROCK_MODEL_ID", "us.anthropic.claude-sonnet-4-6"),
		databaseURI:    envOrDefault("DATABASE_URI", "postgresql://hausy:hausy@localhost:5432/hausy"),
	}
}

// newLLMClient returns the model behind the writer, the planner's fallback and
// the no-database chat. HAUSY_LLM picks it: "local" (the default) or
// "bedrock", which signs with the ambient AWS credentials (AWS_PROFILE) and
// defaults to us-east-1. Any other value is an error, so a typo never
// silently measures the wrong model.
func newLLMClient(ctx context.Context, c serverConfig) (llm.Client, error) {
	switch c.llmProvider {
	case "local":
		return local.NewClient(c.llmURL, c.llmToken, c.llmModel), nil
	case "bedrock":
		cfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("load AWS config: %w", err)
		}
		if cfg.Region == "" {
			cfg.Region = "us-east-1"
		}
		return bedrock.New(bedrockruntime.NewFromConfig(cfg), c.bedrockModel), nil
	}
	return nil, fmt.Errorf("HAUSY_LLM=%q: want local or bedrock", c.llmProvider)
}

// writerClient returns the model behind the reply writer. HAUSY_WRITER_LLM
// ("local" or "bedrock") separates it from HAUSY_LLM, so replies can run on
// Bedrock while planning and local development stay on the local model.
// Unset, the writer shares the model client.
func writerClient(ctx context.Context, c serverConfig, shared llm.Client) (llm.Client, error) {
	if c.writerProvider == c.llmProvider {
		return shared, nil
	}
	c.llmProvider = c.writerProvider
	client, err := newLLMClient(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("HAUSY_WRITER_LLM: %w", err)
	}
	return client, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func logLevelFromEnv() (slog.Level, error) {
	switch strings.ToLower(envOrDefault("HAUSY_LOG_LEVEL", "info")) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("HAUSY_LOG_LEVEL: want debug, info, warn, or error")
	}
}

func main() {
	level, err := logLevelFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With("service", "hausy"))
	config := serverConfigFromEnv()
	llmClient, err := newLLMClient(context.Background(), config)
	if err != nil {
		slog.Error("startup_failed", "stage", "model", "error_class", logging.ErrorClass(err))
		os.Exit(1)
	}
	writer, err := writerClient(context.Background(), config, llmClient)
	if err != nil {
		slog.Error("startup_failed", "stage", "writer_model", "error_class", logging.ErrorClass(err))
		os.Exit(1)
	}
	slog.Info("model_configured", "provider", config.llmProvider, "writer_provider", config.writerProvider)

	// A missing database degrades the agent rather than stopping it: without
	// one it can still take requirements down, and saying so at startup beats
	// a fresh clone failing to boot before anything has been ingested.
	var options []buyer.Option
	var qualifications httpapi.Qualifications
	var accounts auth.Store = auth.NewMemoryStore()
	var catalog agency.Catalog = agency.NewMemoryCatalog()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	storeStage := "open"
	store, err := postgres.Open(ctx, config.databaseURI)
	if err == nil {
		storeStage = "migrate"
		// Accounts need their tables even on a database no one has loaded yet.
		if err = store.Migrate(ctx); err != nil {
			store.Close()
		}
	}
	cancel()
	if err != nil {
		slog.Warn("database_unavailable", "stage", storeStage, "error_class", logging.ErrorClass(err), "mode", "requirements_only")
	} else {
		defer store.Close()
		options = append(options, buyer.WithPipeline(plannerFromEnv(llmClient), store, buyer.LocalWriter{Client: writer}))
		// Jev judges ambiguity and the schema supplies the options (ADR 0004).
		// Without a gateway key only the deterministic checks ask.
		judge := clarification.Judge{Answers: clarification.Model{Client: llmClient}}
		if key := os.Getenv("AI_GATEWAY_API_KEY"); key != "" {
			judge.Evaluate = jev.New(jev.GatewayURL, key, nil).Evaluate
			options = append(options, buyer.WithMatching(matchingjev.New(matchingjev.GatewayURL, key, nil)))
		}
		options = append(options, buyer.WithClarifier(judge))
		accounts = store
		qualifications = store
		catalog = store
		slog.Info("database_connected", "mode", "search")
	}

	agent := buyer.NewAgent(llmClient, options...)
	// ponytail: the only Provider today is Local; an AWS Cognito Provider would
	// be chosen here from configuration without changing httpapi or the frontend.
	provider := auth.NewLocal(accounts)
	server := &http.Server{
		Addr:              config.address,
		Handler:           httpapi.NewHandler(agent, provider, catalog, qualifications),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("server_listening", "address", config.address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server_failed", "error_class", logging.ErrorClass(err))
		os.Exit(1)
	}
}

// plannerFromEnv reads HAUSY_PLANNER. "jev" plans with Jev under a 6 s
// budget and falls back to the local model; anything else plans with the
// local model alone. Switching Jev off is one environment variable.
func plannerFromEnv(client llm.Client) intake.Planner {
	local := intake.Qwen{Client: client}
	if os.Getenv("HAUSY_PLANNER") != "jev" {
		return intake.Planner{Primary: local}
	}
	gateway := jev.New(jev.GatewayURL, os.Getenv("AI_GATEWAY_API_KEY"), nil)
	slog.Info("planner_configured", "primary", "jev", "fallback", "model")
	return intake.Planner{Primary: intake.Jev{Evaluate: gateway.Evaluate}, Fallback: local, Budget: 6 * time.Second}
}
