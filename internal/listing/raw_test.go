package listing_test

import (
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

func mustFloat(t *testing.T, got *float64, want float64, field string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = nil, want %v", field, want)
		return
	}
	if *got != want {
		t.Errorf("%s = %v, want %v", field, *got, want)
	}
}

func mustInt(t *testing.T, got *int, want int, field string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = nil, want %v", field, want)
		return
	}
	if *got != want {
		t.Errorf("%s = %v, want %v", field, *got, want)
	}
}

// Argentine notation uses "." for thousands. Reading "1.200" as 1.2 would turn
// a USD 1,200 flat into one priced at a dollar twenty, which no filter catches.
func TestNormalize_ReadsArgentineNumberFormatting(t *testing.T) {
	cases := []struct {
		text     string
		amount   float64
		currency string
	}{
		{"USD 1.200", 1200, "USD"},
		{"U$S 1.200", 1200, "USD"},
		{"$ 850.000", 850000, "ARS"},
		{"$850.000", 850000, "ARS"},
		{"ARS 1.250.000", 1250000, "ARS"},
		{"$ 1.250.500,50", 1250500.50, "ARS"},
		{"USD 950 por mes", 950, "USD"},
	}

	for _, c := range cases {
		got := listing.Normalize(listing.Raw{URL: "u", Description: "d", PriceText: c.text})
		mustFloat(t, got.Price.Amount, c.amount, "Price.Amount for "+c.text)
		if got.Price.Currency != c.currency {
			t.Errorf("Price.Currency for %q = %q, want %q", c.text, got.Price.Currency, c.currency)
		}
	}
}

// "Consultar precio" is a listing declining to publish a figure. Turning that
// into 0 would make it the cheapest property in every search.
func TestNormalize_LeavesUnpublishedPriceNil(t *testing.T) {
	for _, text := range []string{"", "Consultar precio", "A consultar", "Precio a convenir"} {
		got := listing.Normalize(listing.Raw{URL: "u", Description: "d", PriceText: text})
		if got.Price.Amount != nil {
			t.Errorf("Price.Amount for %q = %v, want nil", text, *got.Price.Amount)
		}
	}
}

func TestNormalize_ReadsExpensesSeparatelyFromPrice(t *testing.T) {
	got := listing.Normalize(listing.Raw{
		URL: "u", Description: "d",
		PriceText:    "$ 850.000",
		ExpensesText: "$ 150.000 Expensas",
	})

	mustFloat(t, got.Price.Amount, 850000, "Price.Amount")
	mustFloat(t, got.Expenses.Amount, 150000, "Expenses.Amount")
	if got.Expenses.Currency != "ARS" {
		t.Errorf("Expenses.Currency = %q, want ARS", got.Expenses.Currency)
	}
}

func TestNormalize_ReadsAreasAndCounts(t *testing.T) {
	got := listing.Normalize(listing.Raw{
		URL: "u", Description: "d",
		Features: map[string]string{
			"Superficie total":    "1.052 m²",
			"Superficie cubierta": "980 m²",
			"Ambientes":           "3 amb.",
			"Dormitorios":         "2 dorm.",
			"Baños":               "1 baño",
			"Cocheras":            "1 coch.",
			"Antigüedad":          "50 años",
		},
	})

	mustFloat(t, got.TotalAreaM2, 1052, "TotalAreaM2")
	mustFloat(t, got.CoveredAreaM2, 980, "CoveredAreaM2")
	mustInt(t, got.Rooms, 3, "Rooms")
	mustInt(t, got.Bedrooms, 2, "Bedrooms")
	mustInt(t, got.Bathrooms, 1, "Bathrooms")
	mustInt(t, got.ParkingSpaces, 1, "ParkingSpaces")
	mustInt(t, got.AgeYears, 50, "AgeYears")
}

// ZonaProp's label casing and accents are not stable across page variants, so
// matching must not hinge on reproducing them exactly.
func TestNormalize_MatchesFeatureLabelsRegardlessOfCaseAndAccents(t *testing.T) {
	got := listing.Normalize(listing.Raw{
		URL: "u", Description: "d",
		Features: map[string]string{
			"SUPERFICIE TOTAL": "52 m²",
			"banos":            "2",
			"antiguedad":       "10 años",
		},
	})

	mustFloat(t, got.TotalAreaM2, 52, "TotalAreaM2")
	mustInt(t, got.Bathrooms, 2, "Bathrooms")
	mustInt(t, got.AgeYears, 10, "AgeYears")
}

// "A estrenar" is a real age of zero, not a missing value.
func TestNormalize_ReadsBrandNewAsZeroYears(t *testing.T) {
	got := listing.Normalize(listing.Raw{
		URL: "u", Description: "d",
		Features: map[string]string{"Antigüedad": "A estrenar"},
	})
	mustInt(t, got.AgeYears, 0, "AgeYears")
}

