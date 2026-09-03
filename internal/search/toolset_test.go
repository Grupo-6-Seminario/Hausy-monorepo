package search_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/tools"
)

// stubRepository records the query it was asked and hands back canned rows.
// The seam under test is the tool boundary -- argument decoding, validation
// and result shaping -- so the repository only has to be observable.
type stubRepository struct {
	lastQuery search.Query
	lastStats search.StatsQuery
	lastURL   string

	results       search.Results
	detail        listing.Listing
	neighborhoods []search.Neighborhood
	stats         search.Stats
	err           error
}

func (s *stubRepository) Search(_ context.Context, query search.Query) (search.Results, error) {
	s.lastQuery = query
	return s.results, s.err
}

func (s *stubRepository) ByURL(_ context.Context, url string) (listing.Listing, error) {
	s.lastURL = url
	return s.detail, s.err
}

func (s *stubRepository) Neighborhoods(context.Context) ([]search.Neighborhood, error) {
	return s.neighborhoods, s.err
}

func (s *stubRepository) PriceStats(_ context.Context, query search.StatsQuery) (search.Stats, error) {
	s.lastStats = query
	return s.stats, s.err
}

func registryFor(t *testing.T, repository search.Repository) *tools.Registry {
	t.Helper()
	registry := tools.NewRegistry()
	for _, tool := range search.Toolset(repository) {
		if err := registry.Register(tool); err != nil {
			t.Fatalf("Register(%s) failed: %v", tool.Name(), err)
		}
	}
	return registry
}

