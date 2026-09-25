// Command parse-audit runs the production attribute parser over a fixed sample
// of scraped descriptions and writes what it extracted, with evidence and
// latency, so its accuracy can be judged before a full re-parse.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
)

type row struct {
	URL         string              `json:"url"`
	Description string              `json:"description"`
	Attributes  []listing.Attribute `json:"attributes"`
	Error       string              `json:"error,omitempty"`
	MS          int64               `json:"ms"`
}

func main() {
	in := flag.String("in", "data/listings.jsonl", "scraped JSONL")
	every := flag.Int("every", 10, "take every Nth listing")
	flag.Parse()
	model := envOr("LOCAL_LLM_MODEL", "Qwen3.5-9B-4bit")
	parser := listing.NewParser(local.NewClient(envOr("LOCAL_LLM_URL", "http://127.0.0.1:8000"), os.Getenv("LOCAL_LLM_TOKEN"), model), model)
	file, err := os.Open(*in)
	if err != nil {
		fail(err)
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<22)
	out := json.NewEncoder(os.Stdout)
	out.SetEscapeHTML(false)
	for i := 0; scanner.Scan(); i++ {
		if i%*every != 0 {
			continue
		}
		var raw listing.Raw
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			fail(err)
		}
		item := listing.Normalize(raw)
		started := time.Now()
		attributes, err := parser.Parse(context.Background(), item.Description)
		r := row{URL: item.URL, Description: item.Description, Attributes: attributes, MS: time.Since(started).Milliseconds()}
		if err != nil {
			r.Error = err.Error()
		}
		_ = out.Encode(r)
		fmt.Fprintf(os.Stderr, "%d attrs=%d ms=%d err=%v\n", i, len(attributes), r.MS, err != nil)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
