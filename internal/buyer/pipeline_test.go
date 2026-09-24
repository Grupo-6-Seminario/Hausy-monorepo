package buyer_test

import (
	"context"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

type fakePlanner struct {
	plans    []intake.Plan
	turns    [][]string
	previous []intake.Plan
}

func (f *fakePlanner) Plan(_ context.Context, turns []string, previous intake.Plan) (intake.Plan, error) {
	f.turns, f.previous = append(f.turns, turns), append(f.previous, previous)
	p := f.plans[0]
	if len(f.plans) > 1 {
		f.plans = f.plans[1:]
	}
	return p, nil
}

type stock struct {
	byHood map[string][]eligibility.Candidate
}

func (f stock) Candidates(_ context.Context, q search.Query) ([]eligibility.Candidate, error) {
	var out []eligibility.Candidate
	for _, h := range q.Neighborhoods {
		out = append(out, f.byHood[h]...)
	}
	return out, nil
}

func (stock) AdmissibleFacts(context.Context) (map[string]bool, error) {
	return map[string]bool{"guarantee": true, "income_band": true}, nil
}

type fakeWriter struct{ packets []buyer.Packet }

func (f *fakeWriter) Write(_ context.Context, p buyer.Packet, _ func(string)) (string, error) {
	f.packets = append(f.packets, p)
	return "respuesta", nil
}

func candidate(url, hood string, price float64, rules []eligibility.Rule, attrs ...listing.Attribute) eligibility.Candidate {
	return eligibility.Candidate{Listing: listing.Listing{URL: url, Neighborhood: hood, Price: listing.Money{Amount: &price, Currency: "ARS"}, Attributes: attrs}, Rules: rules}
}

var (
	propietariaOnly = []eligibility.Rule{{Fact: "guarantee", Operator: "one_of", Values: []string{"propietaria"}, Hardness: "hard", Evidence: "Garantía propietaria"}}
	ownerDecides    = []eligibility.Rule{{Fact: "guarantee", Operator: "one_of", Values: []string{"propietaria", "caucion"}, Hardness: "discretionary", Evidence: "ver cuáles permite la propietaria"}}
	caucionOnly     = []eligibility.Rule{{Fact: "guarantee", Operator: "one_of", Values: []string{"caucion"}, Hardness: "hard", Evidence: "Seguro de caución (excluyente)"}}
	bright          = listing.Attribute{Type: "natural_light", Value: "high", Provenance: listing.Stated, Evidence: "muy luminoso"}
)

// Worked example: the cheapest listing is unknown and the priciest eligible;
// order must follow eligibility, never price.
func fourStates() stock {
	return stock{byHood: map[string][]eligibility.Candidate{"palermo": {
		candidate("unknown", "palermo", 500000, nil),
		candidate("ineligible", "palermo", 600000, caucionOnly),
		candidate("conditional", "palermo", 700000, ownerDecides),
		candidate("eligible", "palermo", 900000, propietariaOnly),
	}}}
}

func palermoPlan(sort string) intake.Plan {
	return intake.Plan{Intent: "new_search", Sort: sort, Branches: []search.Query{{Neighborhoods: []string{"palermo"}, Operation: "alquiler"}}}
}

func turn(t *testing.T, agent *buyer.DefaultAgent, message string, q eligibility.Qualification) *buyer.TurnResponse {
	t.Helper()
	resp, err := agent.HandleMessage(context.Background(), "s", message, q, buyer.Events{})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func urls(resp *buyer.TurnResponse) []string {
	var out []string
	for _, r := range resp.Listings {
		out = append(out, r.URL)
	}
	return out
}

func TestResultsAreOrderedByEligibilityWithIneligibleHidden(t *testing.T) {
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, fourStates(), &fakeWriter{}))
	resp := turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"propietaria"}})
	got := urls(resp)
	if len(got) != 3 || got[0] != "eligible" || got[1] != "conditional" || got[2] != "unknown" {
		t.Fatalf("want eligible, conditional, unknown; got %v", got)
	}
	for i, r := range resp.Listings {
		if r.Rank != i+1 {
			t.Fatalf("ranks must follow the shown order, got %v at %d", r.Rank, i)
		}
	}
	if resp.Listings[1].Eligibility.State != eligibility.ConditionallyEligible || resp.Listings[1].Eligibility.Conditions[0].Rule.Evidence != "ver cuáles permite la propietaria" {
		t.Fatalf("the condition must be named, got %+v", resp.Listings[1].Eligibility)
	}
	if len(resp.Relaxations) != 1 || resp.Relaxations[0] != (eligibility.Relaxation{Fact: "guarantee", Value: "caucion", Count: 1}) {
		t.Fatalf("want 'si conseguís caución, vuelve 1', got %+v", resp.Relaxations)
	}
}

