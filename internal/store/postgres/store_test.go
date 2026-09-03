package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
)

// openTestStore connects to a disposable test database and applies the
// migrations. It skips rather than fails when HAUSY_TEST_DATABASE_URI is unset
// or points to a non-disposable database, so `go test ./...` stays green on a
// clone without wiping development data.
func openTestStore(t *testing.T) *postgres.Store {
	t.Helper()

	uri := os.Getenv("HAUSY_TEST_DATABASE_URI")
	if uri == "" {
		t.Skip("skipping: HAUSY_TEST_DATABASE_URI not set; set it to a disposable database (e.g. postgresql://hausy:hausy@localhost:5432/hausy_test) to run Postgres-backed tests")
	}

	if err := postgres.CheckDisposableURI(uri); err != nil {
		t.Skipf("skipping: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store, err := postgres.Open(ctx, uri)
	if err != nil {
		t.Skipf("skipping: Postgres not reachable at %s (%v)", uri, err)
	}
	t.Cleanup(store.Close)

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := store.DeleteAll(ctx); err != nil {
		t.Fatalf("DeleteAll failed: %v", err)
	}
	return store
}

func TestCheckDisposableURI_RefusesNonDisposableDatabases(t *testing.T) {
	cases := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name:    "empty URI",
			uri:     "",
			wantErr: true,
		},
		{
			name:    "development database",
			uri:     "postgresql://hausy:hausy@localhost:5432/hausy",
			wantErr: true,
		},
		{
			name:    "production database",
			uri:     "postgresql://prod-user:secret@prod-host:5432/production",
			wantErr: true,
		},
		{
			name:    "dev suffix database",
			uri:     "postgresql://hausy:hausy@localhost:5432/hausy_dev",
			wantErr: true,
		},
		{
			name:    "valid test database with _test suffix",
			uri:     "postgresql://hausy:hausy@localhost:5432/hausy_test",
			wantErr: false,
		},
		{
			name:    "valid test database named test",
			uri:     "postgresql://hausy:hausy@localhost:5432/test",
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := postgres.CheckDisposableURI(tc.uri)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for URI %q, got nil", tc.uri)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for URI %q: %v", tc.uri, err)
			}
		})
	}
}

func TestOpenTestStore_SkipsWhenNonDisposable(t *testing.T) {
	t.Run("skips when unset", func(subT *testing.T) {
		subT.Setenv("HAUSY_TEST_DATABASE_URI", "")
		openTestStore(subT)
		subT.Fatal("expected openTestStore to skip when HAUSY_TEST_DATABASE_URI is unset")
	})

	t.Run("skips when pointed at dev database", func(subT *testing.T) {
		subT.Setenv("HAUSY_TEST_DATABASE_URI", "postgresql://hausy:hausy@localhost:5432/hausy")
		openTestStore(subT)
		subT.Fatal("expected openTestStore to skip when pointed at dev database")
	})
}


func float64Ptr(v float64) *float64 { return &v }
func intPtr(v int) *int             { return &v }

func sampleListing() listing.Listing {
	return listing.Listing{
		Source:       "zonaprop",
		URL:          "https://www.zonaprop.com.ar/propiedades/clasificado/test-1.html",
		Neighborhood: "palermo",
		Agency:       "Inmobiliaria Test",
		Address:      "Thames 1234",
		Description:  "Depto muy luminoso al contrafrente.",
		Operation:    "alquiler",
		Price:        listing.Money{Amount: float64Ptr(850000), Currency: "ARS"},
		Expenses:     listing.Money{Amount: float64Ptr(150000), Currency: "ARS"},
		TotalAreaM2:  float64Ptr(52),
		Rooms:        intPtr(2),
		Bedrooms:     intPtr(1),
		Bathrooms:    intPtr(1),
		Floor:        "7",
		ScrapedAt:    time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC),
		ParserModel:  "test-model",
		Attributes: []listing.Attribute{
			{Type: "natural_light", Value: "high", Provenance: listing.Stated, Evidence: "muy luminoso"},
			{Type: "exposure", Value: "contrafrente", Provenance: listing.Stated, Evidence: "al contrafrente"},
		},
	}
}

