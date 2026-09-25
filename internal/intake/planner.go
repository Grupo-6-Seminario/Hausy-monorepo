// Package intake turns a searcher's conversation into a typed search plan.
// Language models only fill the plan; everything downstream is code.
package intake

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
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
		return plan, err
	}
	logging.FromContext(ctx).LogAttrs(ctx, slog.LevelWarn, "planner_fallback",
		slog.String("error_class", logging.ErrorClass(err)))
	fallback, ferr := p.try(ctx, p.Fallback, turns, previous)
	if ferr != nil {
		if len(plan.Branches) > 0 {
			return plan, errors.Join(err, ferr)
		}
		return fallback, errors.Join(err, ferr)
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
	return resolve(plan, previous, turns)
}

// saleCue is the user saying they want to buy. Seen live: Jev read a bare
// "hasta 900 mil" as a sale price, but in CABA a peso amount is a monthly rent
// and sales are quoted in dollars.
var saleCue = regexp.MustCompile(`\b(compr(a|as|o|ar|arme|amos|aria)|venta|adquirir|invertir|inversion)\b`)

// resolve applies the turn rules both sources share: a question about a
// listing never changes the search, rent is the default operation and a sale
// is only what the user says, a bare peso amount means thousands, and no
// branch reaches SQL without validating.
func resolve(plan, previous Plan, turns []string) (Plan, error) {
	if plan.Intent == "ask_about_listing" && len(previous.Branches) > 0 {
		frozen := previous
		frozen.Intent = plan.Intent
		return frozen, nil
	}
	conversation := normalize(strings.Join(turns, "\n"))
	stated := saleCue.MatchString(conversation)
	latest := ""
	if len(turns) > 0 {
		latest = normalize(turns[len(turns)-1])
	}
	said := extractNumbers(turns)
	for i, branch := range plan.Branches {
		// A required amenity must be one some turn names: the model has
		// expanded a generic request into every known amenity in real runs,
		// on its turn and on later ones. A generic request is the
		// clarification's to resolve; "con amenities" left unnamed on this
		// turn stays as the unresolved amenity=any placeholder.
		var kept []search.AttributeFilter
		named := false
		for _, f := range branch.RequiredAttributes {
			if f.Type != "amenity" {
				kept = append(kept, f)
			} else if listing.NamesAmenity(f.Value, conversation) {
				kept, named = append(kept, f), named || listing.NamesAmenity(f.Value, latest)
			}
		}
		if strings.Contains(latest, "con amenities") && !named {
			kept = append(kept, search.AttributeFilter{Type: "amenity", Value: "any"})
		}
		branch.RequiredAttributes = kept
		if n, ok := habitaciones(conversation); ok {
			// Seen live: "dos habitaciones" planned as two ambientes.
			if branch.MinRooms != nil && *branch.MinRooms == n && (branch.MaxRooms == nil || *branch.MaxRooms == n) {
				branch.MinRooms, branch.MaxRooms = nil, nil
			}
			branch.MinBedrooms = &n
		}
		if branch.Operation == "" || branch.Operation == "venta" && !stated {
			branch.Operation = "alquiler"
		}
		barePesos(&branch)
		dropUnsaidPrice(&branch, said)
		plan.Branches[i] = branch
		valid, err := branch.Validate()
		if err != nil {
			return plan, fmt.Errorf("intake: branch %d: %w", i, err)
		}
		plan.Branches[i] = valid
	}
	if plan.Sort == "" {
		plan.Sort = "relevance"
	}
	return plan, nil
}

// dropUnsaidPrice removes a price bound no turn stated. Seen live: "algo
// barato" became max_price 800000, a hard filter nobody asked for. A bound
// survives if a turn said that amount, as is or as bare-peso thousands.
func dropUnsaidPrice(q *search.Query, said []mention) {
	for _, bound := range []**float64{&q.MinPrice, &q.MaxPrice} {
		if *bound == nil {
			continue
		}
		stated := false
		for _, m := range said {
			stated = stated || m.Value == **bound || m.Value*1000 == **bound
		}
		if !stated {
			*bound = nil
		}
	}
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
