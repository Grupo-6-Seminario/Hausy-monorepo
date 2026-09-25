package buyer_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/clarification"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/jev"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

type roomClarifier struct{}

func (roomClarifier) Propose(context.Context, []string, intake.Plan, string, eligibility.Qualification) ([]clarification.Question, error) {
	return []clarification.Question{{
		Source: "dos habitaciones", Prompt: "Cuando dijiste «dos habitaciones», ¿a qué te referías?", Kind: "search",
		Choices: []clarification.Choice{
			{ID: "ambientes", Label: "Dos ambientes", Effects: []clarification.Effect{{Field: "rooms_exact", Value: "2"}}},
			{ID: "dormitorios", Label: "Dos dormitorios", Effects: []clarification.Effect{{Field: "min_bedrooms", Value: "2"}}},
		},
	}}, nil
}

type equivalentChoices struct{}

type invalidAmenitySource struct{}

func (invalidAmenitySource) Plan(context.Context, []string) (intake.Plan, error) {
	plan := palermoPlan("relevance")
	plan.Branches[0].RequiredAttributes = []search.AttributeFilter{{Type: "amenity", Value: "any"}}
	return plan, nil
}

type amenityClarifier struct{}

type badAnswerClarifier struct{ roomClarifier }

func (badAnswerClarifier) Normalize(context.Context, clarification.Question, string) ([]clarification.Effect, error) {
	return []clarification.Effect{{Field: "min_bedrooms", Value: "many"}}, nil
}

func (amenityClarifier) Propose(context.Context, []string, intake.Plan, string, eligibility.Qualification) ([]clarification.Question, error) {
	return []clarification.Question{{Source: "con amenities", Prompt: "Cuando dijiste «con amenities», ¿cuáles necesitás?", Kind: "search", Multi: true, Choices: []clarification.Choice{
		{ID: "pileta", Label: "Pileta", Effects: []clarification.Effect{{Field: "required_attribute", Value: "amenity=pileta"}}},
		{ID: "gimnasio", Label: "Gimnasio", Effects: []clarification.Effect{{Field: "required_attribute", Value: "amenity=gimnasio"}}},
	}}}, nil
}

type amenityInventory struct{}

func (amenityInventory) AdmissibleFacts(context.Context) (map[string]bool, error) { return nil, nil }
func (amenityInventory) Candidates(_ context.Context, q search.Query) ([]eligibility.Candidate, error) {
	pool := listing.Attribute{Type: "amenity", Value: "pileta", Provenance: listing.Stated}
	gym := listing.Attribute{Type: "amenity", Value: "gimnasio", Provenance: listing.Stated}
	all := []eligibility.Candidate{
		candidate("pool", "palermo", 700000, nil, pool),
		candidate("gym", "palermo", 700000, nil, gym),
		candidate("both", "palermo", 700000, nil, pool, gym),
	}
	var out []eligibility.Candidate
	for _, c := range all {
		matches := true
		for _, want := range q.RequiredAttributes {
			found := false
			for _, attr := range c.Listing.Attributes {
				if attr.Type == want.Type && attr.Value == want.Value {
					found = true
					break
				}
			}
			if !found {
				matches = false
				break
			}
		}
		if matches {
			out = append(out, c)
		}
	}
	return out, nil
}

func (equivalentChoices) Propose(context.Context, []string, intake.Plan, string, eligibility.Qualification) ([]clarification.Question, error) {
	return []clarification.Question{{Source: "dos habitaciones", Prompt: "¿Cuál?", Kind: "search", Choices: []clarification.Choice{
		{ID: "a", Label: "Opción A", Effects: []clarification.Effect{{Field: "rooms_exact", Value: "2"}}},
		{ID: "b", Label: "Opción B", Effects: []clarification.Effect{{Field: "rooms_exact", Value: "2"}}},
	}}}, nil
}

type roomInventory struct{}

func (roomInventory) AdmissibleFacts(context.Context) (map[string]bool, error) { return nil, nil }

