package intake_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/jev"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

// fakeJev answers by question ID; unlisted questions get a zero answer, which
// reads as "not mentioned". It records every ID it was asked.
type fakeJev struct {
	answers   map[string]jev.Answer
	asked     []string
	questions map[string]jev.Question
}

func (f *fakeJev) evaluate(_ context.Context, _ any, qs map[string]jev.Question) (map[string]jev.Answer, error) {
	out := map[string]jev.Answer{}
	if f.questions == nil {
		f.questions = map[string]jev.Question{}
	}
	for id, q := range qs {
		f.asked = append(f.asked, id)
		f.questions[id] = q
		out[id] = f.answers[id]
	}
	return out, nil
}

func pick(c string) jev.Answer { return jev.Answer{Type: "choice", Choice: c} }
func yes() jev.Answer          { return jev.Answer{Type: "boolean", Probability: 0.95} }
func planOf(t *testing.T, f *fakeJev, turns ...string) intake.Plan {
	t.Helper()
	p, err := intake.Jev{Evaluate: f.evaluate}.Plan(context.Background(), turns)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestJevPlansASimpleSearch(t *testing.T) {
	f := &fakeJev{answers: map[string]jev.Answer{"intent": pick("new_search"), "operation": pick("alquiler"), "hood_palermo": yes(), "num_0": pick("max_price")}}
	p := planOf(t, f, "Busco alquilar en Palermo hasta 800 mil pesos")
	if p.Intent != "new_search" || len(p.Branches) != 1 {
		t.Fatalf("got %+v", p)
	}
	b := p.Branches[0]
	if !slices.Equal(b.Neighborhoods, []string{"palermo"}) || b.Operation != "alquiler" || b.Currency != "ARS" || b.MaxPrice == nil || *b.MaxPrice != 800000 {
		t.Fatalf("got %+v", b)
	}
}

func TestRequirementsTiedToOneNeighborhoodBecomeSeparateBranches(t *testing.T) {
	f := &fakeJev{answers: map[string]jev.Answer{
		"intent": pick("new_search"), "hood_belgrano": yes(), "hood_monserrat": yes(), "scoped": yes(),
		"spot_amenity_cochera":           pick("excluded"),
		"belgrano:spot_amenity_cochera":  pick("excluded"),
		"monserrat:spot_amenity_cochera": pick("required"),
	}}
	p := planOf(t, f, "departamento en belgrano, olvidate de la cochera, pero buscame también en monserrat, que sí tenga")
	if len(p.Branches) != 2 {
		t.Fatalf("want two branches, got %+v", p.Branches)
	}
	byHood := map[string]search.Query{}
	for _, b := range p.Branches {
		byHood[b.Neighborhoods[0]] = b
	}
	cochera := search.AttributeFilter{Type: "amenity", Value: "cochera"}
	if !slices.Contains(byHood["belgrano"].ExcludedAttributes, cochera) || !slices.Contains(byHood["monserrat"].RequiredAttributes, cochera) {
		t.Fatalf("got %+v", byHood)
	}
}

// Seen in the live request: "Does this number (num_0) apply to the
// neighborhood palermo?" Jev never saw which number that was.
func TestAScopedNumberIsAskedAboutByItsText(t *testing.T) {
	f := &fakeJev{answers: map[string]jev.Answer{
		"intent": pick("new_search"), "hood_palermo": yes(), "hood_monserrat": yes(), "scoped": yes(), "num_0": pick("max_price"),
		"palermo:num_0": yes(),
	}}
	p := planOf(t, f, "Dos ambientes en palermo, hasta 500, o sino dos ambientes en monserrat pero que tenga pileta")
	if q := f.questions["palermo:num_0"].Instructions; !strings.Contains(q, `"500"`) {
		t.Fatalf("the per-neighborhood question must quote the number, got %q", q)
	}
	for _, b := range p.Branches {
		if slices.Equal(b.Neighborhoods, []string{"palermo"}) && (b.MaxPrice == nil || *b.MaxPrice != 500) {
			t.Fatalf("palermo carries the price, got %+v", b)
		}
		if slices.Equal(b.Neighborhoods, []string{"monserrat"}) && b.MaxPrice != nil {
			t.Fatalf("monserrat does not, got %+v", b)
		}
	}
}

func TestUnscopedNeighborhoodsShareOneBranch(t *testing.T) {
	f := &fakeJev{answers: map[string]jev.Answer{"intent": pick("new_search"), "hood_monserrat": yes(), "hood_congreso": yes(), "num_1": pick("max_price")}}
	p := planOf(t, f, "Quiero un 2 ambientes en Monserrat o Congreso, alquiler, tope 650 mil")
	if len(p.Branches) != 1 || len(p.Branches[0].Neighborhoods) != 2 || slices.Contains(f.asked, "scoped") == false {
		t.Fatalf("got %+v (asked scoped: %v)", p.Branches, slices.Contains(f.asked, "scoped"))
	}
	for _, id := range f.asked {
		if len(id) > 9 && id[:9] == "monserrat" {
			t.Fatalf("an unscoped search must not ask per-neighborhood questions, asked %s", id)
		}
	}
}

func TestSortAndVolunteeredQualification(t *testing.T) {
	f := &fakeJev{answers: map[string]jev.Answer{"intent": pick("new_search"), "hood_palermo": yes(), "sort": pick("price_asc"),
		"has_propietaria": yes(), "num_0": pick("monthly_income"), "type_natural_light": pick("high/required")}}
	p := planOf(t, f, "Buscame el departamento más barato de Palermo que tenga buena luz. Tengo garantía propietaria y gano 2,5 millones")
	if p.Sort != "price_asc" || !slices.Equal(p.Qualification["guarantee"], []string{"propietaria"}) || !slices.Equal(p.Qualification["income_band"], []string{"2000000-3000000"}) {
		t.Fatalf("got %+v", p)
	}
	if !slices.Contains(p.Branches[0].RequiredAttributes, search.AttributeFilter{Type: "natural_light", Value: "high"}) {
		t.Fatalf("got %+v", p.Branches[0])
	}
}

func TestAGreetingHasNoSearchAndAFirstMessageIsNeverARefinement(t *testing.T) {
	greeting := planOf(t, &fakeJev{answers: map[string]jev.Answer{"intent": pick("other")}}, "Hola, ¿cómo funciona esto?")
	if len(greeting.Branches) != 0 {
		t.Fatalf("a greeting must not search, got %+v", greeting.Branches)
	}
	first := planOf(t, &fakeJev{answers: map[string]jev.Answer{"intent": pick("refine"), "hood_palermo": yes()}}, "Alquiler en Palermo")
	if first.Intent != "new_search" {
		t.Fatalf("a first message starts a search, got %q", first.Intent)
	}
}
