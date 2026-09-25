package postgres_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/pipeline"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
)

func TestQualitySnapshotCannotApproveChangedListingText(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	item := listing.Listing{URL: "snapshot-change", Neighborhood: "palermo", Description: "Muy luminoso.", Operation: "alquiler", ScrapedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	if err := store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReview(ctx, item.URL, quality.Review{Status: quality.Passed}); err != nil {
		t.Fatal(err)
	}
	var snapshot bytes.Buffer
	if _, err := store.ExportReviews(ctx, &snapshot, map[string]bool{item.URL: true}); err != nil {
		t.Fatal(err)
	}
	item.Description = "Recibe poca luz natural."
	if err := store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	report, err := pipeline.LoadQuality(ctx, &snapshot, store)
	if err != nil {
		t.Fatal(err)
	}
	if report.Loaded != 0 || report.Failed != 1 || mustSearch(t, store, search.Query{Neighborhoods: []string{"palermo"}}).TotalMatches != 0 {
		t.Fatalf("stale review republished changed text: %+v", report)
	}
}

func TestSearchPublishesOnlyReviewedListings(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	item := listing.Listing{URL: "quality-test", Neighborhood: "palermo", Description: "Luminoso.", Operation: "alquiler", ScrapedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	if err := store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	q := search.Query{Neighborhoods: []string{"palermo"}}
	if got := mustSearch(t, store, q).TotalMatches; got != 0 {
		t.Fatalf("pending listing leaked: %d", got)
	}
	if err := store.SaveReview(ctx, item.URL, quality.Review{Status: quality.Passed}); err != nil {
		t.Fatal(err)
	}
	if got := mustSearch(t, store, q).TotalMatches; got != 1 {
		t.Fatalf("reviewed listing missing: %d", got)
	}
	if err := store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	if got := mustSearch(t, store, q).TotalMatches; got != 1 {
		t.Fatalf("idempotent load lost its review: %d", got)
	}
	area := 45.0
	item.TotalAreaM2 = &area
	if err := store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	if got := mustSearch(t, store, q).TotalMatches; got != 0 {
		t.Fatalf("changed published area must wait for review: %d", got)
	}
	if err := store.SaveReview(ctx, item.URL, quality.Review{Status: quality.Passed}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReview(ctx, item.URL, quality.Review{Status: quality.Withheld, Conflicts: []quality.Conflict{{First: "a", Second: "b", Detail: "light"}}}); err != nil {
		t.Fatal(err)
	}
	if got := mustSearch(t, store, q).TotalMatches; got != 0 {
		t.Fatalf("withheld listing leaked: %d", got)
	}
	if candidates, err := store.Candidates(ctx, q); err != nil || len(candidates) != 0 {
		t.Fatalf("withheld listing leaked into buyer candidates: %+v, %v", candidates, err)
	}
	item.Description = "Changed description."
	if err := store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	if got := mustSearch(t, store, q).TotalMatches; got != 0 {
		t.Fatalf("changed listing must wait for re-review: %d", got)
	}
}

// The fixture is a worked example: eight listings whose expected answer to each
// query below was decided by reading them, not by re-running the SQL.
func seedSearchFixture(t *testing.T, store *postgres.Store) {
	t.Helper()
	ctx := context.Background()

	base := func(url, neighborhood string) listing.Listing {
		return listing.Listing{
			Source: "zonaprop", URL: url, Neighborhood: neighborhood,
			Description: "Departamento.", Operation: "alquiler",
			ScrapedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		}
	}
	parsed := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	items := []listing.Listing{
		func() listing.Listing { // 1: bright, quiet, on the D, 2 bed, USD 900
			l := base("https://ex.test/1", "palermo")
			l.Price = listing.Money{Amount: float64Ptr(900), Currency: "USD"}
			l.Expenses = listing.Money{Amount: float64Ptr(120000), Currency: "ARS"}
			l.Rooms, l.Bedrooms, l.TotalAreaM2 = intPtr(3), intPtr(2), float64Ptr(70)
			l.ParsedAt, l.ParserModel = &parsed, "qwen"
			l.Attributes = []listing.Attribute{
				{Type: "natural_light", Value: "high", Provenance: listing.Stated, Evidence: "muy luminoso"},
				{Type: "noise_level", Value: "quiet", Provenance: listing.Inferred, Evidence: "contrafrente"},
				{Type: "transit_access", Value: "subte_d", Provenance: listing.Stated},
				{Type: "exposure", Value: "contrafrente", Provenance: listing.Stated},
			}
			return l
		}(),
		func() listing.Listing { // 2: dark, on the D, 2 bed, USD 700
			l := base("https://ex.test/2", "palermo")
			l.Price = listing.Money{Amount: float64Ptr(700), Currency: "USD"}
			l.Rooms, l.Bedrooms, l.TotalAreaM2 = intPtr(3), intPtr(2), float64Ptr(50)
			l.ParsedAt, l.ParserModel = &parsed, "qwen"
			l.Attributes = []listing.Attribute{
				{Type: "natural_light", Value: "low", Provenance: listing.Inferred},
				{Type: "transit_access", Value: "subte_d", Provenance: listing.Stated},
			}
			return l
		}(),
		func() listing.Listing { // 3: bright but no subte, 2 bed, USD 800
			l := base("https://ex.test/3", "palermo")
			l.Price = listing.Money{Amount: float64Ptr(800), Currency: "USD"}
			l.Rooms, l.Bedrooms, l.TotalAreaM2 = intPtr(3), intPtr(2), float64Ptr(60)
			l.ParsedAt, l.ParserModel = &parsed, "qwen"
			l.Attributes = []listing.Attribute{
				{Type: "natural_light", Value: "high", Provenance: listing.Inferred},
			}
			return l
		}(),
		func() listing.Listing { // 4: over budget at USD 1500
			l := base("https://ex.test/4", "palermo")
			l.Price = listing.Money{Amount: float64Ptr(1500), Currency: "USD"}
			l.Rooms, l.Bedrooms, l.TotalAreaM2 = intPtr(4), intPtr(3), float64Ptr(110)
			return l
		}(),
		func() listing.Listing { // 5: ARS 900 -- must never satisfy a USD 1000 ceiling
			l := base("https://ex.test/5", "palermo")
			l.Price = listing.Money{Amount: float64Ptr(900), Currency: "ARS"}
			l.Rooms, l.Bedrooms = intPtr(3), intPtr(2)
			return l
		}(),
		func() listing.Listing { // 6: no price published at all
			l := base("https://ex.test/6", "palermo")
			l.Rooms, l.Bedrooms = intPtr(3), intPtr(2)
			return l
		}(),
		func() listing.Listing { // 7: cheap but only 1 bedroom
			l := base("https://ex.test/7", "palermo")
			l.Price = listing.Money{Amount: float64Ptr(500), Currency: "USD"}
			l.Rooms, l.Bedrooms = intPtr(2), intPtr(1)
			return l
		}(),
		func() listing.Listing { // 8: right shape, wrong neighborhood
			l := base("https://ex.test/8", "congreso")
			l.Price = listing.Money{Amount: float64Ptr(600), Currency: "USD"}
			l.Rooms, l.Bedrooms, l.TotalAreaM2 = intPtr(3), intPtr(2), float64Ptr(55)
			return l
		}(),
	}

	for _, item := range items {
		if err := store.Save(ctx, item); err != nil {
			t.Fatalf("seeding %s failed: %v", item.URL, err)
		}
		if err := store.SaveReview(ctx, item.URL, quality.Review{Status: quality.Passed}); err != nil {
			t.Fatal(err)
		}
	}
}

func mustSearch(t *testing.T, store *postgres.Store, query search.Query) search.Results {
	t.Helper()
	validated, err := query.Validate()
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	results, err := store.Search(context.Background(), validated)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	return results
}

func urlsOf(matches []search.Match) []string {
	urls := make([]string, 0, len(matches))
	for _, match := range matches {
		urls = append(urls, match.URL)
	}
	return urls
}

func assertURLs(t *testing.T, got []search.Match, want ...string) {
	t.Helper()
	urls := urlsOf(got)
	if len(urls) != len(want) {
		t.Fatalf("expected %v, got %v", want, urls)
	}
	for i := range want {
		if urls[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, urls)
		}
	}
}

// Trap 2: listing 5 is priced at 900, but pesos. It must not answer a
// question about dollars.
func TestSearch_APriceCeilingNeverCrossesCurrencies(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	results := mustSearch(t, store, search.Query{
		Neighborhoods: []string{"palermo"}, Operation: "alquiler",
		Currency: "USD", MaxPrice: float64Ptr(1000), MinBedrooms: intPtr(2),
	})

	// 1, 2 and 3 qualify. 4 is over budget, 5 is in pesos, 6 has no price,
	// 7 has one bedroom, 8 is in Congreso.
	if results.TotalMatches != 3 {
		t.Errorf("expected 3 matches, got %d (%v)", results.TotalMatches, urlsOf(results.Matches))
	}
	for _, match := range results.Matches {
		if match.URL == "https://ex.test/5" {
			t.Error("an ARS price satisfied a USD ceiling")
		}
	}
}

// Trap 1: listing 6 published no price. It is excluded, and the exclusion is
// counted rather than swallowed.
func TestSearch_CountsTheListingsAPriceFilterHides(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	results := mustSearch(t, store, search.Query{
		Neighborhoods: []string{"palermo"}, Currency: "USD", MaxPrice: float64Ptr(1000), MinBedrooms: intPtr(2),
	})
	if results.ExcludedForMissingPrice != 1 {
		t.Errorf("expected 1 listing hidden for having no published price, got %d", results.ExcludedForMissingPrice)
	}

	withUnpriced := mustSearch(t, store, search.Query{
		Neighborhoods: []string{"palermo"}, Currency: "USD", MaxPrice: float64Ptr(1000),
		MinBedrooms: intPtr(2), IncludeUnpriced: true,
	})
	if withUnpriced.TotalMatches != 4 {
		t.Errorf("expected the unpriced listing back, got %d matches", withUnpriced.TotalMatches)
	}
}

func TestSearch_RequiredAttributesMustAllBePresent(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	// Only listing 1 is both bright and on the D.
	results := mustSearch(t, store, search.Query{
		Neighborhoods: []string{"palermo"},
		RequiredAttributes: []search.AttributeFilter{
			{Type: "natural_light", Value: "high"},
			{Type: "transit_access", Value: "subte_d"},
		},
	})
	assertURLs(t, results.Matches, "https://ex.test/1")
}

func TestSearch_ExcludedAttributesDisqualify(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	results := mustSearch(t, store, search.Query{
		Neighborhoods:      []string{"palermo"},
		RequiredAttributes: []search.AttributeFilter{{Type: "transit_access", Value: "subte_d"}},
		ExcludedAttributes: []search.AttributeFilter{{Type: "natural_light", Value: "low"}},
	})
	assertURLs(t, results.Matches, "https://ex.test/1")
}

// Preferences order the results; they never cut them. Listing 2 misses both
// and still comes back, last.
func TestSearch_PreferredAttributesRankWithoutFiltering(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	results := mustSearch(t, store, search.Query{
		Neighborhoods: []string{"palermo"}, Currency: "USD", MaxPrice: float64Ptr(1000), MinBedrooms: intPtr(2),
		PreferredAttributes: []search.AttributeFilter{
			{Type: "natural_light", Value: "high"},
			{Type: "noise_level", Value: "quiet"},
		},
	})

	// 1 has both (light stated, so it outweighs 3's inferred light); 3 has one;
	// 2 has neither.
	assertURLs(t, results.Matches, "https://ex.test/1", "https://ex.test/3", "https://ex.test/2")

	if len(results.Matches[0].MatchedPreferences) != 2 {
		t.Errorf("expected both preferences matched on listing 1, got %+v", results.Matches[0].MatchedPreferences)
	}
	if len(results.Matches[2].MissedPreferences) != 2 {
		t.Errorf("expected both preferences missed on listing 2, got %+v", results.Matches[2].MissedPreferences)
	}
}