func (roomInventory) Candidates(_ context.Context, q search.Query) ([]eligibility.Candidate, error) {
	all := []eligibility.Candidate{
		candidate("two-rooms", "palermo", 700000, nil),
		candidate("two-bedrooms", "palermo", 800000, nil),
		candidate("three-rooms", "palermo", 750000, nil),
	}
	all[0].Listing.Rooms, all[0].Listing.Bedrooms = intPtr(2), intPtr(1)
	all[1].Listing.Rooms, all[1].Listing.Bedrooms = intPtr(3), intPtr(2)
	all[2].Listing.Rooms, all[2].Listing.Bedrooms = intPtr(3), intPtr(1)
	var out []eligibility.Candidate
	for _, c := range all {
		if q.MinRooms != nil && *c.Listing.Rooms < *q.MinRooms || q.MaxRooms != nil && *c.Listing.Rooms > *q.MaxRooms || q.MinBedrooms != nil && *c.Listing.Bedrooms < *q.MinBedrooms {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func intPtr(n int) *int { return &n }

func TestClarificationGatesSearchAndAppliesTheSelectedTypedEffect(t *testing.T) {
	planner := &fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}
	writer := &fakeWriter{}
	agent := buyer.NewAgent(nil, buyer.WithPipeline(planner, roomInventory{}, writer), buyer.WithClarifier(roomClarifier{}))

	first, err := agent.HandleMessage(context.Background(), "chat", "Busco dos habitaciones en Palermo", nil, buyer.Events{})
	if err != nil || first.Clarification == nil || len(writer.packets) != 0 {
		t.Fatalf("search should wait for an answer: response=%+v err=%v packets=%d", first, err, len(writer.packets))
	}

	second, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Selected: []string{"dormitorios"}}, nil, buyer.Events{})
	if err != nil || second.Clarification != nil || len(writer.packets) != 1 || len(second.Listings) != 1 || second.Listings[0].URL != "two-bedrooms" {
		t.Fatalf("answer should select two bedrooms and then search: response=%+v err=%v packets=%d", second, err, len(writer.packets))
	}
	if len(planner.turns) != 1 {
		t.Fatalf("an option label must not be replanned as a user message: %v", planner.turns)
	}
}

func TestClarificationSkipsQuestionsWhoseAnswersShowTheSameResults(t *testing.T) {
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, roomInventory{}, &fakeWriter{}), buyer.WithClarifier(equivalentChoices{}))
	response, err := agent.HandleMessage(context.Background(), "chat", "Busco dos habitaciones en Palermo", nil, buyer.Events{})
	if err != nil || response.Clarification != nil || len(response.Listings) != 3 {
		t.Fatalf("equivalent answers must not interrupt search: response=%+v err=%v", response, err)
	}
}

func TestConfirmedAnswerSurvivesLaterTurnUntilTheSearcherChangesIt(t *testing.T) {
	three := palermoPlan("relevance")
	three.Intent = "refine"
	three.Branches[0].MinRooms, three.Branches[0].MaxRooms = intPtr(3), intPtr(3)
	planner := &fakePlanner{plans: []intake.Plan{palermoPlan("relevance"), palermoPlan("relevance"), three}}
	agent := buyer.NewAgent(nil, buyer.WithPipeline(planner, roomInventory{}, &fakeWriter{}), buyer.WithClarifier(roomClarifier{}))
	first, err := agent.HandleMessage(context.Background(), "chat", "Busco dos habitaciones en Palermo", nil, buyer.Events{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Selected: []string{"dormitorios"}}, nil, buyer.Events{}); err != nil {
		t.Fatal(err)
	}
	second, err := agent.HandleMessage(context.Background(), "chat", "Ordenalos por relevancia", nil, buyer.Events{})
	if err != nil || second.Clarification != nil || len(second.Listings) != 1 || second.Listings[0].URL != "two-bedrooms" {
		t.Fatalf("unrelated refinement lost confirmed bedrooms: %+v %v", second, err)
	}
	third, err := agent.HandleMessage(context.Background(), "chat", "Ahora quiero tres ambientes", nil, buyer.Events{})
	if err != nil || third.Clarification != nil || len(third.Listings) != 2 || !slices.Contains(urls(third), "three-rooms") {
		t.Fatalf("explicit room change did not replace the confirmation: %+v %v", third, err)
	}
}

