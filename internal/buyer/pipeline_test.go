package buyer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

type unavailableWriter struct{}

func (unavailableWriter) Write(context.Context, buyer.Packet, func(string)) (string, error) {
	return "", errors.New("private model response")
}

func TestBuyerLogsTurnStagesAndFallbackWithoutPrivateContent(t *testing.T) {
	var logs bytes.Buffer
	ctx := logging.WithLogger(context.Background(), slog.New(slog.NewJSONHandler(&logs, nil)).With("request_id", "test-request"))
	plan := palermoPlan("relevance")
	plan.PlannedBy = "fallback"
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, fourStates(), unavailableWriter{})
	response, err := agent.HandleMessage(ctx, "private-session", "private search text", eligibility.Qualification{"guarantee": {"propietaria"}}, buyer.Events{})
	if err != nil || response == nil || response.Reply == "" {
		t.Fatalf("writer fallback should retain a usable reply: %v, %+v", err, response)
	}
	var events = map[string]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("invalid JSON log: %v: %s", err, line)
		}
		events[event["msg"].(string)] = event
	}
	if events["buyer_plan"]["planner"] != "fallback" || events["buyer_candidates"]["count"] != float64(4) || events["buyer_eligibility"]["hidden"] != float64(1) || events["buyer_search"]["shown"] != float64(3) || events["buyer_writer"]["outcome"] != "fallback" {
		t.Fatalf("missing stage outcomes: %v", events)
	}
	if strings.Contains(logs.String(), "private search text") || strings.Contains(logs.String(), "private-session") || strings.Contains(logs.String(), "private model response") {
		t.Fatalf("private data leaked into logs: %s", logs.String())
	}
}

type fakePlanner struct {
	plans    []intake.Plan
	turns    [][]string
	previous []intake.Plan
}

type evidenceClassifier struct{ fail bool }

type batchEvidenceClassifier struct{ calls int }

func (b *batchEvidenceClassifier) Classify(context.Context, matching.Candidate, []matching.Criterion) ([]matching.Assessment, error) {
	return nil, errors.New("single candidate call is too expensive")
}
func (b *batchEvidenceClassifier) ClassifyBatch(_ context.Context, candidates []matching.Candidate, qs []matching.Criterion) ([][]matching.Assessment, error) {
	b.calls++
	out := make([][]matching.Assessment, len(candidates))
	for i, c := range candidates {
		answer, refs := "insufficient_evidence", []string{}
		if c.ID == "explicit" {
			answer, refs = "supported", []string{"description"}
		}
		out[i] = []matching.Assessment{{CriterionID: qs[0].ID, Assessment: answer, EvidenceRefs: refs, Status: "evaluated"}}
	}
	return out, nil
}

func (f evidenceClassifier) Classify(_ context.Context, c matching.Candidate, qs []matching.Criterion) ([]matching.Assessment, error) {
	if f.fail {
		return nil, errors.New("gateway unavailable")
	}
	assessment := "insufficient_evidence"
	refs := []string{}
	if c.ID == "explicit" || c.ID == "z-explicit" {
		assessment, refs = "supported", []string{"description"}
	}
	out := make([]matching.Assessment, len(qs))
	for i, q := range qs {
		out[i] = matching.Assessment{CriterionID: q.ID, Assessment: assessment, EvidenceRefs: refs, Status: "evaluated"}
	}
	return out, nil
}

func TestRequiredLightGroupsEvidenceBeforeHintsAndSurvivesGatewayFailure(t *testing.T) {
	plan := palermoPlan("price_asc")
	plan.Branches[0].RequiredAttributes = []search.AttributeFilter{{Type: "natural_light", Value: "high"}}
	explicit := candidate("explicit", "palermo", 750000, nil)
	explicit.Listing.Description = "Departamento luminoso con luz natural en living y dormitorio."
	hint := candidate("hint", "palermo", 500000, nil, listing.Attribute{Type: "exposure", Value: "frente", Provenance: listing.Stated})
	hint.Listing.Description = "Departamento al frente."
	inv := stock{requireAttributes: true, byHood: map[string][]eligibility.Candidate{"palermo": {hint, explicit}}}
	good := turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, inv, &fakeWriter{}, buyer.WithMatching(evidenceClassifier{})), "luminoso", nil)
	if got := urls(good); len(got) != 2 || got[0] != "explicit" || got[1] != "hint" || good.Listings[0].QualitativeFit != "exact" || good.Listings[1].QualitativeFit != "unconfirmed" {
		t.Fatalf("direct evidence should precede a cheaper hint: %+v", good.Listings)
	}
	unavailable := turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, inv, &fakeWriter{}, buyer.WithMatching(evidenceClassifier{fail: true})), "luminoso", nil)
	if len(unavailable.Listings) != 2 || unavailable.Listings[0].QualitativeFit != "unconfirmed" {
		t.Fatalf("classification outage should preserve candidates as unconfirmed: %+v", unavailable.Listings)
	}
}