func TestNormalize_LeavesAbsentFeaturesNil(t *testing.T) {
	got := listing.Normalize(listing.Raw{URL: "u", Description: "d", Features: map[string]string{}})

	if got.TotalAreaM2 != nil {
		t.Errorf("TotalAreaM2 = %v, want nil", *got.TotalAreaM2)
	}
	if got.Bedrooms != nil {
		t.Errorf("Bedrooms = %v, want nil", *got.Bedrooms)
	}
	if got.AgeYears != nil {
		t.Errorf("AgeYears = %v, want nil", *got.AgeYears)
	}
}

// The description is what the parser reads, so ZonaProp's trailing UI affordance
// must not reach it as if the seller had written it.
func TestNormalize_StripsTheReadMoreAffordanceFromTheDescription(t *testing.T) {
	got := listing.Normalize(listing.Raw{
		URL:         "u",
		Description: "Depto luminoso al frente. Leer descripción completa",
	})

	if got.Description != "Depto luminoso al frente." {
		t.Errorf("Description = %q, want %q", got.Description, "Depto luminoso al frente.")
	}
}

func TestNormalize_CarriesIdentityFieldsThrough(t *testing.T) {
	scrapedAt := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	got := listing.Normalize(listing.Raw{
		URL:          "https://www.zonaprop.com.ar/propiedades/clasificado/x.html",
		Neighborhood: "palermo",
		Agency:       "Inmobiliaria Test",
		Address:      "Thames 1234",
		Description:  "Depto.",
		Operation:    "alquiler",
		ScrapedAt:    scrapedAt,
	})

	if got.Source != "zonaprop" {
		t.Errorf("Source = %q, want zonaprop", got.Source)
	}
	if got.Agency != "Inmobiliaria Test" {
		t.Errorf("Agency = %q", got.Agency)
	}
	if got.Neighborhood != "palermo" {
		t.Errorf("Neighborhood = %q", got.Neighborhood)
	}
	if got.Address != "Thames 1234" {
		t.Errorf("Address = %q", got.Address)
	}
	if !got.ScrapedAt.Equal(scrapedAt) {
		t.Errorf("ScrapedAt = %v, want %v", got.ScrapedAt, scrapedAt)
	}
}

// The price span reads "Alquiler $ 450.000", so the operation is already on
// the page and never needs to be guessed by a model.
func TestNormalize_DerivesOperationFromThePriceText(t *testing.T) {
	cases := map[string]string{
		"Alquiler $ 450.000":         "alquiler",
		"Venta USD 120.000":          "venta",
		"Alquiler temporal $ 90.000": "alquiler_temporal",
		"$ 450.000":                  "",
	}

	for text, want := range cases {
		got := listing.Normalize(listing.Raw{URL: "u", Description: "d", PriceText: text})
		if got.Operation != want {
			t.Errorf("Operation for %q = %q, want %q", text, got.Operation, want)
		}
	}
}

// An explicit operation from the caller is authoritative over the derived one.
func TestNormalize_PrefersAnExplicitOperation(t *testing.T) {
	got := listing.Normalize(listing.Raw{
		URL: "u", Description: "d",
		Operation: "venta",
		PriceText: "Alquiler $ 450.000",
	})
	if got.Operation != "venta" {
		t.Errorf("Operation = %q, want venta", got.Operation)
	}
}

// ZonaProp publishes Frente/Contrafrente as its own field, so exposure is a
// fact read off the page rather than something inferred from prose. It is
// recorded as "stated" because the listing states it in a structured field.
func TestNormalize_TurnsThePublishedDispositionIntoAStatedAttribute(t *testing.T) {
	cases := map[string]string{
		"Frente":       "frente",
		"Contrafrente": "contrafrente",
		"Interno":      "interno",
		"Lateral":      "lateral",
	}

	for text, want := range cases {
		got := listing.Normalize(listing.Raw{
			URL: "u", Description: "d",
			Features: map[string]string{"disposicion": text},
		})
		if len(got.Attributes) != 1 {
			t.Fatalf("for %q got %d attributes, want 1: %+v", text, len(got.Attributes), got.Attributes)
		}
		want := listing.Attribute{Type: "exposure", Value: want, Provenance: listing.Stated, Evidence: text}
		if got.Attributes[0] != want {
			t.Errorf("attribute for %q = %+v, want %+v", text, got.Attributes[0], want)
		}
	}
}

func TestNormalize_EmitsNoAttributeForAnUnpublishedDisposition(t *testing.T) {
	got := listing.Normalize(listing.Raw{
		URL: "u", Description: "d",
		Features: map[string]string{"disposicion": ""},
	})
	if len(got.Attributes) != 0 {
		t.Errorf("got %d attributes, want 0: %+v", len(got.Attributes), got.Attributes)
	}
}
