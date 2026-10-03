package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

// EligibilityRecord is one line of data/listings.eligibility.jsonl. A record
// with no rules means the listing was examined and publishes nothing, which
// is what makes its eligibility "unknown" rather than unexamined.
type EligibilityRecord struct {
	URL         string             `json:"url"`
	Rules       []eligibility.Rule `json:"rules"`
	ExtractedAt time.Time          `json:"extracted_at"`
}

// ExtractEligibility reads parsed listings and writes their eligibility rules.
// Like Parse it is resumable; unlike Parse a failed listing is not written,
// so the resumed run retries it.
func ExtractEligibility(ctx context.Context, input io.Reader, output io.Writer, extract func(context.Context, string) ([]eligibility.Rule, error), alreadyDone map[string]bool) (Report, error) {
	var report Report
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	scanner := scannerFor(input)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var item listing.Listing
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			report.fail(fmt.Errorf("line %d: %w", lineNumber, err))
			continue
		}
		if alreadyDone[item.URL] {
			report.AlreadyDone++
			continue
		}
		rules, err := extract(ctx, item.Description)
		if err != nil {
			report.fail(fmt.Errorf("%s: %w", item.URL, err))
			continue
		}
		if rules == nil {
			rules = []eligibility.Rule{}
		}
		if err := encoder.Encode(EligibilityRecord{URL: item.URL, Rules: rules, ExtractedAt: time.Now().UTC()}); err != nil {
			return report, fmt.Errorf("pipeline: write %s: %w", item.URL, err)
		}
		report.Parsed++
	}
	return report, scanner.Err()
}

// EligibilitySink persists a listing's rules and reads the loaded listing
// whose published attributes add rules of their own.
type EligibilitySink interface {
	ByURL(ctx context.Context, url string) (listing.Listing, error)
	Facts(ctx context.Context) (eligibility.Catalog, error)
	SaveEligibility(ctx context.Context, url string, rules []eligibility.Rule) error
}

// LoadEligibility reads data/listings.eligibility.jsonl into the store, adding
// the rules each listing's published attributes state. Like Load it is
// deterministic and repeatable.
func LoadEligibility(ctx context.Context, input io.Reader, sink EligibilitySink) (Report, error) {
	var report Report
	// Published pets rules list the pets choices a listing admits.
	catalog, err := sink.Facts(ctx)
	if err != nil {
		return report, err
	}
	scanner := scannerFor(input)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var record EligibilityRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			report.fail(fmt.Errorf("line %d: %w", lineNumber, err))
			continue
		}
		if record.Rules == nil {
			record.Rules = []eligibility.Rule{}
		}
		item, err := sink.ByURL(ctx, record.URL)
		if err != nil {
			report.fail(fmt.Errorf("%s: %w", record.URL, err))
			continue
		}
		rules := append(record.Rules, eligibility.PublishedRules(item.Attributes, catalog)...)
		if err := sink.SaveEligibility(ctx, record.URL, rules); err != nil {
			report.fail(err)
			continue
		}
		report.Loaded++
	}
	return report, scanner.Err()
}
