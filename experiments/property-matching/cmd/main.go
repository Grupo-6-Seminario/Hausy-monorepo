package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	comparison "github.com/Grupo-6-Seminario/proyecto-angus-back/experiments/property-matching"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	root := flag.String("root", ".", "repository root")
	fixtures := flag.String("fixtures", "experiments/property-matching/fixtures/cases.json", "frozen fixture file")
	output := flag.String("output", "experiments/property-matching/output/comparison.json", "JSON result path")
	selected := flag.String("cases", "", "comma-separated case IDs, empty for all")
	live := flag.Bool("live", false, "generate paired prose using the local provider only")
	endpoint := flag.String("local-url", "http://127.0.0.1:8000", "loopback model endpoint")
	model := flag.String("model", "Qwen3.5-9B-4bit", "installed local model")
	flag.Parse()
	if *live {
		if err := loopbackEndpoint(*endpoint); err != nil {
			return err
		}
	}

	cases, hashes, err := comparison.Load(*root, *fixtures)
	if err != nil {
		return err
	}
	identity, err := baselineIdentity(*root)
	if err != nil {
		return err
	}
	report := struct {
		BaselineIdentity map[string]string              `json:"baseline_source_identity"`
		Created          time.Time                      `json:"created"`
		BaselineRevision string                         `json:"baseline_revision"`
		Runtime          string                         `json:"runtime"`
		Machine          string                         `json:"machine"`
		Hashes           map[string]string              `json:"sha256"`
		Retrieval        string                         `json:"retrieval_boundary"`
		Baseline         string                         `json:"baseline_boundary"`
		Policy           string                         `json:"proposed_policy"`
		Semantics        string                         `json:"new_noise_semantics"`
		Cases            []comparison.Comparison        `json:"cases"`
		Live             map[string]comparison.LivePair `json:"live,omitempty"`
		Correct          bool                           `json:"all_literal_orders_pass"`
	}{BaselineIdentity: identity, Created: time.Now().UTC(), BaselineRevision: identity["git_revision"], Runtime: runtime.Version(), Machine: runtime.GOOS + "/" + runtime.GOARCH, Hashes: hashes, Retrieval: "Offline in-memory adapter over selected frozen rows and synthetic controls. Complete within each fixture only. Published numeric/location/attribute gates and content-bound snapshot publication reviews applied; no SQL, paging, archive or live inventory proof.", Baseline: "Actual buyer.Agent with frozen intake planner, recording classifier and failing recording writer to expose actual deterministic fallback. Global URL dedup and max 10 results preserved.", Policy: "weighted-net-fit-v1, each branch finalized independently with all branch memberships and denominators retained", Semantics: "Current buyer semantically evaluates natural light only. Proposed noise uses recorded direct-property support/contradiction assessments; noise changes are additional rubric behavior, not isolated score-policy benefit. Live classifier accuracy unverified.", Cases: []comparison.Comparison{}, Live: map[string]comparison.LivePair{}, Correct: true}
	ctx := logging.WithLogger(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, c := range cases {
		if *selected != "" && !contains(strings.Split(*selected, ","), c.ID) {
			continue
		}
		result, err := comparison.Run(ctx, c)
		if err != nil {
			return fmt.Errorf("case %s: %w", c.ID, err)
		}
		report.Cases = append(report.Cases, result)
		report.Correct = report.Correct && result.Correct
		if *live {
			caseCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			pair, err := comparison.GeneratePair(caseCtx, c, local.NewClient(*endpoint, "", *model))
			cancel()
			if err != nil {
				return err
			}
			report.Live[c.ID] = pair
		}
	}
	if len(report.Cases) == 0 {
		return fmt.Errorf("no matching case IDs")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(*output, append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("%d cases; all literal orders pass=%t; %s\n", len(report.Cases), report.Correct, *output)
	if !report.Correct {
		return fmt.Errorf("literal order comparison failed")
	}
	return nil
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func loopbackEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid local endpoint: %w", err)
	}
	if u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
		return fmt.Errorf("live experiment permits loopback endpoints without userinfo only")
	}
	return nil
}
func baselineIdentity(root string) (map[string]string, error) {
	identity := map[string]string{}
	revision, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return nil, err
	}
	identity["git_revision"] = strings.TrimSpace(string(revision))
	for _, dir := range []string{"internal/buyer", "internal/intake", "internal/eligibility", "internal/search", "internal/listing", "internal/matching", "internal/local"} {
		files, err := filepath.Glob(filepath.Join(root, dir, "*.go"))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") || filepath.Base(file) == "fit.go" {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(root, file)
			if err != nil {
				return nil, err
			}
			identity[rel] = fmt.Sprintf("%x", sha256.Sum256(data))
		}
	}
	return identity, nil
}