func invoke(t *testing.T, registry *tools.Registry, name, arguments string) map[string]any {
	t.Helper()
	message := registry.Invoke(context.Background(), llm.ToolCall{
		ID: "call_1", Name: name, Arguments: json.RawMessage(arguments),
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(message.Content), &payload); err != nil {
		t.Fatalf("tool %s returned non-JSON content %q: %v", name, message.Content, err)
	}
	return payload
}

func toolError(t *testing.T, payload map[string]any) string {
	t.Helper()
	message, ok := payload["error"].(string)
	if !ok {
		t.Fatalf("expected an error field, got %+v", payload)
	}
	return message
}

func TestToolset_ExposesTheFourReadCapabilities(t *testing.T) {
	registry := registryFor(t, &stubRepository{})

	want := []string{"list_neighborhoods", "search_listings", "get_listing", "neighborhood_price_stats"}
	definitions := registry.Definitions()
	if len(definitions) != len(want) {
		t.Fatalf("expected %d tools, got %d", len(want), len(definitions))
	}
	for i, name := range want {
		if definitions[i].Name != name {
			t.Errorf("expected tool %d to be %q, got %q", i, name, definitions[i].Name)
		}
		if definitions[i].Description == "" {
			t.Errorf("tool %q has no description; it is the only thing telling the model when to call it", name)
		}
		if definitions[i].InputSchema["type"] != "object" {
			t.Errorf("tool %q does not declare an object schema", name)
		}
	}
}

func TestSearchListings_TranslatesModelArgumentsIntoAQuery(t *testing.T) {
	repository := &stubRepository{}
	registry := registryFor(t, repository)

	invoke(t, registry, "search_listings", `{
		"neighborhoods": ["Palermo", "belgrano"],
		"operation": "alquiler",
		"currency": "USD",
		"max_price": 1000,
		"min_bedrooms": 2,
		"required_attributes": [{"type": "transit_access", "value": "subte_d"}],
		"preferred_attributes": [{"type": "natural_light", "value": "high"}],
		"excluded_attributes": [{"type": "exposure", "value": "interno"}],
		"limit": 5
	}`)

	query := repository.lastQuery
	if len(query.Neighborhoods) != 2 || query.Neighborhoods[0] != "palermo" || query.Neighborhoods[1] != "belgrano" {
		t.Errorf("expected normalised neighborhood slugs, got %v", query.Neighborhoods)
	}
	if query.Operation != "alquiler" || query.Currency != "USD" {
		t.Errorf("unexpected operation/currency: %q %q", query.Operation, query.Currency)
	}
	if query.MaxPrice == nil || *query.MaxPrice != 1000 {
		t.Errorf("expected max_price 1000, got %v", query.MaxPrice)
	}
	if query.MinBedrooms == nil || *query.MinBedrooms != 2 {
		t.Errorf("expected min_bedrooms 2, got %v", query.MinBedrooms)
	}
	if len(query.RequiredAttributes) != 1 || query.RequiredAttributes[0].Value != "subte_d" {
		t.Errorf("unexpected required attributes: %+v", query.RequiredAttributes)
	}
	if len(query.PreferredAttributes) != 1 || len(query.ExcludedAttributes) != 1 {
		t.Errorf("expected preferences and exclusions to survive: %+v", query)
	}
	if query.Limit != 5 {
		t.Errorf("expected limit 5, got %d", query.Limit)
	}
}

// The rejection is the model's only chance to correct the call, so it has to
// say what a valid one looks like.
func TestSearchListings_RejectsAPriceBoundWithoutACurrency(t *testing.T) {
	repository := &stubRepository{}
	registry := registryFor(t, repository)

	message := toolError(t, invoke(t, registry, "search_listings", `{"max_price": 1000}`))
	if !strings.Contains(message, "currency") {
		t.Errorf("expected the error to name the missing currency, got %q", message)
	}
	if repository.lastQuery.MaxPrice != nil {
		t.Error("expected the repository never to be reached on an invalid query")
	}
}

func TestSearchListings_RejectsAnInventedAttributeValueAndListsTheRealOnes(t *testing.T) {
	registry := registryFor(t, &stubRepository{})

	message := toolError(t, invoke(t, registry, "search_listings",
		`{"required_attributes": [{"type": "natural_light", "value": "muy_luminoso"}]}`))
	if !strings.Contains(message, "high") {
		t.Errorf("expected the error to list the allowed values, got %q", message)
	}
}

func TestSearchListings_ReportsWhatAPriceFilterHidTheHardWay(t *testing.T) {
	registry := registryFor(t, &stubRepository{results: search.Results{
		TotalMatches:            3,
		ExcludedForMissingPrice: 12,
		Matches:                 []search.Match{{URL: "https://example.test/1", Neighborhood: "palermo"}},
	}})

	payload := invoke(t, registry, "search_listings", `{"currency":"USD","max_price":1000}`)
	if payload["total_matches"] != float64(3) {
		t.Errorf("expected total_matches 3, got %v", payload["total_matches"])
	}
	if payload["excluded_for_missing_price"] != float64(12) {
		t.Errorf("expected the hidden listings to be counted, got %v", payload["excluded_for_missing_price"])
	}
}

func TestSearchListings_SchemaClosesAttributesOverTheVocabulary(t *testing.T) {
	registry := registryFor(t, &stubRepository{})

	var definition llm.ToolDefinition
	for _, candidate := range registry.Definitions() {
		if candidate.Name == "search_listings" {
			definition = candidate
		}
	}

	encoded, err := json.Marshal(definition.InputSchema)
	if err != nil {
		t.Fatalf("schema does not marshal: %v", err)
	}
	document := string(encoded)
	for _, needle := range []string{"natural_light", "contrafrente", "subte_d", "alquiler_temporal", "ARS"} {
		if !strings.Contains(document, needle) {
			t.Errorf("expected the schema to offer %q as a legal value; without it the model invents one", needle)
		}
	}
}

func TestGetListing_ReturnsTheProseAndTheEvidenceBehindEachAttribute(t *testing.T) {
	repository := &stubRepository{detail: listing.Listing{
		URL:          "https://example.test/7",
		Neighborhood: "congreso",
		Description:  "Piso alto, muy luminoso, al contrafrente.",
		ScrapedAt:    time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
		Attributes: []listing.Attribute{
			{Type: "natural_light", Value: "high", Provenance: listing.Inferred, Evidence: "muy luminoso"},
		},
	}}
	registry := registryFor(t, repository)

	payload := invoke(t, registry, "get_listing", `{"url": "https://example.test/7"}`)
	if repository.lastURL != "https://example.test/7" {
		t.Errorf("expected the url to reach the repository, got %q", repository.lastURL)
	}
	if payload["description"] != "Piso alto, muy luminoso, al contrafrente." {
		t.Errorf("expected the full prose, got %v", payload["description"])
	}

	attributes, ok := payload["attributes"].([]any)
	if !ok || len(attributes) != 1 {
		t.Fatalf("expected 1 attribute, got %v", payload["attributes"])
	}
	attribute := attributes[0].(map[string]any)
	if attribute["provenance"] != "inferred" {
		t.Errorf("expected provenance to be surfaced, got %v", attribute["provenance"])
	}
	if attribute["evidence"] != "muy luminoso" {
		t.Errorf("expected the evidence phrase to be quotable, got %v", attribute["evidence"])
	}
}

func TestGetListing_RequiresAURL(t *testing.T) {
	registry := registryFor(t, &stubRepository{})
	if message := toolError(t, invoke(t, registry, "get_listing", `{}`)); !strings.Contains(message, "url") {
		t.Errorf("expected the error to name the missing url, got %q", message)
	}
}

func TestListNeighborhoods_ReturnsTheSlugsThatActuallyExist(t *testing.T) {
	registry := registryFor(t, &stubRepository{neighborhoods: []search.Neighborhood{
		{Slug: "palermo", Listings: 40, ForRent: 30, ForSale: 10},
		{Slug: "congreso", Listings: 12, ForRent: 12},
	}})

	payload := invoke(t, registry, "list_neighborhoods", `{}`)
	neighborhoods, ok := payload["neighborhoods"].([]any)
	if !ok || len(neighborhoods) != 2 {
		t.Fatalf("expected 2 neighborhoods, got %v", payload["neighborhoods"])
	}
	first := neighborhoods[0].(map[string]any)
	if first["neighborhood"] != "palermo" || first["listings"] != float64(40) {
		t.Errorf("unexpected neighborhood row: %+v", first)
	}
}

func TestPriceStats_RequiresASegmentNarrowEnoughToMean_Something(t *testing.T) {
	registry := registryFor(t, &stubRepository{})

	message := toolError(t, invoke(t, registry, "neighborhood_price_stats", `{"neighborhood": "palermo"}`))
	if !strings.Contains(message, "operation") {
		t.Errorf("expected the error to name the missing operation, got %q", message)
	}
}

func TestPriceStats_PassesTheNormalisedSegmentThrough(t *testing.T) {
	repository := &stubRepository{stats: search.Stats{
		Neighborhood: "palermo", Operation: "alquiler", Currency: "ARS", SampleSize: 31,
	}}
	registry := registryFor(t, repository)

	payload := invoke(t, registry, "neighborhood_price_stats",
		`{"neighborhood": "Palermo", "operation": "alquiler", "currency": "ars", "bedrooms": 2}`)

	if repository.lastStats.Neighborhood != "palermo" || repository.lastStats.Currency != "ARS" {
		t.Errorf("expected a normalised segment, got %+v", repository.lastStats)
	}
	if repository.lastStats.Bedrooms == nil || *repository.lastStats.Bedrooms != 2 {
		t.Errorf("expected bedrooms 2, got %v", repository.lastStats.Bedrooms)
	}
	if payload["sample_size"] != float64(31) {
		t.Errorf("expected the sample size to be surfaced, got %v", payload["sample_size"])
	}
}