func TestSave_RoundTripsAListing(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	want := sampleListing()
	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := store.ByURL(ctx, want.URL)
	if err != nil {
		t.Fatalf("ByURL failed: %v", err)
	}

	if got.Neighborhood != want.Neighborhood {
		t.Errorf("Neighborhood = %q, want %q", got.Neighborhood, want.Neighborhood)
	}
	if got.Agency != want.Agency {
		t.Errorf("Agency = %q, want %q", got.Agency, want.Agency)
	}
	if got.Description != want.Description {
		t.Errorf("Description = %q, want %q", got.Description, want.Description)
	}
	if got.Price.Amount == nil || *got.Price.Amount != 850000 {
		t.Errorf("Price.Amount = %v, want 850000", got.Price.Amount)
	}
	if got.Price.Currency != "ARS" {
		t.Errorf("Price.Currency = %q, want ARS", got.Price.Currency)
	}
	if got.Bedrooms == nil || *got.Bedrooms != 1 {
		t.Errorf("Bedrooms = %v, want 1", got.Bedrooms)
	}
	// Attributes come back ordered by (type, value), not in insertion order.
	if len(got.Attributes) != 2 {
		t.Fatalf("got %d attributes, want 2: %+v", len(got.Attributes), got.Attributes)
	}
	wantAttributes := []listing.Attribute{
		{Type: "exposure", Value: "contrafrente", Provenance: listing.Stated, Evidence: "al contrafrente"},
		{Type: "natural_light", Value: "high", Provenance: listing.Stated, Evidence: "muy luminoso"},
	}
	for i := range wantAttributes {
		if got.Attributes[i] != wantAttributes[i] {
			t.Errorf("attribute %d = %+v, want %+v", i, got.Attributes[i], wantAttributes[i])
		}
	}
}

// A missing figure must survive the round trip as missing. Reading it back as
// zero would tell a buyer agent the expenses are nothing, which is a different
// and much more attractive claim than "not published".
func TestSave_KeepsUnpublishedFiguresNull(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	sparse := sampleListing()
	sparse.Expenses = listing.Money{}
	sparse.Bedrooms = nil
	sparse.TotalAreaM2 = nil

	if err := store.Save(ctx, sparse); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := store.ByURL(ctx, sparse.URL)
	if err != nil {
		t.Fatalf("ByURL failed: %v", err)
	}
	if got.Expenses.Amount != nil {
		t.Errorf("Expenses.Amount = %v, want nil", *got.Expenses.Amount)
	}
	if got.Bedrooms != nil {
		t.Errorf("Bedrooms = %v, want nil", *got.Bedrooms)
	}
	if got.TotalAreaM2 != nil {
		t.Errorf("TotalAreaM2 = %v, want nil", *got.TotalAreaM2)
	}
}

// Ingest is meant to be re-runnable so every teammate loading the same seed
// file ends up with the same database, so saving twice must not duplicate.
func TestSave_IsIdempotentOnURL(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	first := sampleListing()
	if err := store.Save(ctx, first); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}
	if err := store.Save(ctx, first); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

// A re-parse replaces the previous attributes rather than accumulating them,
// so a listing never carries two contradictory readings of the same quality.
func TestSave_ReplacesAttributesOnReparse(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	original := sampleListing()
	if err := store.Save(ctx, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reparsed := sampleListing()
	reparsed.Attributes = []listing.Attribute{
		{Type: "natural_light", Value: "low", Provenance: listing.Inferred, Evidence: "contrafrente"},
	}
	if err := store.Save(ctx, reparsed); err != nil {
		t.Fatalf("re-Save failed: %v", err)
	}

	got, err := store.ByURL(ctx, original.URL)
	if err != nil {
		t.Fatalf("ByURL failed: %v", err)
	}
	if len(got.Attributes) != 1 {
		t.Fatalf("got %d attributes, want 1: %+v", len(got.Attributes), got.Attributes)
	}
	if got.Attributes[0].Value != "low" {
		t.Errorf("Value = %q, want %q", got.Attributes[0].Value, "low")
	}
}

// Two listings from the same agency must resolve to one agency row, so the
// name can be counted on as an identity rather than repeated free text.
func TestSave_ReusesTheAgencyRow(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	first := sampleListing()
	second := sampleListing()
	second.URL = "https://www.zonaprop.com.ar/propiedades/clasificado/test-2.html"

	if err := store.Save(ctx, first); err != nil {
		t.Fatalf("Save first failed: %v", err)
	}
	if err := store.Save(ctx, second); err != nil {
		t.Fatalf("Save second failed: %v", err)
	}

	agencies, err := store.CountAgencies(ctx)
	if err != nil {
		t.Fatalf("CountAgencies failed: %v", err)
	}
	if agencies != 1 {
		t.Errorf("CountAgencies = %d, want 1", agencies)
	}
}

func TestSave_AcceptsAListingWithNoAgency(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	anonymous := sampleListing()
	anonymous.Agency = ""

	if err := store.Save(ctx, anonymous); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := store.ByURL(ctx, anonymous.URL)
	if err != nil {
		t.Fatalf("ByURL failed: %v", err)
	}
	if got.Agency != "" {
		t.Errorf("Agency = %q, want empty", got.Agency)
	}
}

// Migrate runs on every start, so applying it to an already-migrated database
// must be a no-op rather than an error.
func TestMigrate_IsRepeatable(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate failed: %v", err)
	}
}
