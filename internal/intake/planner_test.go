package intake_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/jev"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

type sourceFunc func(context.Context, []string) (intake.Plan, error)

func (f sourceFunc) Plan(ctx context.Context, turns []string) (intake.Plan, error) {
	return f(ctx, turns)
}

func fixed(p intake.Plan) sourceFunc {
	return func(context.Context, []string) (intake.Plan, error) { return p, nil }
}

var palermo = intake.Plan{Intent: "new_search", Branches: []search.Query{{Neighborhoods: []string{"palermo"}, Operation: "alquiler"}}}

func TestSlowPrimaryFallsBackWithinTheBudget(t *testing.T) {
	hang := sourceFunc(func(ctx context.Context, _ []string) (intake.Plan, error) {
		<-ctx.Done()
		return intake.Plan{}, ctx.Err()
	})
	p := intake.Planner{Primary: hang, Fallback: fixed(palermo), Budget: 20 * time.Millisecond}
	start := time.Now()
	got, err := p.Plan(context.Background(), []string{"Alquiler en Palermo"}, intake.Plan{})
	if err != nil || got.PlannedBy != "fallback" || got.Branches[0].Neighborhoods[0] != "palermo" || time.Since(start) > time.Second {
		t.Fatalf("got %+v, %v after %v", got, err, time.Since(start))
	}
}