func TestBuyerLogsQualitativeAssessmentTime(t *testing.T) {
	var logs bytes.Buffer
	ctx := logging.WithLogger(context.Background(), slog.New(slog.NewJSONHandler(&logs, nil)))
	plan := palermoPlan("relevance")
	plan.Branches[0].RequiredAttributes = []search.AttributeFilter{{Type: "natural_light", Value: "high"}}
	item := candidate("listing-1", "palermo", 700000, nil)
	item.Listing.Description = "Muy luminoso."
	agent := buyer.NewAgent(
		&fakePlanner{plans: []intake.Plan{plan}}, stock{byHood: map[string][]eligibility.Candidate{"palermo": {item}}}, &fakeWriter{},
		buyer.WithMatching(evidenceClassifier{}))

	if _, err := agent.HandleMessage(ctx, "session", "luminoso", nil, buyer.Events{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), `"msg":"buyer_qualitative"`) || !strings.Contains(logs.String(), `"outcome":"success"`) {
		t.Fatalf("qualitative stage missing: %s", logs.String())
	}
}

func TestBuyerAssessesComparableListingsInOneBatch(t *testing.T) {
	plan := palermoPlan("relevance")
	plan.Branches[0].RequiredAttributes = []search.AttributeFilter{{Type: "natural_light", Value: "high"}}
	explicit := candidate("explicit", "palermo", 700000, nil)
	explicit.Listing.Description = "Muy luminoso."
	unknown := candidate("unknown", "palermo", 600000, nil)
	unknown.Listing.Description = "Al frente."
	classifier := &batchEvidenceClassifier{}
	resp := turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, stock{byHood: map[string][]eligibility.Candidate{"palermo": {unknown, explicit}}}, &fakeWriter{}, buyer.WithMatching(classifier)), "luminoso", nil)
	if classifier.calls != 1 || resp.Listings[0].URL != "explicit" || resp.Listings[1].QualitativeFit != "unconfirmed" {
		t.Fatalf("buyer must batch one comparable group: calls=%d listings=%+v", classifier.calls, resp.Listings)
	}
}

func TestPreferredLightRanksExplicitStatementAboveOrientationHint(t *testing.T) {
	plan := palermoPlan("relevance")
	plan.Branches[0].PreferredAttributes = []search.AttributeFilter{{Type: "natural_light", Value: "high"}}
	explicit := candidate("z-explicit", "palermo", 750000, nil)
	explicit.Listing.Description = "Living y dormitorio muy luminosos."
	hint := candidate("a-hint", "palermo", 500000, nil,
		listing.Attribute{Type: "exposure", Value: "frente", Provenance: listing.Stated},
		listing.Attribute{Type: "orientation", Value: "norte", Provenance: listing.Stated})
	hint.Listing.Description = "Departamento al frente, orientado al norte."
	inv := stock{byHood: map[string][]eligibility.Candidate{"palermo": {hint, explicit}}}
	resp := turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, inv, &fakeWriter{}, buyer.WithMatching(evidenceClassifier{})), "idealmente luminoso", nil)
	if got := urls(resp); len(got) != 2 || got[0] != "z-explicit" || resp.Listings[0].QualitativeFit != "" {
		t.Fatalf("preferred direct claim should rank above weak hints without exact grouping: %+v", resp.Listings)
	}
}

func (f *fakePlanner) Plan(_ context.Context, turns []string, previous intake.Plan, _ eligibility.Catalog) (intake.Plan, error) {
	f.turns, f.previous = append(f.turns, turns), append(f.previous, previous)
	p := f.plans[0]
	if len(f.plans) > 1 {
		f.plans = f.plans[1:]
	}
	return p, nil
}

type stock struct {
	byHood            map[string][]eligibility.Candidate
	requireAttributes bool
}

func (f stock) Candidates(_ context.Context, q search.Query) ([]eligibility.Candidate, error) {
	if f.requireAttributes && len(q.RequiredAttributes) > 0 {
		return nil, nil // the stored inventory has no natural_light rows
	}
	var out []eligibility.Candidate
	for _, h := range q.Neighborhoods {
		out = append(out, f.byHood[h]...)
	}
	return out, nil
}

