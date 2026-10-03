package postgres_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

func TestCandidatesReturnEveryHardFilterMatchWithItsRules(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	save := func(url, hood string, price float64) {
		t.Helper()
		if err := store.Save(ctx, listing.Listing{Source: "zonaprop", URL: url, Neighborhood: hood, Description: "Departamento.", Operation: "alquiler",
			Price: listing.Money{Amount: float64Ptr(price), Currency: "ARS"}, ScrapedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveReview(ctx, url, quality.Review{Status: quality.Passed}); err != nil {
			t.Fatal(err)
		}
	}
	save("https://ex.test/a", "palermo", 800000)
	save("https://ex.test/b", "palermo", 1200000)
	save("https://ex.test/c", "caballito", 700000)
	rule := eligibility.Rule{Fact: "guarantee", Operator: "one_of", Values: []string{"propietaria", "caucion"}, Hardness: "discretionary", Visibility: "public", Source: "parsed", Evidence: "Garantía propietaria o caución (ver cuáles)"}
	if err := store.SaveEligibility(ctx, "https://ex.test/a", []eligibility.Rule{rule}); err != nil {
		t.Fatal(err)
	}
	// Saving again replaces rather than duplicates.
	if err := store.SaveEligibility(ctx, "https://ex.test/a", []eligibility.Rule{rule}); err != nil {
		t.Fatal(err)
	}

	got, err := store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo"}, Currency: "ARS", MaxPrice: float64Ptr(1000000)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Listing.URL != "https://ex.test/a" || len(got[0].Rules) != 1 || !slices.Equal(got[0].Rules[0].Values, rule.Values) || got[0].Rules[0].Evidence != rule.Evidence {
		t.Fatalf("got %+v", got)
	}
	all, err := store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo"}})
	if err != nil || len(all) != 2 {
		t.Fatalf("want both Palermo listings uncapped, got %d, %v", len(all), err)
	}
	preview, err := store.PreviewCandidates(ctx, search.Query{Neighborhoods: []string{"palermo"}}, 1)
	if err != nil || len(preview) != 1 || preview[0].Listing.URL != "https://ex.test/a" {
		t.Fatalf("the impact preview must obey its sample limit: %+v, %v", preview, err)
	}
	hasAmenity, err := store.HasAttributeData(ctx, search.Query{Neighborhoods: []string{"palermo"}}, "amenity")
	if err != nil || hasAmenity {
		t.Fatalf("inventory without amenity attributes cannot answer amenity questions: %v, %v", hasAmenity, err)
	}
	item := listing.Listing{Source: "zonaprop", URL: "https://ex.test/a", Neighborhood: "palermo", Description: "Departamento.", Operation: "alquiler",
		Price: listing.Money{Amount: float64Ptr(800000), Currency: "ARS"}, Attributes: []listing.Attribute{{Type: "amenity", Value: "pileta", Provenance: listing.Stated}}, ScrapedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	if err := store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReview(ctx, item.URL, quality.Review{Status: quality.Passed}); err != nil {
		t.Fatal(err)
	}
	hasAmenity, err = store.HasAttributeData(ctx, search.Query{Neighborhoods: []string{"palermo"}}, "amenity")
	if err != nil || !hasAmenity {
		t.Fatalf("an approved searchable amenity should be detected: %v, %v", hasAmenity, err)
	}
}

func TestAdmissibleFactsAreDataAndExcludeProtectedCharacteristics(t *testing.T) {
	store := openTestStore(t)
	facts, err := store.Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !facts.Admissible("guarantee") || !facts.Admissible("income_band") || facts.Admissible("age") || facts.Admissible("nationality") {
		t.Fatalf("got %v", facts)
	}
}

// The form reads the catalog from the table (specs/004, phase 1 contract), so
// the table holds every fact's label, priority, cardinality, order and choices.
func TestFactsCarryTheFormCatalog(t *testing.T) {
	store := openTestStore(t)
	facts, err := store.Facts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	yesNo := []eligibility.Choice{{Value: "yes", Label: "Sí"}, {Value: "no", Label: "No"}}
	want := map[string]eligibility.Fact{
		"guarantee": {Admissible: true, Label: "Garantía", Priority: "high", Multiple: true, Position: 1, Choices: []eligibility.Choice{
			{Value: "propietaria", Label: "Garantía propietaria"},
			{Value: "caucion", Label: "Seguro de caución"},
			{Value: "recibos_garante", Label: "Recibos de sueldo de un garante"},
			{Value: "none", Label: "No tengo"},
		}},
		"income_documented": {Admissible: true, Label: "¿Podés comprobar tus ingresos?", Priority: "high", Position: 2, Choices: yesNo},
		"pets": {Admissible: true, Label: "Mascotas", Priority: "high", Multiple: true, Position: 3, Choices: []eligibility.Choice{
			{Value: "none", Label: "Ninguna"}, {Value: "dog", Label: "Perro"}, {Value: "cat", Label: "Gato"},
			{Value: "other", Label: "Otra"},
		}},
		"income_band": {Admissible: true, Label: "Ingresos mensuales", Priority: "medium", Position: 4, Choices: []eligibility.Choice{
			{Value: "0-1000000", Label: "Hasta $1.000.000"},
			{Value: "1000000-2000000", Label: "$1.000.000 a $2.000.000"},
			{Value: "2000000-3000000", Label: "$2.000.000 a $3.000.000"},
			{Value: "3000000-", Label: "Más de $3.000.000"},
		}},
		"caucion_quoted": {Admissible: true, Label: "¿Ya cotizaste un seguro de caución?", Priority: "medium", Position: 5, Choices: yesNo},
	}
	for name, fact := range want {
		if got := facts[name]; !reflect.DeepEqual(got, fact) {
			t.Errorf("%s:\n got %+v\nwant %+v", name, got, fact)
		}
	}
	for _, protected := range []string{"age", "nationality", "gender"} {
		if facts.Admissible(protected) {
			t.Errorf("%s must stay inadmissible", protected)
		}
	}
}

// A rule nobody can evaluate is corrupt data. Saving it is refused, so a bad
// eligibility file cannot break later searches; a row that still arrives by
// other means fails loudly on read instead of reaching the evaluator.
func TestARuleWithAnUnknownOperatorIsRefusedOnWriteAndOnRead(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	url := "https://ex.test/between"
	if err := store.Save(ctx, listing.Listing{Source: "zonaprop", URL: url, Neighborhood: "palermo", Description: "Departamento.", Operation: "alquiler",
		Price: listing.Money{Amount: float64Ptr(800000), Currency: "ARS"}, ScrapedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReview(ctx, url, quality.Review{Status: quality.Passed}); err != nil {
		t.Fatal(err)
	}
	valid := eligibility.Rule{Fact: "guarantee", Operator: "one_of", Values: []string{"caucion"}, Hardness: "hard"}
	if err := store.SaveEligibility(ctx, url, []eligibility.Rule{valid}); err != nil {
		t.Fatal(err)
	}
	between := eligibility.Rule{Fact: "guarantee", Operator: "between", Values: []string{"propietaria"}, Hardness: "hard"}

	if err := store.SaveEligibility(ctx, url, []eligibility.Rule{valid, between}); err == nil || !strings.Contains(err.Error(), `"between"`) {
		t.Fatalf("want the save refused naming the unknown operator, got %v", err)
	}
	got, err := store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo"}})
	if err != nil || len(got) != 1 || len(got[0].Rules) != 1 || got[0].Rules[0].Operator != "one_of" {
		t.Fatalf("a refused save must leave the previous rules: %+v, %v", got, err)
	}

	conn, err := pgx.Connect(ctx, os.Getenv("HAUSY_TEST_DATABASE_URI"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, `INSERT INTO listing_eligibility_rules (listing_id, fact, operator, "values", hardness)
		SELECT id, 'guarantee', 'between', '["propietaria"]', 'hard' FROM listings WHERE url = $1`, url); err != nil {
		t.Fatal(err)
	}
	got, err = store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo"}})
	if err == nil || !strings.Contains(err.Error(), `"between"`) {
		t.Fatalf("want an error naming the unknown operator, got %+v, %v", got, err)
	}
}

func TestQualificationRoundTripsPerUser(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	user, err := store.CreateUser(ctx, auth.NewUser{Email: fmt.Sprintf("q%d@ex.test", time.Now().UnixNano()), Name: "Q", Role: auth.RoleSearcher, PasswordHash: "x"})
	if err != nil {
		t.Fatal(err)
	}
	q := eligibility.Qualification{"guarantee": {"caucion", "propietaria"}, "income_band": {"2000000-3000000"}}
	if err := store.SaveQualification(ctx, user.ID, q); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveQualification(ctx, user.ID, eligibility.Qualification{"guarantee": {"caucion"}}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Qualification(ctx, user.ID)
	if err != nil || len(got) != 1 || !slices.Equal(got["guarantee"], []string{"caucion"}) {
		t.Fatalf("saving replaces the whole qualification, got %v, %v", got, err)
	}
	if err := store.SaveQualification(ctx, user.ID, eligibility.Qualification{"age": {"19"}}); err == nil {
		t.Fatal("an inadmissible fact must be refused")
	}
}
