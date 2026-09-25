package search_test

import (
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

func floatPtr(v float64) *float64 { return &v }
func intPtr(v int) *int           { return &v }

// Trap 2 from docs/DATA_MODEL.md: there is no stored exchange rate, so a bound
// of 1000 is meaningless until it says which 1000.
func TestQuery_Validate_RejectsAPriceBoundWithoutACurrency(t *testing.T) {
	_, err := search.Query{Neighborhoods: []string{"palermo"}, MaxPrice: floatPtr(1000)}.Validate()
	if err == nil {
		t.Fatal("expected a price bound without a currency to be rejected")
	}
	if !strings.Contains(err.Error(), "currency") {
		t.Errorf("expected the error to name the missing currency, got: %v", err)
	}
}

func TestQuery_Validate_RejectsACurrencyThatIsNotStored(t *testing.T) {
	_, err := search.Query{Currency: "EUR", MaxPrice: floatPtr(1000)}.Validate()
	if err == nil {
		t.Fatal("expected an unsupported currency to be rejected")
	}
	if !strings.Contains(err.Error(), "ARS") || !strings.Contains(err.Error(), "USD") {
		t.Errorf("expected the error to list the stored currencies, got: %v", err)
	}
}

func TestQuery_Validate_RejectsAnInvertedPriceRange(t *testing.T) {
	_, err := search.Query{Currency: "USD", MinPrice: floatPtr(1500), MaxPrice: floatPtr(900)}.Validate()
	if err == nil {
		t.Fatal("expected min_price above max_price to be rejected")
	}
}

// The vocabulary is closed. A value the model invents matches nothing, so the
// error has to hand back the values that do exist.
func TestQuery_Validate_RejectsAttributesOutsideTheVocabulary(t *testing.T) {
	cases := map[string]search.AttributeFilter{
		"unknown type":  {Type: "vista_al_rio", Value: "yes"},
		"unknown value": {Type: "natural_light", Value: "luminoso"},
	}

	for name, filter := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := search.Query{RequiredAttributes: []search.AttributeFilter{filter}}.Validate()
			if err == nil {
				t.Fatalf("expected %+v to be rejected", filter)
			}
			if !strings.Contains(err.Error(), filter.Type) {
				t.Errorf("expected the error to quote the offending type, got: %v", err)
			}
		})
	}

	_, err := search.Query{RequiredAttributes: []search.AttributeFilter{
		{Type: "natural_light", Value: "luminoso"},
	}}.Validate()
	if !strings.Contains(err.Error(), "high") {
		t.Errorf("expected the error to list the allowed values for natural_light, got: %v", err)
	}
}

func TestQuery_Validate_AcceptsAVocabularyAttribute(t *testing.T) {
	validated, err := search.Query{RequiredAttributes: []search.AttributeFilter{
		{Type: "transit_access", Value: "subte_d"},
	}}.Validate()
	if err != nil {
		t.Fatalf("expected a vocabulary attribute to be accepted, got: %v", err)
	}
	if len(validated.RequiredAttributes) != 1 {
		t.Errorf("expected the filter to survive validation, got %+v", validated.RequiredAttributes)
	}
}

func TestQuery_Validate_RejectsAnUnknownOperation(t *testing.T) {
	_, err := search.Query{Operation: "leasing"}.Validate()
	if err == nil {
		t.Fatal("expected an unknown operation to be rejected")
	}
	if !strings.Contains(err.Error(), "alquiler") {
		t.Errorf("expected the error to list the stored operations, got: %v", err)
	}
}

func TestQuery_Validate_NormalisesNeighborhoodsToStoredSlugs(t *testing.T) {
	validated, err := search.Query{Neighborhoods: []string{" Palermo ", "PUERTO MADERO", "villa  crespo"}}.Validate()
	if err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}

	want := []string{"palermo", "puerto_madero", "villa_crespo"}
	if len(validated.Neighborhoods) != len(want) {
		t.Fatalf("expected %v, got %v", want, validated.Neighborhoods)
	}
	for i, slug := range want {
		if validated.Neighborhoods[i] != slug {
			t.Errorf("expected neighborhood %d to be %q, got %q", i, slug, validated.Neighborhoods[i])
		}
	}
}

func TestQuery_Validate_AppliesAndCapsTheResultLimit(t *testing.T) {
	cases := map[string]struct {
		limit int
		want  int
	}{
		"unset falls back to the default": {limit: 0, want: search.DefaultLimit},
		"negative falls back too":         {limit: -3, want: search.DefaultLimit},
		"within bounds is kept":           {limit: 7, want: 7},
		"beyond the cap is clamped":       {limit: 500, want: search.MaxLimit},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			validated, err := search.Query{Limit: testCase.limit}.Validate()
			if err != nil {
				t.Fatalf("Validate returned unexpected error: %v", err)
			}
			if validated.Limit != testCase.want {
				t.Errorf("expected limit %d, got %d", testCase.want, validated.Limit)
			}
		})
	}
}

func TestQuery_Validate_RejectsNegativeBounds(t *testing.T) {
	cases := map[string]search.Query{
		"negative price":    {Currency: "USD", MaxPrice: floatPtr(-1)},
		"negative expenses": {MaxExpensesARS: floatPtr(-1)},
		"negative rooms":    {MinRooms: intPtr(-2)},
		"negative area":     {MinTotalAreaM2: floatPtr(-10)},
	}

	for name, query := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := query.Validate(); err == nil {
				t.Error("expected a negative bound to be rejected")
			}
		})
	}
}