func TestUnresolvedRequiredAmenityBecomesAQuestionAndMultipleAnswersAreRequired(t *testing.T) {
	agent := buyer.NewAgent(nil, buyer.WithPipeline(intake.Planner{Primary: invalidAmenitySource{}}, amenityInventory{}, &fakeWriter{}), buyer.WithClarifier(amenityClarifier{}))
	first, err := agent.HandleMessage(context.Background(), "chat", "Busco en Palermo con amenities", nil, buyer.Events{})
	if err != nil || first.Clarification == nil || !first.Clarification.Multi {
		t.Fatalf("invalid amenity plan should ask, not fail: %+v %v", first, err)
	}
	second, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Selected: []string{"pileta", "gimnasio"}}, nil, buyer.Events{})
	if err != nil || second.Clarification != nil || !slices.Equal(confirmed(second), []string{"both"}) {
		t.Fatalf("both required amenities should confirm only the listing with both: %+v %v", second, err)
	}
}

// Captured live (Qwen3.5-9B, 2026-09-25): the planner required every amenity
// for "con amenities", and the proposer's question used the plural field,
// an untyped value and a single choice, so validation rejected it.
type everyAmenitySource struct{}

func (everyAmenitySource) Plan(context.Context, []string) (intake.Plan, error) {
	plan := palermoPlan("relevance")
	for _, value := range []string{"pileta", "gimnasio", "laundry", "coworking", "sum", "seguridad", "parrilla", "ascensor", "cochera", "solarium", "terraza_comun"} {
		plan.Branches[0].RequiredAttributes = append(plan.Branches[0].RequiredAttributes, search.AttributeFilter{Type: "amenity", Value: value})
	}
	return plan, nil
}

type malformedAmenityClarifier struct{}

func (malformedAmenityClarifier) Propose(context.Context, []string, intake.Plan, string, eligibility.Qualification) ([]clarification.Question, error) {
	return []clarification.Question{{Source: "amenities", Prompt: "¿Qué amenidades específicas son obligatorias?", Kind: "search", Fact: "required_attributes", Multi: true, Choices: []clarification.Choice{
		{ID: "pileta", Label: "Pileta", Effects: []clarification.Effect{{Field: "required_attributes", Value: "pileta", Branch: intPtr(0), Clear: []string{"required_attributes"}}}},
	}}}, nil
}

func TestGenericAmenitiesAreAskedEvenWhenTheModelQuestionIsUnusable(t *testing.T) {
	agent := buyer.NewAgent(nil, buyer.WithPipeline(intake.Planner{Primary: everyAmenitySource{}}, amenityInventory{}, &fakeWriter{}), buyer.WithClarifier(malformedAmenityClarifier{}))
	first, err := agent.HandleMessage(context.Background(), "chat", "Busco un dos ambientes en Palermo con amenities", nil, buyer.Events{})
	if err != nil || first.Clarification == nil || first.Clarification.Kind != "search" || !first.Clarification.Multi {
		t.Fatalf("searchable amenities must be asked about, not stopped as unsupported: %+v %v", first.Clarification, err)
	}
	second, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Selected: []string{"pileta"}}, nil, buyer.Events{})
	if err != nil || second.Clarification != nil || !slices.Equal(confirmed(second), []string{"both", "pool"}) {
		t.Fatalf("choosing pileta should require only pileta: %+v %v", second, err)
	}
}

