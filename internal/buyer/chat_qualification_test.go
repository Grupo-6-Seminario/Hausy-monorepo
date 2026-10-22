package buyer_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

// cannedModel answers every chat call with the same content, standing in for
// the local model's plan JSON.
type cannedModel struct{ content string }

func (m cannedModel) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: m.content}, nil
}

// catalogStock offers the migration 0007 guarantee and pets choices.
type catalogStock struct{ stock }

func (catalogStock) Facts(context.Context) (eligibility.Catalog, error) {
	return eligibility.Catalog{
		"guarantee": {Admissible: true, Multiple: true, Choices: []eligibility.Choice{
			{Value: "propietaria", Label: "Garantía propietaria"}, {Value: "caucion", Label: "Seguro de caución"},
			{Value: "recibos_garante", Label: "Recibos de sueldo de un garante"}, {Value: "none", Label: "No tengo"},
		}},
		"pets": {Admissible: true, Multiple: true, Choices: []eligibility.Choice{
			{Value: "none", Label: "Ninguna"}, {Value: "dog", Label: "Perro"}, {Value: "cat", Label: "Gato"}, {Value: "other", Label: "Otra"},
		}},
	}, nil
}

// Spec 006, error 3: what a searcher says in the chat ends in the same states
// as the same answer in the form. The planner is the real Qwen adapter over a
// model that returns the plan a model would.
func TestTheChatDeclaresWhatTheFormDeclares(t *testing.T) {
	inventory := catalogStock{stock{byHood: map[string][]eligibility.Candidate{"palermo": {
		candidate("caucion-only", "palermo", 700000, []eligibility.Rule{{Fact: "guarantee", Operator: "one_of", Values: []string{"caucion"}, Hardness: "hard"}}),
		candidate("no-pets", "palermo", 700000, []eligibility.Rule{{Fact: "pets", Operator: "subset_of", Values: []string{"none"}, Hardness: "hard"}}),
		candidate("slips-only", "palermo", 700000, []eligibility.Rule{{Fact: "guarantee", Operator: "one_of", Values: []string{"recibos_garante"}, Hardness: "hard"}}),
	}}}}
	plan := func(qualification string) string {
		return `{"intent":"new_search","branches":[{"neighborhoods":["palermo"],"operation":"alquiler"}],"qualification":` + qualification + `}`
	}
	states := func(resp *buyer.TurnResponse) map[string]eligibility.State {
		out := map[string]eligibility.State{}
		for _, l := range resp.Listings {
			out[l.URL] = l.Eligibility.State
		}
		return out
	}
	for _, tc := range []struct {
		message string
		model   string
		form    eligibility.Qualification
	}{
		{"Alquiler en Palermo, tengo un perro", `{"pets":["dog"]}`, eligibility.Qualification{"pets": {"dog"}}},
		{"Alquiler en Palermo, no tengo garantía", `{"guarantee":["none"]}`, eligibility.Qualification{"guarantee": {"none"}}},
		{"Alquiler en Palermo, tengo garante con recibo de sueldo", `{"guarantee":["recibos_garante"]}`, eligibility.Qualification{"guarantee": {"recibos_garante"}}},
	} {
		chat := buyer.NewAgent(intake.Planner{Primary: intake.Qwen{Client: cannedModel{plan(tc.model)}}}, inventory, &fakeWriter{})
		form := buyer.NewAgent(&fakePlanner{plans: []intake.Plan{palermoPlan("relevance")}}, inventory, &fakeWriter{})
		got := states(turn(t, chat, tc.message, nil))
		want := states(turn(t, form, "Alquiler en Palermo", tc.form))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q:\n chat %v\n form %v", tc.message, got, want)
		}
	}
}
