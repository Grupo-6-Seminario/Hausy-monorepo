// Package intake turns a searcher's conversation into a typed search plan.
// Language models only fill the plan; everything downstream is code.
package intake

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

// Plan is what the searcher wants after their latest message.
type Plan struct {
	// Intent: new_search | refine | ask_about_listing | other.
	Intent string `json:"intent"`
	// Branches run as separate queries; a branch is a search.Query. There is
	// more than one only when requirements differ by neighborhood.
	Branches []search.Query `json:"branches"`
	// Sort within each eligibility section: relevance (default) | price_asc |
	// price_desc | area_desc.
	Sort string `json:"sort,omitempty"`
	// Qualification the searcher volunteered in the chat.
	Qualification eligibility.Qualification `json:"qualification,omitempty"`
	// PlannedBy is primary | fallback, so the fallback rate can be measured.
	PlannedBy string `json:"planned_by,omitempty"`
}

// Source fills a plan from the conversation's user turns. Jev and the local
// model are the two adapters.
type Source interface {
	Plan(ctx context.Context, turns []string) (Plan, error)
}

// Planner runs Primary within Budget and falls back when it is slow, fails,
// or returns a plan that does not validate. Budget bounds Primary only: the
// fallback is the last planner left and gets the caller's full deadline.
type Planner struct {
	Primary, Fallback Source
	Budget            time.Duration
}

func (p Planner) Plan(ctx context.Context, turns []string, previous Plan) (Plan, error) {
	primaryCtx := ctx
	if p.Budget > 0 {
		var cancel context.CancelFunc
		primaryCtx, cancel = context.WithTimeout(ctx, p.Budget)
		defer cancel()
	}
	plan, err := p.try(primaryCtx, p.Primary, turns, previous)
	if err == nil {
		plan.PlannedBy = "primary"
		return plan, nil
	}
	if p.Fallback == nil {
		return Plan{}, err
	}
	// Logged so the fallback rate can be measured (plan Q7).
	log.Printf("intake: primary planner failed, falling back: %v", err)
	fallback, ferr := p.try(ctx, p.Fallback, turns, previous)
	if ferr != nil {
		return Plan{}, errors.Join(err, ferr)
	}
	fallback.PlannedBy = "fallback"
	return fallback, nil
}

func (p Planner) try(ctx context.Context, source Source, turns []string, previous Plan) (Plan, error) {
	if source == nil {
		return Plan{}, errors.New("intake: no planner configured")
	}
	plan, err := source.Plan(ctx, turns)
	if err != nil {
		return Plan{}, err
	}
	return resolve(plan, previous)
}

// resolve applies the turn rules both sources share: a question about a
// listing never changes the search, rent is the default operation, a bare
// peso amount means thousands, and no branch reaches SQL without validating.
func resolve(plan, previous Plan) (Plan, error) {
	if plan.Intent == "ask_about_listing" && len(previous.Branches) > 0 {
		frozen := previous
		frozen.Intent = plan.Intent
		return frozen, nil
	}
	for i, branch := range plan.Branches {
		if branch.Operation == "" {
			branch.Operation = "alquiler"
		}
		barePesos(&branch)
		valid, err := branch.Validate()
		if err != nil {
			return Plan{}, fmt.Errorf("intake: branch %d: %w", i, err)
		}
		plan.Branches[i] = valid
	}
	if plan.Sort == "" {
		plan.Sort = "relevance"
	}
	return plan, nil
}

// barePesos reads a peso price or expensas bound under 10.000 as thousands:
// "hasta 500" is 500 mil, since nothing in CABA rents or charges expensas for
// ARS 500. Dollar amounts are left as said.
func barePesos(q *search.Query) {
	scale := func(f **float64) {
		if *f != nil && **f < 10000 {
			thousands := **f * 1000
			*f = &thousands
		}
	}
	if q.Currency == "ARS" {
		scale(&q.MinPrice)
		scale(&q.MaxPrice)
	}
	scale(&q.MaxExpensesARS)
}
