package postgres_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
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
}

func TestAdmissibleFactsAreDataAndExcludeProtectedCharacteristics(t *testing.T) {
	store := openTestStore(t)
	facts, err := store.AdmissibleFacts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !facts["guarantee"] || !facts["income_band"] || facts["age"] || facts["nationality"] {
		t.Fatalf("got %v", facts)
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
