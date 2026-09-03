// Command hausy serves the buyer agent to the local web application.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
)

type serverConfig struct {
	address     string
	llmURL      string
	llmToken    string
	llmModel    string
	databaseURI string
}

func serverConfigFromEnv() serverConfig {
	return serverConfig{
		address:     envOrDefault("HAUSY_API_ADDR", "127.0.0.1:8080"),
		llmURL:      envOrDefault("LOCAL_LLM_URL", "http://127.0.0.1:8000"),
		llmToken:    os.Getenv("LOCAL_LLM_TOKEN"),
		llmModel:    envOrDefault("LOCAL_LLM_MODEL", "Qwen3.5-9B-4bit"),
		databaseURI: envOrDefault("DATABASE_URI", "postgresql://hausy:hausy@localhost:5432/hausy"),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	config := serverConfigFromEnv()
	llmClient := local.NewClient(config.llmURL, config.llmToken, config.llmModel)

	// A missing database degrades the agent rather than stopping it: without
	// one it can still take requirements down, and saying so at startup beats
	// a fresh clone failing to boot before anything has been ingested.
	var options []buyer.Option
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	store, err := postgres.Open(ctx, config.databaseURI)
	cancel()
	if err != nil {
		log.Printf("no listing store at %s (%v); the agent will collect requirements but cannot search", config.databaseURI, err)
	} else {
		defer store.Close()
		options = append(options, buyer.WithInventory(store))
		log.Printf("listing store connected at %s", config.databaseURI)
	}

	agent := buyer.NewAgent(llmClient, options...)
	server := &http.Server{
		Addr:              config.address,
		Handler:           httpapi.NewHandler(agent),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Hausy API listening on http://%s", config.address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