func TestRequiredLightStillShowsStoredProseAsUnconfirmedAlternative(t *testing.T) {
	max := 800000.0
	rooms := 2
	plan := intake.Plan{Intent: "new_search", Branches: []search.Query{{
		Neighborhoods: []string{"palermo"}, Operation: "alquiler", Currency: "ARS", MaxPrice: &max,
		MinRooms: &rooms, MaxRooms: &rooms,
		RequiredAttributes: []search.AttributeFilter{{Type: "natural_light", Value: "high"}},
	}}}
	brightProse := candidate("bright-prose", "palermo", 700000, nil)
	brightProse.Listing.Description = "Departamento muy luminoso, dos ambientes."
	inv := stock{requireAttributes: true, byHood: map[string][]eligibility.Candidate{"palermo": {brightProse}}}
	resp := turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, inv, &fakeWriter{}),
		"Quiero alquilar un dos ambientes en Palermo, luminoso, hasta 800 mil pesos", nil)
	if len(resp.Listings) != 1 || resp.Listings[0].URL != "bright-prose" || resp.Listings[0].QualitativeFit != "unconfirmed" {
		t.Fatalf("missing light attribute must not make stored inventory vanish; got %+v", resp.Listings)
	}
}

// Facts is the catalog's guarantee and income facts, with the labels the
// zero-results line names (migration 0007).
func (stock) Facts(context.Context) (eligibility.Catalog, error) {
	return eligibility.Catalog{
		"guarantee": {Admissible: true, Multiple: true, Choices: []eligibility.Choice{
			{Value: "propietaria", Label: "Garantía propietaria"},
			{Value: "caucion", Label: "Seguro de caución"},
		}},
		"income_band": {Admissible: true},
	}, nil
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
	writer := &fakeWriter{}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, fourStates(), writer)
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
	if len(resp.Relaxations) != 1 || resp.Relaxations[0] != (eligibility.Relaxation{Fact: "guarantee", Value: "caucion", Label: "Seguro de caución", Count: 1}) {
		t.Fatalf("want 'si conseguís caución, vuelve 1', got %+v", resp.Relaxations)
	}
	// The reply learns how many were hidden, so it can say so, and so does the
	// results screen, which has nothing else to show when every match was hidden.
	if len(writer.packets) != 1 || writer.packets[0].Hidden != 1 {
		t.Fatalf("want 1 hidden ineligible listing in the writer packet, got %+v", writer.packets)
	}
	if resp.HiddenIneligible != 1 {
		t.Fatalf("want 1 hidden ineligible listing in the response, got %d", resp.HiddenIneligible)
	}
}

func TestWithinASectionTheUsersSortWinsElseRequirementFit(t *testing.T) {
	inv := stock{byHood: map[string][]eligibility.Candidate{"palermo": {
		candidate("cheap-dark", "palermo", 500000, nil),
		candidate("pricey-bright", "palermo", 900000, nil, bright),
	}}}
	plan := palermoPlan("relevance")
	plan.Branches[0].PreferredAttributes = []search.AttributeFilter{{Type: "natural_light", Value: "high"}}
	byFit := turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, inv, &fakeWriter{}), "luminoso", nil)
	if urls(byFit)[0] != "pricey-bright" {
		t.Fatalf("relevance must follow requirement fit, got %v", urls(byFit))
	}
	plan.Sort = "price_asc"
	byPrice := turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, inv, &fakeWriter{}), "el más barato", nil)
	if urls(byPrice)[0] != "cheap-dark" {
		t.Fatalf("an explicit sort must win within the section, got %v", urls(byPrice))
	}
}

func TestBranchesMergeIntoOneListAndEmptyBranchesAreReported(t *testing.T) {
	plan := intake.Plan{Intent: "new_search", Branches: []search.Query{
		{Neighborhoods: []string{"palermo"}}, {Neighborhoods: []string{"caballito"}},
	}}
	w := &fakeWriter{}
	resp := turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, fourStates(), w), "Palermo o Caballito", eligibility.Qualification{"guarantee": {"propietaria"}})
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
	agent := buyer.NewAgent(planner, fourStates(), &fakeWriter{})
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
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance"), asked}}, fourStates(), w)
	first := turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"propietaria"}})
	second := turn(t, agent, "¿el primero tiene balcón?", eligibility.Qualification{"guarantee": {"propietaria"}})
	if w.packets[1].Intent != "ask_about_listing" || w.packets[1].Question != "¿el primero tiene balcón?" || urls(second)[0] != urls(first)[0] {
		t.Fatalf("got %+v", w.packets[1])
	}
}

