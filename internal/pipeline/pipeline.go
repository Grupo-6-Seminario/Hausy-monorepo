// Package pipeline moves listings from the scraper's output into the store.
//
// It is split into two steps on purpose. Parse runs the model and writes its
// results to a file; Load reads that file into Postgres. Keeping them apart is
// what makes the data reproducible for a team: the parsed file is committed,
// so every teammate loads the identical attributes instead of each running a
// model of their own and getting a slightly different reading of every
// listing.
package pipeline

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

// AttributeParser extracts the fuzzy qualities of a property from its prose.
type AttributeParser interface {
	Parse(ctx context.Context, description string) ([]listing.Attribute, error)
	Model() string
}

// Sink persists a listing.
type Sink interface {
	Save(ctx context.Context, item listing.Listing) error
}

// Report counts what a run did. Nothing here is fatal on its own: a single bad
// listing among hundreds is expected, and is reported rather than thrown.
type Report struct {
	Parsed      int
	Loaded      int
	Skipped     int
	AlreadyDone int
	Failed      int
	Errors      []error
}

func (r *Report) fail(err error) {
	r.Failed++
	r.Errors = append(r.Errors, err)
}

// scannerFor returns a line scanner sized for listing descriptions, which
// routinely exceed bufio's default 64KB line limit.
func scannerFor(input io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	return scanner
}

// Parse reads scraped rows, normalizes them, asks the parser for the qualities
// that exist only in prose, and writes the result as JSONL.
//
// URLs in alreadyParsed are passed over, so an interrupted run resumes instead
// of paying for every completion a second time.
func Parse(ctx context.Context, input io.Reader, output io.Writer, parser AttributeParser, alreadyParsed map[string]bool) (Report, error) {
	var report Report

	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)

	scanner := scannerFor(input)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw listing.Raw
		if err := json.Unmarshal(line, &raw); err != nil {
			report.fail(fmt.Errorf("line %d: %w", lineNumber, err))
			continue
		}
		if err := raw.Validate(); err != nil {
			report.Skipped++
			continue
		}
		if alreadyParsed[raw.URL] {
			report.AlreadyDone++
			continue
		}

		item := listing.Normalize(raw)

		// The deterministic attributes are established before the model runs,
		// so a model failure below still leaves them in place.
		extracted, err := parser.Parse(ctx, item.Description)
		if err != nil {
			report.fail(fmt.Errorf("%s: %w", item.URL, err))
		} else {
			item.Attributes = mergeAttributes(item.Attributes, extracted)
			item.ParserModel = parser.Model()
			parsedAt := time.Now().UTC()
			item.ParsedAt = &parsedAt
			report.Parsed++
		}

		if err := encoder.Encode(item); err != nil {
			return report, fmt.Errorf("pipeline: write %s: %w", item.URL, err)
		}
	}
	return report, scanner.Err()
}

// mergeAttributes keeps the deterministic readings and adds the model's, minus
// any type the page already settled.
//
// A published field outranks prose: if the listing states its disposition, a
// model reading "contrafrente" into the description does not get to contradict
// it, and storing both would leave the listing claiming two exposures at once.
func mergeAttributes(established, extracted []listing.Attribute) []listing.Attribute {
	settled := make(map[string]bool, len(established))
	for _, attribute := range established {
		settled[attribute.Type] = true
	}

	merged := append([]listing.Attribute{}, established...)
	seen := make(map[[2]string]bool, len(extracted))
	for _, attribute := range extracted {
		if settled[attribute.Type] {
			continue
		}
		key := [2]string{attribute.Type, attribute.Value}
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, attribute)
	}
	return merged
}

// KeepParsed copies to output the rows an earlier Parse run finished and
// returns their URLs, so a resumed run skips only those. A row written after a
// failed model call is dropped rather than skipped: the retry replaces it
// instead of sitting beside it as a duplicate.
func KeepParsed(input io.Reader, output io.Writer) (map[string]bool, error) {
	urls := make(map[string]bool)

	scanner := scannerFor(input)
	for scanner.Scan() {
		line := scanner.Bytes()
		var item listing.Listing
		// A truncated final line from an interrupted run is not an error; it
		// simply means that URL still needs parsing.
		if len(line) == 0 || json.Unmarshal(line, &item) != nil || item.URL == "" || item.ParsedAt == nil {
			continue
		}
		urls[item.URL] = true
		if _, err := fmt.Fprintf(output, "%s\n", line); err != nil {
			return nil, err
		}
	}
	return urls, scanner.Err()
}

// ParsedURLs reads every URL an earlier Parse run wrote, whether or not its
// model call succeeded: the rows the committed file carries. Resuming a parse
// uses KeepParsed instead.
func ParsedURLs(input io.Reader) (map[string]bool, error) {
	urls := make(map[string]bool)

	scanner := scannerFor(input)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item listing.Listing
		// A truncated final line from an interrupted run is not an error.
		if err := json.Unmarshal(line, &item); err != nil {
			continue
		}
		if item.URL != "" {
			urls[item.URL] = true
		}
	}
	return urls, scanner.Err()
}

// Load reads parsed rows into the store. It is deterministic and repeatable:
// the same file loaded twice leaves the same database.
func Load(ctx context.Context, input io.Reader, sink Sink) (Report, error) {
	var report Report

	scanner := scannerFor(input)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var item listing.Listing
		if err := json.Unmarshal(line, &item); err != nil {
			report.fail(fmt.Errorf("line %d: %w", lineNumber, err))
			continue
		}
		if item.URL == "" {
			report.Skipped++
			continue
		}
		// The model's reading stays in the committed file; what the search
		// may rely on is decided here, so changing it needs only a reload.
		item.Attributes = listing.StatedAmenities(item.Attributes)
		if err := sink.Save(ctx, item); err != nil {
			report.fail(err)
			continue
		}
		report.Loaded++
	}
	return report, scanner.Err()
}