// The budget bounds Jev, not the local model: seen live, Jev spent its 6 s on
// Gateway 503s and the local planner, needing 7 s, was cut off too.
func TestFallbackIsNotBoundByThePrimaryBudget(t *testing.T) {
	budget := 20 * time.Millisecond
	hang := sourceFunc(func(ctx context.Context, _ []string) (intake.Plan, error) {
		<-ctx.Done()
		return intake.Plan{}, ctx.Err()
	})
	slow := sourceFunc(func(ctx context.Context, _ []string) (intake.Plan, error) {
		select {
		case <-ctx.Done():
			return intake.Plan{}, ctx.Err()
		case <-time.After(3 * budget):
			return palermo, nil
		}
	})
	got, err := intake.Planner{Primary: hang, Fallback: slow, Budget: budget}.Plan(context.Background(), []string{"Alquiler en Palermo"}, intake.Plan{})
	if err != nil || got.PlannedBy != "fallback" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestFailingPrimaryFallsBack(t *testing.T) {
	failing := sourceFunc(func(context.Context, []string) (intake.Plan, error) { return intake.Plan{}, errors.New("HTTP 503") })
	got, err := intake.Planner{Primary: failing, Fallback: fixed(palermo), Budget: time.Second}.Plan(context.Background(), []string{"x"}, intake.Plan{})
	if err != nil || got.PlannedBy != "fallback" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestPlannerFallbackLogsReasonClassWithoutProviderBody(t *testing.T) {
	var logs bytes.Buffer
	ctx := logging.WithLogger(context.Background(), slog.New(slog.NewJSONHandler(&logs, nil)).With("request_id", "test-request"))
	failing := sourceFunc(func(context.Context, []string) (intake.Plan, error) {
		return intake.Plan{}, errors.New("private provider response")
	})
	got, err := (intake.Planner{Primary: failing, Fallback: fixed(palermo)}).Plan(ctx, []string{"private search text"}, intake.Plan{})
	if err != nil || got.PlannedBy != "fallback" {
		t.Fatalf("expected fallback plan, got %+v, %v", got, err)
	}
	if !strings.Contains(logs.String(), `"msg":"planner_fallback"`) || !strings.Contains(logs.String(), `"error_class":"failed"`) {
		t.Fatalf("fallback reason missing from structured log: %s", logs.String())
	}
	if strings.Contains(logs.String(), "private provider response") || strings.Contains(logs.String(), "private search text") {
		t.Fatalf("private content leaked: %s", logs.String())
	}
}

func TestQuestionAboutAListingFreezesThePlan(t *testing.T) {
	asked := intake.Plan{Intent: "ask_about_listing", Branches: []search.Query{{Neighborhoods: []string{"congreso"}, RequiredAttributes: []search.AttributeFilter{{Type: "outdoor_space", Value: "balcon"}}}}}
	got, err := intake.Planner{Primary: fixed(asked), Budget: time.Second}.Plan(context.Background(), []string{"...", "¿el primero tiene balcón?"}, palermo)
	if err != nil || got.Intent != "ask_about_listing" || len(got.Branches) != 1 || got.Branches[0].Neighborhoods[0] != "palermo" || len(got.Branches[0].RequiredAttributes) != 0 {
		t.Fatalf("the previous search must survive a question about a listing, got %+v, %v", got, err)
	}
}

// Captured live: the local model required every amenity for "con amenities".
// Only amenities the searcher names may stay required; otherwise the plan
// comes back unresolved, with no amenity invented, for clarification.
func TestGenericAmenitiesKeepOnlyWhatTheSearcherNamed(t *testing.T) {
	// A source returns a fresh plan per call; resolve writes into it.
	everyAmenity := func(context.Context, []string) (intake.Plan, error) {
		plan := intake.Plan{Intent: "new_search", Branches: []search.Query{{Neighborhoods: []string{"palermo"}}}}
		for _, value := range []string{"pileta", "gimnasio", "laundry", "coworking", "sum", "seguridad", "parrilla", "ascensor", "cochera", "solarium", "terraza_comun"} {
			plan.Branches[0].RequiredAttributes = append(plan.Branches[0].RequiredAttributes, search.AttributeFilter{Type: "amenity", Value: value})
		}
		return plan, nil
	}
	for _, tc := range []struct {
		message    string
		want       []search.AttributeFilter
		unresolved bool
	}{
		{"Busco en Palermo con amenities", nil, true},
		{"Busco en Palermo con amenities: pileta y gym", []search.AttributeFilter{{Type: "amenity", Value: "pileta"}, {Type: "amenity", Value: "gimnasio"}}, false},
		// A generic request in other words: nothing named, nothing required;
		// asking which amenities is the clarification judge's job.
		{"Busco en Palermo, que tenga todos los chiches del edificio", nil, false},
		{"Busco en Palermo con natatorio", []search.AttributeFilter{{Type: "amenity", Value: "pileta"}}, false},
	} {
		plan, err := (intake.Planner{Primary: sourceFunc(everyAmenity)}).Plan(context.Background(), []string{tc.message}, intake.Plan{})
		if (err != nil) != tc.unresolved || len(plan.Branches) != 1 || !slices.Equal(plan.Branches[0].RequiredAttributes, tc.want) {
			t.Errorf("%q: want %v (unresolved=%v), got %+v %v", tc.message, tc.want, tc.unresolved, plan.Branches, err)
		}
	}
}

// Measured live 2026-09-25: Jev planned "dos ambientes en palermo con
// amenities" in about a second, then every such turn also ran the Bedrock
// fallback for about 2.5 s, because unnamed amenities looked like a failed
// plan. No planner can name them; asking is the clarification's job.
func TestUnnamedAmenitiesAreNotAPlannerFailure(t *testing.T) {
	jev := sourceFunc(func(context.Context, []string) (intake.Plan, error) {
		two := 2
		return intake.Plan{Intent: "new_search", Branches: []search.Query{{Neighborhoods: []string{"palermo"}, MinRooms: &two, MaxRooms: &two}}}, nil
	})
	fallbackCalls := 0
	fallback := sourceFunc(func(context.Context, []string) (intake.Plan, error) {
		fallbackCalls++
		return intake.Plan{}, errors.New("the fallback cannot name them either")
	})
	plan, err := intake.Planner{Primary: jev, Fallback: fallback}.Plan(context.Background(), []string{"dos ambientes en palermo con amenities"}, intake.Plan{})
	if err == nil || fallbackCalls != 0 || plan.PlannedBy != "primary" || len(plan.Branches) != 1 || *plan.Branches[0].MinRooms != 2 || len(plan.Branches[0].RequiredAttributes) != 0 {
		t.Fatalf("want Jev's plan back unresolved without a fallback call, got %d fallback calls, %+v, %v", fallbackCalls, plan, err)
	}
}

// Captured live: the local model planned "dos habitaciones" as two ambientes.
// A habitación is a dormitorio (CONTEXT.md), on this turn and on later ones.
func TestHabitacionesAreDormitorios(t *testing.T) {
	two := 2
	misread := func(context.Context, []string) (intake.Plan, error) {
		return intake.Plan{Intent: "new_search", Branches: []search.Query{{Neighborhoods: []string{"palermo"}, MinRooms: &two}}}, nil
	}
	for _, turns := range [][]string{{"Busco dos habitaciones en Palermo"}, {"Busco 2 habitaciones en Palermo", "Ordenalos por precio"}} {
		plan, err := (intake.Planner{Primary: sourceFunc(misread)}).Plan(context.Background(), turns, intake.Plan{})
		if err != nil || plan.Branches[0].MinRooms != nil || plan.Branches[0].MinBedrooms == nil || *plan.Branches[0].MinBedrooms != 2 {
			t.Errorf("%q: want two dormitorios and no ambientes, got %+v %v", turns, plan.Branches, err)
		}
	}
}

// Captured live (twice, temperature 0): the local model turned "algo barato"
// into max_price 800000, a hard filter nobody stated. A price bound must be a
// number the searcher said; "barato" alone is at most an order.
func TestAPriceBoundMustBeANumberTheSearcherSaid(t *testing.T) {
	for _, tc := range []struct {
		message string
		said    float64
		want    *float64
	}{
		{"Algo barato en Monserrat para un estudiante", 800000, nil},
		{"Algo en Monserrat hasta 900 mil pesos", 900000, ptr(900000)},
		{"Algo en Monserrat, puedo pagar hasta 900", 900, ptr(900000)},
	} {
		said := tc.said
		guessed := func(context.Context, []string) (intake.Plan, error) {
			return intake.Plan{Intent: "new_search", Branches: []search.Query{{Neighborhoods: []string{"monserrat"}, Currency: "ARS", MaxPrice: &said}}}, nil
		}
		plan, err := (intake.Planner{Primary: sourceFunc(guessed)}).Plan(context.Background(), []string{tc.message}, intake.Plan{})
		got := plan.Branches[0].MaxPrice
		if err != nil || (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
			t.Errorf("%q: max price %v, want %v (%v)", tc.message, deref(got), deref(tc.want), err)
		}
	}
}

func ptr(f float64) *float64 { return &f }

func deref(f *float64) any {
	if f == nil {
		return nil
	}
	return *f
}

func TestUnstatedOperationDefaultsToRent(t *testing.T) {
	noOp := intake.Plan{Intent: "new_search", Branches: []search.Query{{Neighborhoods: []string{"palermo"}}}}
	got, err := intake.Planner{Primary: fixed(noOp), Budget: time.Second}.Plan(context.Background(), []string{"Departamento luminoso en Palermo"}, intake.Plan{})
	if err != nil || got.Branches[0].Operation != "alquiler" || got.PlannedBy != "primary" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

// Seen live: "…hasta 900 mil, tengo garantía propietaria" never says rent or
// buy, and Jev picked venta over alquiler (0.58 / 0.20) in 3 of 3 runs; the sale
// search under ARS 900.000 came back empty. Only the user's words make it a sale.
func TestASaleTheUserNeverStatedIsARent(t *testing.T) {
	venta := jev.Answer{Type: "choice", Choice: "venta", Probabilities: map[string]float64{"alquiler": 0.20, "venta": 0.58, "unspecified": 0.22}}
	implied := &fakeJev{answers: map[string]jev.Answer{"intent": pick("new_search"), "operation": venta, "num_0": pick("rooms_exact"), "num_1": pick("max_price"), "has_propietaria": yes()}}
	got, err := intake.Planner{Primary: intake.Jev{Evaluate: implied.evaluate}}.Plan(context.Background(), []string{"Busco un 2 ambientes hasta 900 mil, tengo garantía propietaria"}, intake.Plan{})
	if err != nil || got.Branches[0].Operation != "alquiler" || *got.Branches[0].MaxPrice != 900000 {
		t.Fatalf("an implied rental must search rentals, got %+v, %v", got.Branches, err)
	}

	for _, turns := range [][]string{
		{"Busco comprar en Palermo, hasta USD 150.000, 3 ambientes"},
		{"Alquiler en Palermo hasta 800 mil", "¿y si quisiera comprar?"},
		{"Departamento en venta en Belgrano"},
	} {
		stated := &fakeJev{answers: map[string]jev.Answer{"intent": pick("new_search"), "operation": pick("venta")}}
		got, err := intake.Planner{Primary: intake.Jev{Evaluate: stated.evaluate}}.Plan(context.Background(), turns, intake.Plan{})
		if err != nil || got.Branches[0].Operation != "venta" {
			t.Fatalf("%q states a sale, got %+v, %v", turns, got.Branches, err)
		}
	}
}

// Seen live: Jev read "hasta 500" as ARS 500, which no CABA rental matches, and
// the turn came back empty. A peso amount that small means thousands.
func TestABarePesoAmountMeansThousands(t *testing.T) {
	f := &fakeJev{answers: map[string]jev.Answer{"intent": pick("new_search"), "hood_palermo": yes(), "num_0": pick("max_price"), "num_1": pick("max_expenses")}}
	got, err := intake.Planner{Primary: intake.Jev{Evaluate: f.evaluate}}.Plan(context.Background(), []string{"Dos ambientes en palermo, hasta 500, expensas hasta 90"}, intake.Plan{})
	if err != nil {
		t.Fatal(err)
	}
	b := got.Branches[0]
	if b.Currency != "ARS" || b.MaxPrice == nil || *b.MaxPrice != 500000 || b.MaxExpensesARS == nil || *b.MaxExpensesARS != 90000 {
		t.Fatalf("want ARS 500.000 and expensas 90.000, got %+v", b)
	}

	usd := &fakeJev{answers: map[string]jev.Answer{"intent": pick("new_search"), "hood_palermo": yes(), "num_0": pick("max_price")}}
	got, err = intake.Planner{Primary: intake.Jev{Evaluate: usd.evaluate}}.Plan(context.Background(), []string{"Palermo, hasta 500 dólares"}, intake.Plan{})
	if err != nil || got.Branches[0].Currency != "USD" || *got.Branches[0].MaxPrice != 500 {
		t.Fatalf("a dollar amount stays as said, got %+v, %v", got.Branches, err)
	}
}

func TestInvalidBranchIsAnErrorWhenNothingElseCanPlan(t *testing.T) {
	bad := intake.Plan{Intent: "new_search", Branches: []search.Query{{Currency: "EUR"}}}
	if _, err := (intake.Planner{Primary: fixed(bad), Budget: time.Second}).Plan(context.Background(), []string{"x"}, intake.Plan{}); err == nil {
		t.Fatal("an invalid query must not reach SQL")
	}
}
