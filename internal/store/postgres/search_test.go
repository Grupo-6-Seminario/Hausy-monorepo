package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
)

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

func TestSearch_ReportsTheTotalBeyondTheReturnedPage(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	results := mustSearch(t, store, search.Query{Neighborhoods: []string{"palermo"}, Limit: 2})
	if results.TotalMatches != 7 {
		t.Errorf("expected 7 listings in palermo, got %d", results.TotalMatches)
	}
	if len(results.Matches) != 2 {
		t.Errorf("expected the page to honour the limit, got %d rows", len(results.Matches))
	}
}

// An empty attribute list on an unparsed listing means "never read", not
// "does not have any", and the difference has to reach the agent.
func TestSearch_FlagsListingsWhoseProseWasNeverParsed(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	results := mustSearch(t, store, search.Query{Neighborhoods: []string{"congreso"}})
	if len(results.Matches) != 1 {
		t.Fatalf("expected 1 listing in congreso, got %d", len(results.Matches))
	}
	if !results.Matches[0].Unparsed {
		t.Error("expected the unparsed listing to say so")
	}
}

func TestSearch_CarriesAttributesWithTheirProvenance(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	results := mustSearch(t, store, search.Query{
		Neighborhoods:      []string{"palermo"},
		RequiredAttributes: []search.AttributeFilter{{Type: "exposure", Value: "contrafrente"}},
	})
	if len(results.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results.Matches))
	}

	found := false
	for _, attribute := range results.Matches[0].Attributes {
		if attribute.Type == "noise_level" {
			found = true
			if attribute.Provenance != listing.Inferred {
				t.Errorf("expected the noise reading to be marked inferred, got %q", attribute.Provenance)
			}
		}
	}
	if !found {
		t.Error("expected the listing's attributes to come back with the row")
	}
}

func TestNeighborhoods_ReportsWhatIsActuallyStored(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	neighborhoods, err := store.Neighborhoods(context.Background())
	if err != nil {
		t.Fatalf("Neighborhoods failed: %v", err)
	}
	if len(neighborhoods) != 2 {
		t.Fatalf("expected palermo and congreso, got %+v", neighborhoods)
	}
	if neighborhoods[0].Slug != "palermo" || neighborhoods[0].Listings != 7 {
		t.Errorf("expected palermo with 7 listings first, got %+v", neighborhoods[0])
	}
	if neighborhoods[0].MinPriceUSD == nil || *neighborhoods[0].MinPriceUSD != 500 {
		t.Errorf("expected the cheapest USD listing at 500, got %v", neighborhoods[0].MinPriceUSD)
	}
}

// The five USD rentals in Palermo are 500, 700, 800, 900 and 1500: median 800,
// min 500, max 1500. Worked out from the fixture, not from the query.
func TestPriceStats_DescribesTheSegmentItWasAskedFor(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	stats, err := store.PriceStats(context.Background(), search.StatsQuery{
		Neighborhood: "palermo", Operation: "alquiler", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("PriceStats failed: %v", err)
	}
	if stats.SampleSize != 5 {
		t.Fatalf("expected a sample of 5, got %d", stats.SampleSize)
	}
	if stats.Median == nil || *stats.Median != 800 {
		t.Errorf("expected a median of 800, got %v", stats.Median)
	}
	if stats.Min == nil || *stats.Min != 500 || stats.Max == nil || *stats.Max != 1500 {
		t.Errorf("expected the range 500-1500, got %v-%v", stats.Min, stats.Max)
	}
	if len(stats.Notes) == 0 {
		t.Error("expected a caveat about a sample this small")
	}
}

func TestPriceStats_NarrowsToBedroomsAndStaysInOneCurrency(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	stats, err := store.PriceStats(context.Background(), search.StatsQuery{
		Neighborhood: "palermo", Operation: "alquiler", Currency: "USD", Bedrooms: intPtr(2),
	})
	if err != nil {
		t.Fatalf("PriceStats failed: %v", err)
	}
	// 700, 800 and 900 -- listing 5 is in pesos and listing 4 has three bedrooms.
	if stats.SampleSize != 3 {
		t.Fatalf("expected a sample of 3, got %d", stats.SampleSize)
	}
	if stats.Median == nil || *stats.Median != 800 {
		t.Errorf("expected a median of 800, got %v", stats.Median)
	}
}

func TestPriceStats_SaysSoWhenTheSegmentIsEmpty(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	stats, err := store.PriceStats(context.Background(), search.StatsQuery{
		Neighborhood: "congreso", Operation: "venta", Currency: "USD",
	})
	if err != nil {
		t.Fatalf("PriceStats failed: %v", err)
	}
	if stats.SampleSize != 0 || stats.Median != nil {
		t.Errorf("expected an empty segment to report nothing, got %+v", stats)
	}
	if len(stats.Notes) == 0 {
		t.Error("expected an empty segment to say it is empty")
	}
}

// The store is the production Repository; if it drifts from the port the tools
// sit on, nothing else compiles.
func TestStore_SatisfiesTheSearchRepositoryPort(t *testing.T) {
	var _ search.Repository = (*postgres.Store)(nil)
}

func TestSearch_AssignsExplicitSequentialRankToMatches(t *testing.T) {
	store := openTestStore(t)
	seedSearchFixture(t, store)

	results := mustSearch(t, store, search.Query{
		Neighborhoods: []string{"palermo"},
		Limit:         5,
	})

	if len(results.Matches) == 0 {
		t.Fatal("expected matches, got 0")
	}

	for i, match := range results.Matches {
		wantRank := i + 1
		if match.Rank != wantRank {
			t.Errorf("match %d: got Rank %d, want %d", i, match.Rank, wantRank)
		}
	}
}
