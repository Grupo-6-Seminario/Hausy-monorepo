package main

import "testing"

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
