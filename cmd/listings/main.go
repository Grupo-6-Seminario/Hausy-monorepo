// Command listings turns scraped ZonaProp rows into database records.
//
// It is three steps, run separately:
//
//	listings parse       -in data/listings.jsonl -out data/listings.parsed.jsonl
//	listings eligibility -in data/listings.parsed.jsonl -out data/listings.eligibility.jsonl
//	listings load        -in data/listings.parsed.jsonl
//
// parse and eligibility run a model; they are slow, non-deterministic and
// deliberate, and their output is committed. parse runs the local model and is slow, non-deterministic and occasional.
// load is fast, deterministic and repeatable. Committing the parsed file
// between them is what lets every teammate build the same database without
// scraping the site or running a model of their own.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/jev"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/local"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/pipeline"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
	}

	var err error
	switch os.Args[1] {
	case "parse":
		err = runParse(os.Args[2:])
	case "eligibility":
		err = runEligibility(os.Args[2:])
	case "load":
		err = runLoad(os.Args[2:])
	case "audit":
		err = runAudit(os.Args[2:])
	case "snapshot":
		err = runSnapshot(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: listings <command> [flags]

  parse  -in <scraped.jsonl> -out <parsed.jsonl>   extract attributes with the local model
  eligibility -in <parsed.jsonl> -out <eligibility.jsonl>   extract eligibility rules with Jev
  load   -in <parsed.jsonl>                        load parsed listings and committed reviews into Postgres
  audit  -database <postgres URI> -out <quality.jsonl>  review stored listings and write a snapshot
  snapshot -database <postgres URI> -out <quality.jsonl>  export completed reviews without model calls
`)
	os.Exit(2)
}

func runSnapshot(args []string) error {
	flags := flag.NewFlagSet("snapshot", flag.ExitOnError)
	uri := flags.String("database", envOrDefault("DATABASE_URI", "postgresql://hausy:hausy@localhost:5432/hausy"), "Postgres URI")
	in := flags.String("in", "data/listings.parsed.jsonl", "committed listings to include")
	out := flags.String("out", "data/listings.quality.jsonl", "review snapshot to commit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, *uri)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return err
	}
	return writeReviewSnapshot(ctx, store, *in, *out)
}

func runAudit(args []string) error {
	flags := flag.NewFlagSet("audit", flag.ExitOnError)
	uri := flags.String("database", envOrDefault("DATABASE_URI", "postgresql://hausy:hausy@localhost:5432/hausy"), "Postgres URI")
	in := flags.String("in", "data/listings.parsed.jsonl", "committed listings to include in snapshot")
	out := flags.String("out", "data/listings.quality.jsonl", "review snapshot to commit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if os.Getenv("AI_GATEWAY_API_KEY") == "" {
		return fmt.Errorf("audit requires AI_GATEWAY_API_KEY")
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, *uri)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return err
	}
	queue, err := store.ReviewQueue(ctx)
	if err != nil {
		return err
	}
	gate := jev.New(jev.GatewayURL, os.Getenv("AI_GATEWAY_API_KEY"), nil)
	gate.Retries, gate.Backoff = 3, 2*time.Second
	model := local.NewClient(envOrDefault("LOCAL_LLM_URL", "http://127.0.0.1:8000"), os.Getenv("LOCAL_LLM_TOKEN"), envOrDefault("LOCAL_LLM_MODEL", "Qwen3.5-9B-4bit"))
	passed, withheld, failed := 0, 0, 0
	for start := 0; start < len(queue); start += 10 {
		end := min(start+10, len(queue))
		reviews, errs := quality.AuditBatch(ctx, queue[start:end], gate.Evaluate, model)
		for i, item := range queue[start:end] {
			review, err := reviews[i], errs[i]
			if err == nil {
				err = store.SaveReview(ctx, item.URL, review)
			}
			if err != nil {
				failed++
				if failed <= 10 {
					log.Printf("audit %s: %v", item.URL, err)
				}
				continue
			}
			if review.Status == quality.Withheld {
				withheld++
			} else {
				passed++
			}
		}
		log.Printf("audit: reviewed %d/%d", end, len(queue))
		if end < len(queue) {
			time.Sleep(3 * time.Second)
		}
	}
	quarantined, err := store.QuarantineLegacy(ctx)
	if err != nil {
		return err
	}
	log.Printf("audit: passed %d, withheld %d, pending %d", passed, withheld, failed)
	if quarantined > 0 {
		log.Printf("audit: quarantined %d unresolved legacy listings", quarantined)
	}
	if err := writeReviewSnapshot(ctx, store, *in, *out); err != nil {
		return err
	}
	if failed > 0 {
		return fmt.Errorf("audit left %d listings pending; rerun to resume", failed)
	}
	return nil
}

func writeReviewSnapshot(ctx context.Context, store *postgres.Store, inputPath, path string) error {
	input, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	committedURLs, err := pipeline.ParsedURLs(input)
	input.Close()
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".quality-*.jsonl")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	count, err := store.ExportReviews(ctx, file, committedURLs)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	log.Printf("quality snapshot: wrote %d completed reviews to %s", count, path)
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func runParse(args []string) error {
	flags := flag.NewFlagSet("parse", flag.ExitOnError)
	in := flags.String("in", "data/listings.jsonl", "scraped JSONL to read")
	out := flags.String("out", "data/listings.parsed.jsonl", "parsed JSONL to write")
	resume := flags.Bool("resume", true, "skip urls already present in -out")
	if err := flags.Parse(args); err != nil {
		return err
	}

	client := local.NewClient(
		envOrDefault("LOCAL_LLM_URL", "http://127.0.0.1:8000"),
		os.Getenv("LOCAL_LLM_TOKEN"),
		envOrDefault("LOCAL_LLM_MODEL", "Qwen3.5-9B-4bit"),
	)
	parser := listing.NewParser(client, envOrDefault("LOCAL_LLM_MODEL", "Qwen3.5-9B-4bit"))

	input, err := os.Open(*in)
	if err != nil {
		return err
	}
	defer input.Close()

	// Resuming appends, so an interrupted run keeps what it already paid for.
	alreadyParsed := map[string]bool{}
	openFlags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if *resume {
		if existing, err := os.Open(*out); err == nil {
			alreadyParsed, err = pipeline.ParsedURLs(existing)
			existing.Close()
			if err != nil {
				return err
			}
			if len(alreadyParsed) > 0 {
				log.Printf("resuming: %d listings already parsed", len(alreadyParsed))
				openFlags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
			}
		}
	}

	output, err := os.OpenFile(*out, openFlags, 0o644)
	if err != nil {
		return err
	}
	defer output.Close()

	// The per-request deadline is the LLM client's own HTTP timeout.
	report, err := pipeline.Parse(context.Background(), input, output, parser, alreadyParsed)
	if err != nil {
		return err
	}

	log.Printf("parsed %d, skipped %d, already done %d, failed %d",
		report.Parsed, report.Skipped, report.AlreadyDone, report.Failed)
	reportErrors(report)
	return nil
}

func runEligibility(args []string) error {
	flags := flag.NewFlagSet("eligibility", flag.ExitOnError)
	in := flags.String("in", "data/listings.parsed.jsonl", "parsed JSONL to read")
	out := flags.String("out", "data/listings.eligibility.jsonl", "eligibility JSONL to write (appends; resumable)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	client := jev.New(jev.GatewayURL, os.Getenv("AI_GATEWAY_API_KEY"), nil)
	// A batch run can wait out Gateway's rate limit; a chat turn cannot.
	client.Retries, client.Backoff = 6, time.Second
	extract := func(ctx context.Context, description string) ([]eligibility.Rule, error) {
		return eligibility.Extract(ctx, client.Evaluate, description)
	}

	done := map[string]bool{}
	if existing, err := os.Open(*out); err == nil {
		done, err = pipeline.ParsedURLs(existing)
		existing.Close()
		if err != nil {
			return err
		}
	}
	input, err := os.Open(*in)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(*out, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer output.Close()

	report, err := pipeline.ExtractEligibility(context.Background(), input, output, extract, done)
	if err != nil {
		return err
	}
	log.Printf("examined %d, already done %d, failed %d", report.Parsed, report.AlreadyDone, report.Failed)
	reportErrors(report)
	return nil
}

func runLoad(args []string) error {
	flags := flag.NewFlagSet("load", flag.ExitOnError)
	in := flags.String("in", "data/listings.parsed.jsonl", "parsed JSONL to read")
	eligibilityIn := flags.String("eligibility", "data/listings.eligibility.jsonl", "eligibility JSONL to read; skipped if absent")
	qualityIn := flags.String("quality", "data/listings.quality.jsonl", "quality review JSONL to read; skipped if absent")
	uri := flags.String("database", envOrDefault("DATABASE_URI", "postgresql://hausy:hausy@localhost:5432/hausy"), "Postgres URI")
	fresh := flags.Bool("fresh", false, "empty the listing tables before loading")
	if err := flags.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	store, err := postgres.Open(ctx, *uri)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		return err
	}
	if *fresh {
		if err := store.DeleteAll(ctx); err != nil {
			return err
		}
	}

	input, err := os.Open(*in)
	if err != nil {
		return err
	}
	defer input.Close()

	report, err := pipeline.Load(ctx, input, store)
	if err != nil {
		return err
	}

	total, err := store.Count(ctx)
	if err != nil {
		return err
	}
	agencies, err := store.CountAgencies(ctx)
	if err != nil {
		return err
	}

	log.Printf("loaded %d, skipped %d, failed %d", report.Loaded, report.Skipped, report.Failed)
	if rules, err := os.Open(*eligibilityIn); err == nil {
		eligibilityReport, err := pipeline.LoadEligibility(ctx, rules, store)
		rules.Close()
		if err != nil {
			return err
		}
		log.Printf("eligibility: loaded %d, failed %d", eligibilityReport.Loaded, eligibilityReport.Failed)
		reportErrors(eligibilityReport)
	} else {
		log.Printf("eligibility: %s not found; every listing stays unknown", *eligibilityIn)
	}
	if reviews, err := os.Open(*qualityIn); err == nil {
		qualityReport, err := pipeline.LoadQuality(ctx, reviews, store)
		reviews.Close()
		if err != nil {
			return err
		}
		log.Printf("quality: loaded %d, failed %d", qualityReport.Loaded, qualityReport.Failed)
		reportErrors(qualityReport)
	} else {
		log.Printf("quality: %s not found; new listings stay pending", *qualityIn)
	}
	log.Printf("database now holds %d listings across %d agencies", total, agencies)
	reportErrors(report)
	return nil
}

// reportErrors prints a bounded sample: a run of three hundred can fail in
// three hundred ways, and a wall of identical errors hides the useful one.
func reportErrors(report pipeline.Report) {
	const shown = 10
	for i, err := range report.Errors {
		if i == shown {
			log.Printf("  ... and %d more", len(report.Errors)-shown)
			break
		}
		log.Printf("  %v", err)
	}
}