// Captured live: on a later turn the planner re-reads the first message's
// "con amenities" as every amenity again, with no error.
func TestChosenAmenitiesSurviveTheNextTurnsRereading(t *testing.T) {
	agent := buyer.NewAgent(nil, buyer.WithPipeline(intake.Planner{Primary: everyAmenitySource{}}, amenityInventory{}, &fakeWriter{}), buyer.WithClarifier(amenityClarifier{}))
	first, err := agent.HandleMessage(context.Background(), "chat", "Busco en Palermo con amenities", nil, buyer.Events{})
	if err != nil || first.Clarification == nil {
		t.Fatalf("generic amenities should be asked about: %+v %v", first, err)
	}
	if _, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Selected: []string{"pileta"}}, nil, buyer.Events{}); err != nil {
		t.Fatal(err)
	}
	later, err := agent.HandleMessage(context.Background(), "chat", "Ordenalos por precio", nil, buyer.Events{})
	if err != nil || later.Clarification != nil || !slices.Equal(confirmed(later), []string{"both", "pool"}) {
		t.Fatalf("the answer pileta must still be the amenity requirement: %+v %v", later, err)
	}
}

type silentClarifier struct{}

func (silentClarifier) Propose(context.Context, []string, intake.Plan, string, eligibility.Qualification) ([]clarification.Question, error) {
	return nil, nil
}

// Seen live: the planner drops "cerca del trabajo" and the proposer asks
// nothing, so only the known phrase can stop the search.
func TestUnsupportedCommuteIsNamedAndItsRemovalReachesTheReply(t *testing.T) {
	writer := &fakeWriter{}
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, roomInventory{}, writer), buyer.WithClarifier(silentClarifier{}))
	first, err := agent.HandleMessage(context.Background(), "chat", "Necesito alquilar en Palermo cerca del trabajo", nil, buyer.Events{})
	if err != nil || first.Clarification == nil || first.Clarification.Kind != "unsupported" || first.Clarification.Source != "cerca del trabajo" || !strings.Contains(first.Clarification.Prompt, "«cerca del trabajo»") {
		t.Fatalf("the stop should name the unsupported phrase: %+v %v", first.Clarification, err)
	}
	second, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Action: "remove"}, nil, buyer.Events{})
	if err != nil || second.Clarification != nil || len(second.Listings) != 3 {
		t.Fatalf("removing the commute should search the rest: %+v %v", second, err)
	}
	// Only the removal notice quotes the phrase; the searcher's message does not.
	if len(writer.packets) != 1 || !strings.Contains(writer.packets[0].Question, "«cerca del trabajo»") {
		t.Fatalf("the reply must know the searcher removed the condition: %+v", writer.packets)
	}
}

func TestMissingAmenityInventoryExplainsTheLimitAndRequiresExplicitRemoval(t *testing.T) {
	agent := buyer.NewAgent(nil, buyer.WithPipeline(intake.Planner{Primary: invalidAmenitySource{}}, roomInventory{}, &fakeWriter{}), buyer.WithClarifier(amenityClarifier{}))
	first, err := agent.HandleMessage(context.Background(), "chat", "Busco en Palermo con amenities", nil, buyer.Events{})
	if err != nil || first.Clarification == nil || first.Clarification.Kind != "unsupported" || !first.Clarification.CanRemove {
		t.Fatalf("no amenity data should explain the limit and offer removal: %+v %v", first, err)
	}
	second, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Action: "remove"}, nil, buyer.Events{})
	if err != nil || second.Clarification != nil || len(second.Listings) != 3 {
		t.Fatalf("explicit removal should resume the remaining search: %+v %v", second, err)
	}
}

func TestUnclearFreeTextKeepsTheQuestionOpen(t *testing.T) {
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, roomInventory{}, &fakeWriter{}), buyer.WithClarifier(badAnswerClarifier{}))
	first, err := agent.HandleMessage(context.Background(), "chat", "Busco dos habitaciones en Palermo", nil, buyer.Events{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Other: "algo grande"}, nil, buyer.Events{})
	if err != nil || second.Clarification == nil || second.Clarification.ID != first.Clarification.ID {
		t.Fatalf("an unreadable answer should allow retry, not fail the turn: %+v %v", second, err)
	}
}

