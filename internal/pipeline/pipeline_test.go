package pipeline_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/pipeline"
)

type fakeParser struct {
	attributes []listing.Attribute
	err        error
	calls      int
}

func (p *fakeParser) Parse(context.Context, string) ([]listing.Attribute, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return p.attributes, nil
}

func (p *fakeParser) Model() string { return "fake-model" }

type fakeSink struct {
	saved []listing.Listing
	err   error
}

func (s *fakeSink) Save(_ context.Context, item listing.Listing) error {
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, item)
	return nil
}

const rawRow = `{"source":"zonaprop","url":"https://z.com/a.html","neighborhood":"palermo","agency":"Test SA","address":"Thames 1234","description":"Depto muy luminoso.","price_text":"Alquiler $ 850.000","expenses_text":"Expensas $ 150.000","features":{"ambientes":"2 amb.","dormitorios":"1 dorm.","disposicion":"Frente"},"scraped_at":"2026-09-03T10:00:00Z"}`

func parsedRows(t *testing.T, out *bytes.Buffer) []listing.Listing {
	t.Helper()
	var rows []listing.Listing
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var item listing.Listing
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("output line is not valid JSON: %v: %s", err, line)
		}
		rows = append(rows, item)
	}
	return rows
}

func TestParse_NormalizesAndAttachesAttributes(t *testing.T) {
	parser := &fakeParser{attributes: []listing.Attribute{
		{Type: "natural_light", Value: "high", Provenance: listing.Stated, Evidence: "muy luminoso"},
	}}
	var out bytes.Buffer

	report, err := pipeline.Parse(context.Background(), strings.NewReader(rawRow), &out, parser, nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if report.Parsed != 1 {
		t.Errorf("Parsed = %d, want 1", report.Parsed)
	}

	rows := parsedRows(t, &out)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	row := rows[0]

	if row.Price.Amount == nil || *row.Price.Amount != 850000 {
		t.Errorf("Price.Amount = %v, want 850000", row.Price.Amount)
	}
	if row.Operation != "alquiler" {
		t.Errorf("Operation = %q, want alquiler", row.Operation)
	}
	if row.Agency != "Test SA" {
		t.Errorf("Agency = %q", row.Agency)
	}
	if row.ParserModel != "fake-model" {
		t.Errorf("ParserModel = %q, want fake-model", row.ParserModel)
	}
	if row.ParsedAt == nil {
		t.Error("ParsedAt is nil, want a timestamp")
	}

	// The deterministic exposure and the model's finding both survive.
	if !hasAttribute(row.Attributes, "exposure", "frente") {
		t.Errorf("missing deterministic exposure attribute: %+v", row.Attributes)
	}
	if !hasAttribute(row.Attributes, "natural_light", "high") {
		t.Errorf("missing parsed attribute: %+v", row.Attributes)
	}
}

// A field the page publishes outranks anything the model reads into the prose.
// Storing both would leave the listing claiming two incompatible exposures.
func TestParse_PublishedFieldsOverrideTheModel(t *testing.T) {
	parser := &fakeParser{attributes: []listing.Attribute{
		{Type: "exposure", Value: "contrafrente", Provenance: listing.Inferred},
		{Type: "natural_light", Value: "high", Provenance: listing.Stated},
	}}
	var out bytes.Buffer

	if _, err := pipeline.Parse(context.Background(), strings.NewReader(rawRow), &out, parser, nil); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	row := parsedRows(t, &out)[0]
	if hasAttribute(row.Attributes, "exposure", "contrafrente") {
		t.Errorf("model exposure overrode the published one: %+v", row.Attributes)
	}
	if !hasAttribute(row.Attributes, "exposure", "frente") {
		t.Errorf("published exposure was lost: %+v", row.Attributes)
	}
	if !hasAttribute(row.Attributes, "natural_light", "high") {
		t.Errorf("unrelated parsed attribute was dropped: %+v", row.Attributes)
	}
}

// One unparseable listing must not cost the whole run: the model is called
// three hundred times and any of those calls can fail on its own.
func TestParse_KeepsTheListingWhenTheModelFails(t *testing.T) {
	parser := &fakeParser{err: errors.New("model unavailable")}
	var out bytes.Buffer

	report, err := pipeline.Parse(context.Background(), strings.NewReader(rawRow), &out, parser, nil)
	if err != nil {
		t.Fatalf("Parse returned a fatal error: %v", err)
	}
	if report.Failed != 1 {
		t.Errorf("Failed = %d, want 1", report.Failed)
	}

	rows := parsedRows(t, &out)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want the listing kept", len(rows))
	}
	// The deterministic work survives even though the model call did not.
	if !hasAttribute(rows[0].Attributes, "exposure", "frente") {
		t.Errorf("deterministic attributes were lost: %+v", rows[0].Attributes)
	}
	if rows[0].ParsedAt != nil {
		t.Error("ParsedAt was set despite the parse failing")
	}
}

