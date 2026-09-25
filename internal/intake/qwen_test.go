package intake_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

type fakeLLM struct {
	reply string
	last  llm.ChatRequest
}

func (f *fakeLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.last = req
	return &llm.ChatResponse{Content: f.reply}, nil
}

func TestQwenFillsThePlanInOneCall(t *testing.T) {
	f := &fakeLLM{reply: "```json\n" + `{"intent":"refine","sort":"price_asc","branches":[{"neighborhoods":["palermo"],"operation":"alquiler","currency":"ARS","max_price":950000,"preferred_attributes":[{"type":"natural_light","value":"high"}]}],"qualification":{"guarantee":["caucion"]}}` + "\n```"}
	p, err := intake.Qwen{Client: f}.Plan(context.Background(), []string{"Alquiler en Palermo hasta 800 mil", "mejor hasta 950 mil, con buena luz. Tengo caución"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Intent != "refine" || p.Sort != "price_asc" || len(p.Branches) != 1 || *p.Branches[0].MaxPrice != 950000 || p.Qualification["guarantee"][0] != "caucion" {
		t.Fatalf("got %+v", p)
	}
	prompt := f.last.Messages[len(f.last.Messages)-1].Content
	if !strings.Contains(prompt, "hasta 800 mil") || !strings.Contains(prompt, "mejor hasta 950 mil") {
		t.Fatalf("the single call must carry every user turn: %+v", f.last)
	}
}

func TestQwenRejectsAReplyThatIsNotAPlan(t *testing.T) {
	if _, err := (intake.Qwen{Client: &fakeLLM{reply: "Claro, busco en Palermo."}}).Plan(context.Background(), []string{"Palermo"}); err == nil {
		t.Fatal("prose must not pass as a plan")
	}
}

// Seen live (experiments/intake, 2026-09-23): the model copies every field of
// the prompt's JSON shape, zeros included, and writes amenities as types.
func TestQwenTreatsZerosAsUnmentionedAndRepairsAmenityShorthand(t *testing.T) {
	f := &fakeLLM{reply: `{"intent":"new_search","branches":[{"neighborhoods":["monserrat"],"operation":"alquiler","currency":"ARS","min_price":0,"max_price":700000,"max_expenses_ars":0,"min_rooms":2,"max_rooms":0,"min_bedrooms":0,"min_bathrooms":0,"min_total_area_m2":0,"required_attributes":[{"type":"cochera","value":""},{"type":"","value":""}],"excluded_attributes":[{"type":"amenity","value":"pileta"}]}]}`}
	p, err := intake.Planner{Primary: intake.Qwen{Client: f}}.Plan(context.Background(), []string{"2 ambientes en Monserrat hasta 700 mil, con cochera, sin pileta"}, intake.Plan{})
	if err != nil {
		t.Fatal(err)
	}
	b := p.Branches[0]
	if b.MinPrice != nil || b.MaxRooms != nil || b.MaxExpensesARS != nil || b.MinTotalAreaM2 != nil || *b.MinRooms != 2 || *b.MaxPrice != 700000 {
		t.Fatalf("zeros must mean 'not mentioned': %+v", b)
	}
	if len(b.RequiredAttributes) != 1 || b.RequiredAttributes[0] != (search.AttributeFilter{Type: "amenity", Value: "cochera"}) || len(b.ExcludedAttributes) != 1 {
		t.Fatalf("got %+v", b)
	}
}

// Seen live (2026-09-25): "sin amenities" came back as amenity=any, which is
// not a value, and the whole turn failed with a 502.
func TestQwenDropsAnAttributeValueOutsideTheVocabulary(t *testing.T) {
	f := &fakeLLM{reply: `{"intent":"refine","sort":"relevance","branches":[{"neighborhoods":["palermo"],"operation":"alquiler","currency":"ARS","max_price":800000,"min_rooms":2,"required_attributes":[],"preferred_attributes":[{"type":"amenity","value":"any"}],"excluded_attributes":[{"type":"amenity","value":"any"}]}],"qualification":{"guarantee":[],"income_band":[]}}`}
	p, err := intake.Planner{Primary: intake.Qwen{Client: f}}.Plan(context.Background(), []string{"dos ambientes en palermo, con amenities, hasta 800k", "sin amenities"}, intake.Plan{})
	if err != nil {
		t.Fatal(err)
	}
	b := p.Branches[0]
	if b.Neighborhoods[0] != "palermo" || *b.MaxPrice != 800000 || *b.MinRooms != 2 {
		t.Fatalf("the rest of the search must survive: %+v", b)
	}
	if len(b.RequiredAttributes)+len(b.PreferredAttributes)+len(b.ExcludedAttributes) != 0 {
		t.Fatalf("amenity=any must not reach the query: %+v", b)
	}
}

// "con amenities" is a requirement without saying which: dropping it would
// widen the search behind the user's back, so it must reach the caller.
func TestQwenNeverDropsARequirementItCannotRead(t *testing.T) {
	f := &fakeLLM{reply: `{"intent":"new_search","branches":[{"neighborhoods":["palermo"],"operation":"alquiler","currency":"ARS","max_price":800000,"min_rooms":2,"required_attributes":[{"type":"amenity","value":"any"}]}]}`}
	if p, err := (intake.Planner{Primary: intake.Qwen{Client: f}}).Plan(context.Background(), []string{"dos ambientes en palermo, con amenities, hasta 800k"}, intake.Plan{}); err == nil {
		t.Fatalf("a required amenity=any must not be dropped silently, got %+v", p)
	}
}

// Seen live: the model filled income_band "0-1000000" for users who never
// mentioned income. A declared fact changes eligibility, so it must be said.
func TestQwenKeepsOnlyQualificationTheUserActuallyMentioned(t *testing.T) {
	reply := `{"intent":"new_search","branches":[{"neighborhoods":["palermo"]}],"qualification":{"income_band":["0-1000000"],"guarantee":["propietaria"]}}`
	silent, err := intake.Qwen{Client: &fakeLLM{reply: reply}}.Plan(context.Background(), []string{"Busco alquilar en Palermo hasta 800 mil pesos"})
	if err != nil || len(silent.Qualification) != 0 {
		t.Fatalf("nothing was declared, got %v, %v", silent.Qualification, err)
	}
	said, _ := intake.Qwen{Client: &fakeLLM{reply: reply}}.Plan(context.Background(), []string{"Palermo. Tengo garantía propietaria y gano 900 mil"})
	if len(said.Qualification["income_band"]) != 1 || len(said.Qualification["guarantee"]) != 1 {
		t.Fatalf("declared facts must survive, got %v", said.Qualification)
	}
}

// Captured local-model shape: one qualification fact was a JSON string where
// the typed plan expects a list. The value is still explicit in the user's text.
func TestQwenAcceptsASingleDeclaredIncomeBandString(t *testing.T) {
	reply := `{"intent":"new_search","branches":[{"neighborhoods":["palermo"]}],"qualification":{"income_band":"1000000-2000000"}}`
	plan, err := (intake.Qwen{Client: &fakeLLM{reply: reply}}).Plan(context.Background(), []string{"Busco en Palermo. Mis ingresos son entre 1 y 2 millones"})
	if err != nil || len(plan.Qualification["income_band"]) != 1 || plan.Qualification["income_band"][0] != "1000000-2000000" {
		t.Fatalf("a declared single income band should remain usable: %+v %v", plan, err)
	}
}

func TestQwenDoesNotInferAnIncomeBandFromAQualitativeStatement(t *testing.T) {
	reply := `{"intent":"new_search","branches":[{"neighborhoods":["palermo"]}],"qualification":{"income_band":["3000000-"]}}`
	plan, err := (intake.Qwen{Client: &fakeLLM{reply: reply}}).Plan(context.Background(), []string{"Busco hasta 800 mil pesos en Palermo. Tengo ingresos estables"})
	if err != nil || len(plan.Qualification["income_band"]) != 0 {
		t.Fatalf("the rent amount cannot become a declared income band: %+v %v", plan, err)
	}
}

// Captured live (experiments/clarification, 2026-09-25): a stated guarantee
// came back as a required listing attribute, which failed the whole plan.
func TestQwenStatedGuaranteeIsQualificationNotAListingFilter(t *testing.T) {
	reply := `{"intent":"new_search","branches":[{"neighborhoods":["palermo"],"operation":"alquiler","required_attributes":[{"type":"guarantee","value":"propietaria"}]}]}`
	plan, err := intake.Planner{Primary: intake.Qwen{Client: &fakeLLM{reply: reply}}}.Plan(context.Background(), []string{"Busco alquilar en Palermo. Tengo garantía propietaria"}, intake.Plan{})
	if err != nil || len(plan.Branches[0].RequiredAttributes) != 0 || len(plan.Qualification["guarantee"]) != 1 || plan.Qualification["guarantee"][0] != "propietaria" {
		t.Fatalf("a clear search with a stated guarantee must plan: %+v %v", plan, err)
	}
}
