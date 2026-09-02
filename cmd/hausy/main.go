// Command hausy serves the buyer agent to the local web application.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
)

type serverConfig struct {
	address  string
	llmURL   string
	llmToken string
	llmModel string
}

func serverConfigFromEnv() serverConfig {
	return serverConfig{
		address:  envOrDefault("HAUSY_API_ADDR", "127.0.0.1:8080"),
		llmURL:   envOrDefault("LOCAL_LLM_URL", "http://127.0.0.1:8000"),
		llmToken: os.Getenv("LOCAL_LLM_TOKEN"),
		llmModel: envOrDefault("LOCAL_LLM_MODEL", "Qwen3.5-9B-4bit"),
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
	agent := buyer.NewAgent(llmClient)
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