func TestParse_SkipsRowsThatCannotBeIngested(t *testing.T) {
	input := rawRow + "\n" + `{"url":"","description":"sin url"}` + "\n" + `{"url":"https://z.com/b.html","description":""}`
	parser := &fakeParser{}
	var out bytes.Buffer

	report, err := pipeline.Parse(context.Background(), strings.NewReader(input), &out, parser, nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if report.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", report.Skipped)
	}
	if report.Parsed != 1 {
		t.Errorf("Parsed = %d, want 1", report.Parsed)
	}
	if parser.calls != 1 {
		t.Errorf("model called %d times, want 1", parser.calls)
	}
}

// Parsing 300 listings through a local model is long enough that it will be
// interrupted, so a rerun must pick up where it stopped rather than pay for
// every completion again.
func TestParse_SkipsURLsAlreadyParsed(t *testing.T) {
	parser := &fakeParser{}
	var out bytes.Buffer

	report, err := pipeline.Parse(context.Background(), strings.NewReader(rawRow), &out, parser,
		map[string]bool{"https://z.com/a.html": true})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if report.AlreadyDone != 1 {
		t.Errorf("AlreadyDone = %d, want 1", report.AlreadyDone)
	}
	if parser.calls != 0 {
		t.Errorf("model called %d times for an already-parsed url, want 0", parser.calls)
	}
	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("wrote output for an already-parsed url: %q", out.String())
	}
}

// Seen on the committed snapshot: every row was written after a failed model
// call, and resuming treated all of them as parsed, so none was ever retried.
func TestKeepParsed_RetriesRowsWhoseModelCallFailed(t *testing.T) {
	parsed := `{"url":"https://z.com/a.html","description":"x","parsed_at":"2026-09-25T00:00:00Z"}`
	existing := parsed + "\n" + `{"url":"https://z.com/b.html","description":"y"}` + "\n"

	var kept bytes.Buffer
	urls, err := pipeline.KeepParsed(strings.NewReader(existing), &kept)
	if err != nil {
		t.Fatalf("KeepParsed failed: %v", err)
	}
	if len(urls) != 1 || !urls["https://z.com/a.html"] {
		t.Errorf("skippable urls = %v, want only the parsed one", urls)
	}
	if strings.TrimSpace(kept.String()) != parsed {
		t.Errorf("kept rows = %q, want only the parsed row so the retry is not a duplicate", kept.String())
	}
}

func TestParsedURLs_ReadsBackWhatWasAlreadyWritten(t *testing.T) {
	existing := `{"url":"https://z.com/a.html","description":"x"}` + "\n" + `{"url":"https://z.com/b.html","description":"y"}`

	urls, err := pipeline.ParsedURLs(strings.NewReader(existing))
	if err != nil {
		t.Fatalf("ParsedURLs failed: %v", err)
	}
	if len(urls) != 2 || !urls["https://z.com/a.html"] || !urls["https://z.com/b.html"] {
		t.Errorf("ParsedURLs = %v", urls)
	}
}

