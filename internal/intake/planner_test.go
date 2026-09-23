package intake_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
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

func TestFailingPrimaryFallsBack(t *testing.T) {
	failing := sourceFunc(func(context.Context, []string) (intake.Plan, error) { return intake.Plan{}, errors.New("HTTP 503") })
	got, err := intake.Planner{Primary: failing, Fallback: fixed(palermo), Budget: time.Second}.Plan(context.Background(), []string{"x"}, intake.Plan{})
	if err != nil || got.PlannedBy != "fallback" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestQuestionAboutAListingFreezesThePlan(t *testing.T) {
	asked := intake.Plan{Intent: "ask_about_listing", Branches: []search.Query{{Neighborhoods: []string{"congreso"}, RequiredAttributes: []search.AttributeFilter{{Type: "outdoor_space", Value: "balcon"}}}}}
	got, err := intake.Planner{Primary: fixed(asked), Budget: time.Second}.Plan(context.Background(), []string{"...", "¿el primero tiene balcón?"}, palermo)
	if err != nil || got.Intent != "ask_about_listing" || len(got.Branches) != 1 || got.Branches[0].Neighborhoods[0] != "palermo" || len(got.Branches[0].RequiredAttributes) != 0 {
		t.Fatalf("the previous search must survive a question about a listing, got %+v, %v", got, err)
	}
}

func TestUnstatedOperationDefaultsToRent(t *testing.T) {
	noOp := intake.Plan{Intent: "new_search", Branches: []search.Query{{Neighborhoods: []string{"palermo"}}}}
	got, err := intake.Planner{Primary: fixed(noOp), Budget: time.Second}.Plan(context.Background(), []string{"Departamento luminoso en Palermo"}, intake.Plan{})
	if err != nil || got.Branches[0].Operation != "alquiler" || got.PlannedBy != "primary" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestInvalidBranchIsAnErrorWhenNothingElseCanPlan(t *testing.T) {
	bad := intake.Plan{Intent: "new_search", Branches: []search.Query{{Currency: "EUR"}}}
	if _, err := (intake.Planner{Primary: fixed(bad), Budget: time.Second}).Plan(context.Background(), []string{"x"}, intake.Plan{}); err == nil {
		t.Fatal("an invalid query must not reach SQL")
	}
}