func TestWithinASectionTheUsersSortWinsElseRequirementFit(t *testing.T) {
	inv := stock{byHood: map[string][]eligibility.Candidate{"palermo": {
		candidate("cheap-dark", "palermo", 500000, nil),
		candidate("pricey-bright", "palermo", 900000, nil, bright),
	}}}
	plan := palermoPlan("relevance")
	plan.Branches[0].PreferredAttributes = []search.AttributeFilter{{Type: "natural_light", Value: "high"}}
	byFit := turn(t, buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{plan}}, inv, &fakeWriter{})), "luminoso", nil)
	if urls(byFit)[0] != "pricey-bright" {
		t.Fatalf("relevance must follow requirement fit, got %v", urls(byFit))
	}
	plan.Sort = "price_asc"
	byPrice := turn(t, buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{plan}}, inv, &fakeWriter{})), "el más barato", nil)
	if urls(byPrice)[0] != "cheap-dark" {
		t.Fatalf("an explicit sort must win within the section, got %v", urls(byPrice))
	}
}

func TestBranchesMergeIntoOneListAndEmptyBranchesAreReported(t *testing.T) {
	plan := intake.Plan{Intent: "new_search", Branches: []search.Query{
		{Neighborhoods: []string{"palermo"}}, {Neighborhoods: []string{"caballito"}},
	}}
	w := &fakeWriter{}
	resp := turn(t, buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{plan}}, fourStates(), w)), "Palermo o Caballito", eligibility.Qualification{"guarantee": {"propietaria"}})
	if len(resp.Listings) != 3 {
		t.Fatalf("got %v", urls(resp))
	}
	branches := w.packets[0].Branches
	if len(branches) != 2 || branches[1].Matches != 0 || branches[1].Neighborhoods[0] != "caballito" {
		t.Fatalf("an empty branch must stay visible, got %+v", branches)
	}
}

func TestChatQualificationAddsToTheDeclaredOneAndPlansSeeTheHistory(t *testing.T) {
	second := palermoPlan("relevance")
	second.Intent = "refine"
	second.Qualification = eligibility.Qualification{"guarantee": {"caucion"}}
	planner := &fakePlanner{plans: []intake.Plan{palermoPlan("relevance"), second}}
	agent := buyer.NewAgent(nil, buyer.WithPipeline(planner, fourStates(), &fakeWriter{}))
	turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"propietaria"}})
	resp := turn(t, agent, "tengo caución también", eligibility.Qualification{"guarantee": {"propietaria"}})
	if len(resp.Listings) != 4 || len(resp.Relaxations) != 0 {
		t.Fatalf("with caución declared in the chat the caución-only listing returns, got %v", urls(resp))
	}
	if len(planner.turns[1]) != 2 || len(planner.previous[1].Branches) != 1 {
		t.Fatalf("the planner must see every turn and the previous plan, got %v / %+v", planner.turns[1], planner.previous[1])
	}
}

func TestQuestionAboutAListingSendsTheShownListingsToTheWriter(t *testing.T) {
	asked := palermoPlan("relevance")
	asked.Intent = "ask_about_listing"
	w := &fakeWriter{}
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance"), asked}}, fourStates(), w))
	first := turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"propietaria"}})
	second := turn(t, agent, "¿el primero tiene balcón?", eligibility.Qualification{"guarantee": {"propietaria"}})
	if w.packets[1].Intent != "ask_about_listing" || w.packets[1].Question != "¿el primero tiene balcón?" || urls(second)[0] != urls(first)[0] {
		t.Fatalf("got %+v", w.packets[1])
	}
}