// confirmed lists the results that state every required amenity; the rest are
// shown after them as unconfirmed.
func confirmed(resp *buyer.TurnResponse) []string {
	var out []string
	for _, r := range resp.Listings {
		if r.QualitativeFit == "exact" {
			out = append(out, r.URL)
		}
	}
	return out
}

// flags stands in for Jev: the probability that the latest message leaves
// each plan field open. Jev only judges; the options come from the schema.
func flags(p map[string]float64) jev.Evaluator {
	return func(_ context.Context, _ any, questions map[string]jev.Question) (map[string]jev.Answer, error) {
		out := map[string]jev.Answer{}
		for field := range questions {
			out[field] = jev.Answer{Type: "boolean", Probability: p[field]}
		}
		return out, nil
	}
}

func TestJudgedRoomAmbiguityAsksWithTypedOptions(t *testing.T) {
	misread := palermoPlan("relevance")
	misread.Branches[0].MinRooms = intPtr(2)
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{misread}}, roomInventory{}, &fakeWriter{}),
		buyer.WithClarifier(clarification.Judge{Evaluate: flags(map[string]float64{"rooms": 0.8})}))
	first, err := agent.HandleMessage(context.Background(), "chat", "Busco 2 cuartos en Palermo", nil, buyer.Events{})
	if err != nil || first.Clarification == nil || first.Clarification.Kind != "search" {
		t.Fatalf("a judged room ambiguity should ask: %+v %v", first, err)
	}
	second, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Selected: []string{"dormitorios"}}, nil, buyer.Events{})
	if err != nil || second.Clarification != nil || !slices.Equal(urls(second), []string{"two-bedrooms"}) {
		t.Fatalf("two dormitorios should replace the two-ambientes reading: %+v %v", second, err)
	}
}

// Seen in experiments/ambiguity: Jev flagged the guarantee in "Tengo garantía
// propietaria"; the plan already carries it, so there is nothing to ask.
func TestJudgedAmbiguityThePlanAlreadyResolvesIsNotAsked(t *testing.T) {
	plan := palermoPlan("relevance")
	plan.Qualification = eligibility.Qualification{"guarantee": {"propietaria"}}
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{plan}}, roomInventory{}, &fakeWriter{}),
		buyer.WithClarifier(clarification.Judge{Evaluate: flags(map[string]float64{"guarantee": 0.65})}))
	resp, err := agent.HandleMessage(context.Background(), "chat", "Busco alquilar en Palermo. Tengo garantía propietaria", nil, buyer.Events{})
	if err != nil || resp.Clarification != nil || len(resp.Listings) != 3 {
		t.Fatalf("a guarantee the plan carries must not be asked: %+v %v", resp.Clarification, err)
	}
}

func TestJudgedGenericAmenitiesAskFromTheVocabulary(t *testing.T) {
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, amenityInventory{}, &fakeWriter{}),
		buyer.WithClarifier(clarification.Judge{Evaluate: flags(map[string]float64{"amenities": 0.85})}))
	first, err := agent.HandleMessage(context.Background(), "chat", "Busco en Palermo, que tenga todos los chiches del edificio", nil, buyer.Events{})
	if err != nil || first.Clarification == nil || !first.Clarification.Multi || len(first.Clarification.Choices) != len(listing.Vocabulary["amenity"]) {
		t.Fatalf("generic amenities should offer every amenity: %+v %v", first.Clarification, err)
	}
	second, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Selected: []string{"pileta"}}, nil, buyer.Events{})
	if err != nil || !slices.Equal(confirmed(second), []string{"both", "pool"}) {
		t.Fatalf("pileta should be the confirmed requirement: %+v %v", second, err)
	}
}

type priceInventory struct{}

