package clarification_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/clarification"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/jev"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

func number(n int) *int { return &n }

func TestTypedAnswerPreservesAnIndependentRoomCondition(t *testing.T) {
	plan := intake.Plan{Branches: []search.Query{{MinRooms: number(3), MaxRooms: number(3)}}}
	resolved, err := clarification.Apply(plan, []clarification.Effect{{Field: "min_bedrooms", Value: "2"}}, catalog)
	if err != nil || *resolved.Branches[0].MinRooms != 3 || *resolved.Branches[0].MaxRooms != 3 || *resolved.Branches[0].MinBedrooms != 2 {
		t.Fatalf("two bedrooms must not erase an independent three-room requirement: %+v %v", resolved, err)
	}
	if plan.Branches[0].MinBedrooms != nil {
		t.Fatal("applying an answer mutated the tentative plan")
	}
}

func TestTypedAnswerCanReplaceOnlyTheDraftFieldsTiedToItsAmbiguity(t *testing.T) {
	plan := intake.Plan{Branches: []search.Query{{MinRooms: number(2), MaxRooms: number(2)}}}
	resolved, err := clarification.Apply(plan, []clarification.Effect{{Field: "min_bedrooms", Value: "2", Clear: []string{"min_rooms", "max_rooms"}}}, catalog)
	if err != nil || resolved.Branches[0].MinRooms != nil || resolved.Branches[0].MaxRooms != nil || *resolved.Branches[0].MinBedrooms != 2 {
		t.Fatalf("the selected meaning should replace only its tentative room reading: %+v %v", resolved, err)
	}
}

// catalog is migration 0007's qualification catalog.
var catalog = eligibility.Catalog{
	"guarantee": {Admissible: true, Multiple: true, Choices: []eligibility.Choice{
		{Value: "propietaria", Label: "Garantía propietaria"}, {Value: "caucion", Label: "Seguro de caución"},
		{Value: "recibos_garante", Label: "Recibos de sueldo de un garante"}, {Value: "none", Label: "No tengo"},
	}},
	"pets": {Admissible: true, Multiple: true, Choices: []eligibility.Choice{
		{Value: "none", Label: "Ninguna"}, {Value: "dog", Label: "Perro"}, {Value: "cat", Label: "Gato"}, {Value: "other", Label: "Otra"},
	}},
	"income_band": {Admissible: true, Choices: []eligibility.Choice{
		{Value: "0-1000000", Label: "Hasta $1.000.000"}, {Value: "3000000-", Label: "Más de $3.000.000"},
	}},
	"age": {Admissible: false},
}

// Spec 006, error 3: a clarification can set any value the form offers, and
// none the form does not.
func TestQualificationEffectsFollowTheCatalog(t *testing.T) {
	for _, tc := range []struct {
		effect clarification.Effect
		valid  bool
	}{
		{clarification.Effect{Field: "qualification.pets", Value: "dog"}, true},
		{clarification.Effect{Field: "qualification.guarantee", Value: "recibos_garante"}, true},
		{clarification.Effect{Field: "qualification.guarantee", Value: "none"}, true},
		{clarification.Effect{Field: "qualification.guarantee", Value: "aval_bancario"}, false},
		{clarification.Effect{Field: "qualification.age", Value: "30"}, false},
	} {
		if err := clarification.ValidateEffect(tc.effect, catalog); (err == nil) != tc.valid {
			t.Errorf("%s=%s: want valid %v, got %v", tc.effect.Field, tc.effect.Value, tc.valid, err)
		}
	}
	pets := clarification.Question{Source: "tengo mascota", Prompt: "¿Qué mascota tenés?", Kind: "qualification", Fact: "pets", Choices: []clarification.Choice{
		{ID: "dog", Label: "Perro", Effects: []clarification.Effect{{Field: "qualification.pets", Value: "dog"}}},
		{ID: "cat", Label: "Gato", Effects: []clarification.Effect{{Field: "qualification.pets", Value: "cat"}}},
	}}
	if err := clarification.Validate(pets, []string{"Busco en Palermo, tengo mascota"}, catalog); err != nil {
		t.Errorf("a pets question must be valid: %v", err)
	}
}

// Spec 006, error 3: asked which guarantee they have, the searcher picks among
// the catalog's choices, labelled as the form labels them.
func TestTheGuaranteeQuestionOffersTheCatalogChoices(t *testing.T) {
	yes := func(_ context.Context, _ any, qs map[string]jev.Question) (map[string]jev.Answer, error) {
		out := map[string]jev.Answer{}
		for id := range qs {
			out[id] = jev.Answer{Type: "boolean", Probability: 0.95}
		}
		return out, nil
	}
	questions, err := clarification.Judge{Evaluate: yes}.Propose(context.Background(), []string{"Busco en Palermo, tengo garantía"}, intake.Plan{}, "", nil, catalog)
	if err != nil {
		t.Fatal(err)
	}
	var got []clarification.Choice
	for _, q := range questions {
		if q.Fact == "guarantee" {
			got = q.Choices
		}
	}
	effect := func(v string) []clarification.Effect {
		return []clarification.Effect{{Field: "qualification.guarantee", Value: v}}
	}
	want := []clarification.Choice{
		{ID: "propietaria", Label: "Garantía propietaria", Effects: effect("propietaria")},
		{ID: "caucion", Label: "Seguro de caución", Effects: effect("caucion")},
		{ID: "recibos_garante", Label: "Recibos de sueldo de un garante", Effects: effect("recibos_garante")},
		{ID: "none", Label: "No tengo", Effects: effect("none")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