func TestLoad_SavesEveryRow(t *testing.T) {
	parsed := `{"source":"zonaprop","url":"https://z.com/a.html","neighborhood":"palermo","description":"Depto.","scraped_at":"2026-09-03T10:00:00Z","attributes":[{"type":"exposure","value":"frente","provenance":"stated"}]}`
	sink := &fakeSink{}

	report, err := pipeline.Load(context.Background(), strings.NewReader(parsed), sink)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if report.Loaded != 1 {
		t.Errorf("Loaded = %d, want 1", report.Loaded)
	}
	if len(sink.saved) != 1 {
		t.Fatalf("saved %d listings, want 1", len(sink.saved))
	}
	if sink.saved[0].URL != "https://z.com/a.html" {
		t.Errorf("URL = %q", sink.saved[0].URL)
	}
	if len(sink.saved[0].Attributes) != 1 {
		t.Errorf("attributes = %+v", sink.saved[0].Attributes)
	}
}

// A malformed line is reported and stepped over. Aborting would leave the
// database holding a partial load with no indication of where it stopped.
// Captured from the local model (experiments/parse-audit, 2026-09-25): an
// amenity counts only when the listing states it in words that name it.
func TestLoad_KeepsOnlyAmenitiesTheListingNamesInItsOwnWords(t *testing.T) {
	parsed := `{"source":"zonaprop","url":"https://z.com/a.html","neighborhood":"palermo","description":"Pileta climatizada en el último piso. Solo con seguro Respaldar.","scraped_at":"2026-09-03T10:00:00Z","attributes":[` +
		`{"type":"amenity","value":"pileta","provenance":"stated","evidence":"Pileta climatizada en el último piso"},` +
		`{"type":"amenity","value":"seguridad","provenance":"stated","evidence":"Solo con seguro Respaldar."},` +
		`{"type":"amenity","value":"seguridad","provenance":"inferred","evidence":"ubicación en corazón de Congreso (zona segura por contexto)"},` +
		`{"type":"exposure","value":"frente","provenance":"inferred","evidence":"luminoso"}]}`
	sink := &fakeSink{}

	if _, err := pipeline.Load(context.Background(), strings.NewReader(parsed), sink); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	var got []string
	for _, a := range sink.saved[0].Attributes {
		got = append(got, a.Type+"="+a.Value)
	}
	if want := []string{"amenity=pileta", "exposure=frente"}; !slices.Equal(got, want) {
		t.Errorf("stored attributes = %v, want %v", got, want)
	}
}

func TestLoad_ReportsBadLinesWithoutAborting(t *testing.T) {
	parsed := `{"url":"https://z.com/a.html","description":"Depto.","scraped_at":"2026-09-03T10:00:00Z"}` +
		"\n" + `not json at all` +
		"\n" + `{"url":"https://z.com/b.html","description":"Otro.","scraped_at":"2026-09-03T10:00:00Z"}`
	sink := &fakeSink{}

	report, err := pipeline.Load(context.Background(), strings.NewReader(parsed), sink)
	if err != nil {
		t.Fatalf("Load returned a fatal error: %v", err)
	}
	if report.Loaded != 2 {
		t.Errorf("Loaded = %d, want 2", report.Loaded)
	}
	if report.Failed != 1 {
		t.Errorf("Failed = %d, want 1", report.Failed)
	}
}

func TestLoad_ReportsStoreFailures(t *testing.T) {
	parsed := `{"url":"https://z.com/a.html","description":"Depto.","scraped_at":"2026-09-03T10:00:00Z"}`
	sink := &fakeSink{err: errors.New("constraint violation")}

	report, err := pipeline.Load(context.Background(), strings.NewReader(parsed), sink)
	if err != nil {
		t.Fatalf("Load returned a fatal error: %v", err)
	}
	if report.Failed != 1 {
		t.Errorf("Failed = %d, want 1", report.Failed)
	}
	if report.Loaded != 0 {
		t.Errorf("Loaded = %d, want 0", report.Loaded)
	}
}

func hasAttribute(attributes []listing.Attribute, attributeType, value string) bool {
	for _, attribute := range attributes {
		if attribute.Type == attributeType && attribute.Value == value {
			return true
		}
	}
	return false
}