func (priceInventory) AdmissibleFacts(context.Context) (map[string]bool, error) { return nil, nil }
func (priceInventory) Candidates(_ context.Context, q search.Query) ([]eligibility.Candidate, error) {
	usd := candidate("usd-850", "palermo", 850, nil)
	usd.Listing.Price.Currency = "USD"
	var out []eligibility.Candidate
	for _, c := range []eligibility.Candidate{candidate("ars-850k", "palermo", 850000, nil), usd} {
		if q.Currency == c.Listing.Price.Currency && (q.MaxPrice == nil || *c.Listing.Price.Amount <= *q.MaxPrice) {
			out = append(out, c)
		}
	}
	return out, nil
}

// Seen in experiments/ambiguity: "puedo pagar hasta 900" reads as pesos or
// dollars; the planner's bare-peso rule alone would pick $900.000.
func TestJudgedCurrencyAmbiguityOffersBothReadingsOfTheSameAmount(t *testing.T) {
	plan := palermoPlan("relevance")
	max := 900000.0
	plan.Branches[0].Currency, plan.Branches[0].MaxPrice = "ARS", &max
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{plan}}, priceInventory{}, &fakeWriter{}),
		buyer.WithClarifier(clarification.Judge{Evaluate: flags(map[string]float64{"currency": 0.91})}))
	first, err := agent.HandleMessage(context.Background(), "chat", "Busco en Palermo, puedo pagar hasta 900", nil, buyer.Events{})
	if err != nil || first.Clarification == nil {
		t.Fatalf("a judged currency ambiguity should ask: %+v %v", first, err)
	}
	var labels []string
	for _, c := range first.Clarification.Choices {
		labels = append(labels, c.Label)
	}
	if !slices.Equal(labels, []string{"Hasta $900.000 por mes", "Hasta USD 900 por mes"}) {
		t.Fatalf("both readings should name the amount: %v", labels)
	}
	second, err := agent.HandleClarification(context.Background(), "chat", buyer.ClarificationAnswer{QuestionID: first.Clarification.ID, Selected: []string{"dolares"}}, nil, buyer.Events{})
	if err != nil || !slices.Equal(urls(second), []string{"usd-850"}) {
		t.Fatalf("dollars should search up to USD 900: %+v %v", second, err)
	}
}

// hoodInventory has amenity data, but only in Palermo, and answers coverage
// per query as the store does.
type hoodInventory struct{}

func (hoodInventory) AdmissibleFacts(context.Context) (map[string]bool, error) { return nil, nil }
func (hoodInventory) Candidates(_ context.Context, q search.Query) ([]eligibility.Candidate, error) {
	if len(q.Neighborhoods) > 0 && !slices.Contains(q.Neighborhoods, "palermo") {
		return nil, nil
	}
	return []eligibility.Candidate{candidate("pool", "palermo", 700000, nil, listing.Attribute{Type: "amenity", Value: "pileta", Provenance: listing.Stated})}, nil
}
func (h hoodInventory) HasAttributeData(ctx context.Context, q search.Query, typ string) (bool, error) {
	found, _ := h.Candidates(ctx, q)
	return len(found) > 0, nil
}

// Seen end to end: a generic amenities request in a barrio with no listings
// stopped as unsupported. Amenities are searchable; that barrio is just empty.
func TestAnEmptyBarrioIsNotAnUnsupportedCondition(t *testing.T) {
	plan := palermoPlan("relevance")
	plan.Branches[0].Neighborhoods = []string{"nunez"}
	agent := buyer.NewAgent(nil, buyer.WithPipeline(&fakePlanner{plans: []intake.Plan{plan}}, hoodInventory{}, &fakeWriter{}),
		buyer.WithClarifier(clarification.Judge{Evaluate: flags(map[string]float64{"amenities": 0.83})}))
	resp, err := agent.HandleMessage(context.Background(), "chat", "Que tenga todos los chiches del edificio, en Núñez", nil, buyer.Events{})
	if err != nil || resp.Clarification != nil || len(resp.Listings) != 0 {
		t.Fatalf("an empty barrio should reach the zero-results flow, not stop: %+v %v", resp.Clarification, err)
	}
}