// A listing that never mentions a required amenity is shown after the ones
// that state it, marked unconfirmed, never excluded (CONTEXT.md).
func TestRequiredAmenityRanksUnconfirmedListingsLast(t *testing.T) {
	plan := palermoPlan("relevance")
	plan.Branches[0].RequiredAttributes = []search.AttributeFilter{{Type: "amenity", Value: "pileta"}}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, amenityInventory{}, &fakeWriter{})

	resp := turn(t, agent, "Busco en Palermo con pileta", nil)

	var got []string
	for _, r := range resp.Listings {
		got = append(got, r.URL+":"+r.QualitativeFit)
	}
	if want := []string{"both:exact", "pool:exact", "gym:unconfirmed"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Seen live: with amenities out of SQL, the branch count included listings
// that never mention the amenity, and the reply said "92 opciones con pileta".
func TestBranchCountSeparatesUnconfirmedListings(t *testing.T) {
	plan := palermoPlan("relevance")
	plan.Branches[0].RequiredAttributes = []search.AttributeFilter{{Type: "amenity", Value: "pileta"}}
	writer := &fakeWriter{}
	turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, amenityInventory{}, writer), "Busco en Palermo con pileta", nil)
	if got := writer.packets[0].Branches; len(got) != 1 || got[0].Matches != 2 || got[0].Unconfirmed != 1 {
		t.Fatalf("want 2 matches and 1 unconfirmed, got %+v", got)
	}
}

// Seen live 2026-09-26: the searcher picked two amenities and every card listed
// all ten parsed qualities. A card carries only what answers the request.
func TestResultsCarryOnlyTheQualitiesTheSearcherAskedFor(t *testing.T) {
	pool := listing.Attribute{Type: "amenity", Value: "pileta", Provenance: listing.Stated, Evidence: "PILETA"}
	gym := listing.Attribute{Type: "amenity", Value: "gimnasio", Provenance: listing.Stated, Evidence: "GYM"}
	front := listing.Attribute{Type: "exposure", Value: "frente", Provenance: listing.Stated, Evidence: "Frente"}
	balcony := listing.Attribute{Type: "outdoor_space", Value: "balcon", Provenance: listing.Stated, Evidence: "SALIDA A BALCON"}
	security := listing.Attribute{Type: "amenity", Value: "seguridad", Provenance: listing.Stated, Evidence: "SEGURIDAD"}
	inv := stock{byHood: map[string][]eligibility.Candidate{"palermo": {
		candidate("all", "palermo", 700000, nil, security, pool, balcony, gym, front),
		candidate("pool-only", "palermo", 700000, nil, balcony, pool),
	}}}
	plan := palermoPlan("relevance")
	plan.Branches[0].RequiredAttributes = []search.AttributeFilter{{Type: "amenity", Value: "pileta"}, {Type: "amenity", Value: "gimnasio"}}
	plan.Branches[0].PreferredAttributes = []search.AttributeFilter{{Type: "exposure", Value: "frente"}}

	resp := turn(t, buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan}}, inv, &fakeWriter{}), "Dos ambientes en Palermo con pileta y gimnasio, al frente", nil)

	body, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Listings []struct {
			URL     string `json:"url"`
			Matched []struct{ Type, Value, Evidence string }
		}
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	var cards []string
	for _, l := range got.Listings {
		card := l.URL + ":"
		for _, m := range l.Matched {
			card += " " + m.Type + "=" + m.Value + "«" + m.Evidence + "»"
		}
		cards = append(cards, card)
	}
	want := []string{
		"all: amenity=pileta«PILETA» amenity=gimnasio«GYM» exposure=frente«Frente»",
		"pool-only: amenity=pileta«PILETA»",
	}
	if !slices.Equal(cards, want) {
		t.Fatalf("got %q\nwant %q", cards, want)
	}
}

// Seen in review: a turn with nothing to search (a greeting) kept the shown
// listings but dropped the hidden count and the relaxations, so an empty
// screen lost the only lines that explained it.
func TestATurnWithNothingToSearchKeepsWhatEligibilityHid(t *testing.T) {
	greeting := intake.Plan{Intent: "other"}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance"), greeting}}, fourStates(), &fakeWriter{})
	q := eligibility.Qualification{"guarantee": {"propietaria"}}
	first := turn(t, agent, "Alquiler en Palermo", q)
	second := turn(t, agent, "gracias", q)
	if second.HiddenIneligible != first.HiddenIneligible || !slices.Equal(second.Relaxations, first.Relaxations) || len(first.Relaxations) == 0 {
		t.Fatalf("want the first turn's hidden %d and relaxations %+v kept, got %d and %+v",
			first.HiddenIneligible, first.Relaxations, second.HiddenIneligible, second.Relaxations)
	}
}

