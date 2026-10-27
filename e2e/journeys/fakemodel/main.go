// Command fakemodel stands in for the local OpenAI-compatible model in the
// journeys suite. It answers the planner with a canned plan per message and
// the writer with one fixed reply, so a journey measures the app and the
// database, never a model. It also serves the publication pages that the
// fixture listings link to.
//
//	go run ./e2e/journeys/fakemodel -addr 127.0.0.1:8710
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// plans maps the searcher's latest message to the plan the planner returns.
// The journeys type exactly these messages.
var plans = map[string]string{
	"Busco alquiler en Palermo y tengo seguro de caución": `{"intent":"new_search","branches":[{"neighborhoods":["palermo"],"operation":"alquiler"}],"qualification":{"guarantee":["caucion"]}}`,
	"Busco alquiler en Palermo con amenities":             `{"intent":"new_search","branches":[{"neighborhoods":["palermo"],"operation":"alquiler"}]}`,
	"Busco alquiler en Palermo":                           `{"intent":"new_search","branches":[{"neighborhoods":["palermo"],"operation":"alquiler"}]}`,
	"Busco alquiler en Monserrat":                         `{"intent":"new_search","branches":[{"neighborhoods":["monserrat"],"operation":"alquiler"}]}`,
}

const writerReply = "Estas son las propiedades del catálogo de prueba que cumplen lo que pediste."

const (
	plannerPrompt = "Convertí la conversación"
	writerPrompt  = "Sos el agente de Hausy"
)

var latestMessage = regexp.MustCompile(`(?m)^Mensaje \d+ del usuario: (.*)$`)

type chatRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Stream bool `json:"stream"`
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8710", "listen address")
	flag.Parse()
	log.Printf("fakemodel listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, handler()))
}

func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", complete)
	mux.HandleFunc("GET /publicacion/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><title>Publicación %[1]s</title><h1>Publicación %[1]s</h1>", r.PathValue("id"))
	})
	return mux
}

func complete(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) < 2 {
		http.Error(w, "fakemodel: want a system and a user message", http.StatusBadRequest)
		return
	}
	system, user := req.Messages[0].Content, req.Messages[len(req.Messages)-1].Content
	var content string
	switch {
	case strings.HasPrefix(system, plannerPrompt):
		matches := latestMessage.FindAllStringSubmatch(user, -1)
		if len(matches) > 0 {
			content = plans[strings.TrimSpace(matches[len(matches)-1][1])]
		}
	case strings.HasPrefix(system, writerPrompt):
		content = writerReply
	}
	if content == "" {
		// A request no journey expects is a broken journey or a changed
		// prompt; fail loudly with what was asked.
		log.Printf("fakemodel: unexpected request: %.80q / %q", system, user)
		http.Error(w, fmt.Sprintf("fakemodel: no canned answer for %.80q / %q", system, user), http.StatusInternalServerError)
		return
	}
	if !req.Stream {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": content}}}})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	for _, word := range strings.SplitAfter(content, " ") {
		chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": word}}}})
		fmt.Fprintf(w, "data: %s\n\n", chunk)
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}
