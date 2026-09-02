package buyer_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
)

func TestAgent_LiveLocalModel_Integration(t *testing.T) {
	baseURL := os.Getenv("LOCAL_LLM_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}
	token := os.Getenv("LOCAL_LLM_TOKEN")
	if token == "" {
		token = "2262a7b268ac07e14612762b9c039ad040e4736911d45e73e706ff298a84038c"
	}

	// Quick check if local server is listening
	client := &http.Client{Timeout: 1 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		t.Skip("skipping integration test: cannot create request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("skipping integration test: local model not reachable at %s (%v)", baseURL, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("skipping integration test: local model returned status %d", resp.StatusCode)
	}

	localClient := local.NewClient(baseURL, token, "Qwen3.5-9B-4bit")
	agent := buyer.NewAgent(localClient)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	res, err := agent.HandleMessage(ctx, "live-session-1", "Busco un departamento de dos ambientes luminoso en Palermo")
	if err != nil {
		t.Fatalf("agent.HandleMessage failed against live model: %v", err)
	}

	if len(res.Requirements) == 0 {
		t.Errorf("expected at least one extracted requirement from live model, got none")
	}

	t.Logf("Live model reply: %s", res.Reply)
	t.Logf("Live model extracted requirements: %+v", res.Requirements)
}