// Spec 009: when nothing is left to show, the turn carries the listings the
// searcher does not qualify for, fewest unmet requirements first, at most
// ten; the streamed results carry them too.
func TestAnEmptySearchCarriesTheListingsTheSearcherDoesNotQualifyFor(t *testing.T) {
	income := []eligibility.Rule{caucionOnly[0], {Fact: "income_band", Operator: "income_multiple", Values: []string{"3"}, Hardness: "hard", Evidence: "tripliquen el alquiler"}}
	var candidates []eligibility.Candidate
	candidates = append(candidates, candidate("two-unmet", "palermo", 700000, income))
	for i := range 11 {
		candidates = append(candidates, candidate(fmt.Sprintf("one-unmet-%02d", i), "palermo", 500000+float64(i), caucionOnly))
	}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, stock{byHood: map[string][]eligibility.Candidate{"palermo": candidates}}, &fakeWriter{})
	var early buyer.TurnResponse
	resp, err := agent.HandleMessage(context.Background(), "s", "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"propietaria"}, "income_band": {"0-1000000"}}, buyer.Events{
		Results: func(r buyer.TurnResponse) { early = r },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Listings) != 0 || resp.HiddenIneligible != 12 || len(resp.Ineligible) != 10 {
		t.Fatalf("want 0 shown, 12 hidden and 10 offered, got %d, %d, %d", len(resp.Listings), resp.HiddenIneligible, len(resp.Ineligible))
	}
	for _, r := range resp.Ineligible {
		if r.Eligibility == nil || r.Eligibility.State != eligibility.Ineligible {
			t.Fatalf("every offered listing must be ineligible, got %+v", r.Eligibility)
		}
		if r.URL == "two-unmet" {
			t.Fatal("the listing with two unmet requirements must come after the ten with one")
		}
	}
	if len(early.Ineligible) != 10 {
		t.Fatalf("the streamed results must carry them too, got %d", len(early.Ineligible))
	}
}

// Spec 009: a search with any visible result never sends what was hidden.
func TestASearchWithResultsSendsNoIneligibleListings(t *testing.T) {
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, fourStates(), &fakeWriter{})
	resp := turn(t, agent, "Alquiler en Palermo", eligibility.Qualification{"guarantee": {"propietaria"}})
	if len(resp.Listings) == 0 || resp.HiddenIneligible == 0 || resp.Ineligible != nil {
		t.Fatalf("want results, a hidden count and no ineligible list, got %d, %d, %v", len(resp.Listings), resp.HiddenIneligible, resp.Ineligible)
	}
}

// Found in review: among listings that miss as much, the offered ones follow
// the search's usual order, which for relevance favours what the searcher
// prefers; and a turn with nothing to search keeps them.
func TestTheOfferedListingsFollowTheSearchOrderAndSurviveAGreeting(t *testing.T) {
	plan := palermoPlan("relevance")
	plan.Branches[0].PreferredAttributes = []search.AttributeFilter{{Type: "natural_light", Value: "high"}}
	inventory := stock{byHood: map[string][]eligibility.Candidate{"palermo": {
		candidate("a-plain", "palermo", 600000, caucionOnly),
		candidate("b-bright", "palermo", 600000, caucionOnly, bright),
	}}}
	agent := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{plan, {Intent: "other"}}}, inventory, &fakeWriter{})
	q := eligibility.Qualification{"guarantee": {"propietaria"}}
	first := turn(t, agent, "Alquiler en Palermo, luminoso", q)
	if len(first.Ineligible) != 2 || first.Ineligible[0].URL != "b-bright" {
		t.Fatalf("want the bright listing first, got %v", ineligibleURLs(first))
	}
	second := turn(t, agent, "gracias", q)
	if !slices.Equal(ineligibleURLs(second), ineligibleURLs(first)) {
		t.Fatalf("a greeting must keep the offered listings, got %v", ineligibleURLs(second))
	}
}

func ineligibleURLs(resp *buyer.TurnResponse) []string {
	var out []string
	for _, r := range resp.Ineligible {
		out = append(out, r.URL)
	}
	return out
}
