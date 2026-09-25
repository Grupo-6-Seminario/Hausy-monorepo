// Package clarification turns a model's proposed follow-up into validated,
// typed changes to a search plan. The model never executes a filter itself.
package clarification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

type Effect struct {
	Field  string   `json:"field"`
	Value  string   `json:"value"`
	Branch *int     `json:"branch,omitempty"` // nil applies to every branch
	Clear  []string `json:"clear,omitempty"`  // tentative fields this answer replaces
}

type Choice struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Effects []Effect `json:"effects"`
}

type Question struct {
	ID        string   `json:"id,omitempty"`
	Request   string   `json:"request,omitempty"`
	Source    string   `json:"source"`
	Prompt    string   `json:"prompt"`
	Kind      string   `json:"kind"` // search | qualification | unsupported
	CanRemove bool     `json:"can_remove,omitempty"`
	Fact      string   `json:"fact,omitempty"`
	Multi     bool     `json:"multi,omitempty"`
	Choices   []Choice `json:"choices,omitempty"`
}

type Proposer interface {
	Propose(ctx context.Context, turns []string, plan intake.Plan, problem string, known eligibility.Qualification) ([]Question, error)
}

type Normalizer interface {
	Normalize(ctx context.Context, question Question, answer string) ([]Effect, error)
}

// Model is the generative adapter used with either Jev or a chat-model planner.
type Model struct{ Client llm.Client }

func (m Model) Propose(ctx context.Context, turns []string, plan intake.Plan, problem string, known eligibility.Qualification) ([]Question, error) {
	request, _ := json.Marshal(struct {
		Turns   []string                  `json:"turns"`
		Plan    intake.Plan               `json:"tentative_plan"`
		Problem string                    `json:"plan_problem,omitempty"`
		Known   eligibility.Qualification `json:"known_qualification,omitempty"`
	}{turns, plan, problem, known})
	resp, err := m.Client.Chat(ctx, llm.ChatRequest{Messages: []llm.Message{
		{Role: "system", Content: proposalPrompt},
		{Role: "user", Content: string(request)},
	}, Temperature: 0, MaxTokens: 1100})
	if err != nil {
		return nil, err
	}
	var output struct {
		Questions []Question `json:"questions"`
	}
	if err := json.Unmarshal([]byte(stripFence(resp.Content)), &output); err != nil {
		return nil, err
	}
	return output.Questions, nil
}

func (m Model) Normalize(ctx context.Context, question Question, answer string) ([]Effect, error) {
	request, _ := json.Marshal(map[string]any{"question": question, "answer": answer})
	resp, err := m.Client.Chat(ctx, llm.ChatRequest{Messages: []llm.Message{
		{Role: "system", Content: `Convertí la respuesta libre a efectos tipados para la pregunta. Devolvé sólo JSON {"effects":[{"field":"...","value":"...","branch":0}]}. Usá sólo los field y el branch presentes en las opciones (omití branch si las opciones lo omiten); si no se puede leer sin adivinar, devolvé effects vacío. Para atributos, value es type=value. Para rooms_exact y min_bedrooms, value es un entero positivo. No incluyas clear; la aplicación conserva los campos de reemplazo de la pregunta.`},
		{Role: "user", Content: string(request)},
	}, Temperature: 0, MaxTokens: 250})
	if err != nil {
		return nil, err
	}
	var output struct {
		Effects []Effect `json:"effects"`
	}
	if err := json.Unmarshal([]byte(stripFence(resp.Content)), &output); err != nil {
		return nil, err
	}
	if len(output.Effects) == 0 {
		return nil, errors.New("answer does not resolve the question")
	}
	return output.Effects, nil
}

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "```json"), "```")
	return strings.TrimSpace(strings.TrimSuffix(s, "```"))
}

const proposalPrompt = `Sos el detector de condiciones ambiguas de una búsqueda de alquileres en CABA. El plan adjunto es tentativo: puede omitir condiciones o contener valores inventados. Mirá TODA la conversación, con prioridad al último mensaje; known_qualification ya está declarada y no se vuelve a preguntar. Proponé la menor cantidad de preguntas necesarias antes de buscar. Preguntá sólo si la respuesta puede cambiar filtros, orden o elegibilidad del buscador actual. No preguntes por cualidades que puedan evaluarse con evidencia de avisos, ni por meras explicaciones. No supongas respuestas. Jev no genera opciones: vos sí.
Devolvé sólo JSON {"questions":[{"source":"frase textual del usuario","prompt":"pregunta breve en español que menciona la frase","kind":"search|qualification|unsupported","fact":"guarantee|income_band|caucion_quoted si aplica","multi":false,"choices":[{"id":"identificador","label":"texto","effects":[{"field":"rooms_exact|min_bedrooms|required_attribute|preferred_attribute|excluded_attribute|qualification.guarantee|qualification.income_band|qualification.caucion_quoted","value":"valor","branch":0,"clear":["min_rooms","max_rooms"]}]}]}]}. source debe ser una subcadena literal del mensaje. Cada opción cambia el plan de forma concreta. Para atributos usá value="type=value". branch es opcional, índice de rama desde cero; omitilo cuando aplica a todas. clear es opcional y nombra sólo campos tentativos que interpretaron mal ESTA frase; no borres condiciones independientes. No uses amenity=any. Dos amenities elegidos como requisito son ambos obligatorios (multi=true). Para condición dura que el buscador no soporta, kind=unsupported y choices vacío. Para calificación voluntaria insuficiente, kind=qualification y fact obligatorio. No generes una pregunta por gusto o por una palabra vaga si el resultado no puede cambiar. Si no hace falta, {"questions":[]}.`

var amenityLabels = map[string]string{"pileta": "Pileta", "gimnasio": "Gimnasio", "laundry": "Laundry", "coworking": "Coworking", "sum": "SUM",
	"seguridad": "Seguridad", "parrilla": "Parrilla", "ascensor": "Ascensor", "cochera": "Cochera", "solarium": "Solárium", "terraza_comun": "Terraza común"}

// UnnamedAmenities asks which amenities a generic "amenities" means when the
// tentative plan holds the amenity=any placeholder. The vocabulary is known,
// so this question needs no model and survives a malformed proposal.
func UnnamedAmenities(plan intake.Plan, latest string) (Question, bool) {
	placeholder := false
	for _, b := range plan.Branches {
		placeholder = placeholder || slices.Contains(b.RequiredAttributes, search.AttributeFilter{Type: "amenity", Value: "any"})
	}
	text := strings.ToLower(latest)
	if !placeholder || !strings.Contains(text, "amenities") {
		return Question{}, false
	}
	source := "amenities"
	if strings.Contains(text, "con amenities") {
		source = "con amenities"
	}
	return Question{Source: source, Prompt: "Cuando dijiste «" + source + "», ¿cuáles necesitás sí o sí?", Kind: "search", Multi: true, Choices: amenityChoices()}, true
}

// Validate checks the public question and every possible typed effect. Source
// matching proves provenance, while labeled evaluations judge semantic fit.
func Validate(q Question, turns []string) error {
	if q.Source == "" || q.Prompt == "" || !strings.Contains(strings.ToLower(strings.Join(turns, "\n")), strings.ToLower(q.Source)) {
		return errors.New("question has no cited source")
	}
	if q.Kind == "unsupported" {
		if len(q.Choices) != 0 {
			return errors.New("unsupported condition has choices")
		}
		return nil
	}
	// The cap bounds a question; the widest the schema offers is amenities.
	if q.Kind != "search" && q.Kind != "qualification" || len(q.Choices) < 2 || len(q.Choices) > 12 {
		return errors.New("invalid question shape")
	}
	if q.Kind == "qualification" && !slices.Contains([]string{"guarantee", "income_band", "caucion_quoted"}, q.Fact) {
		return errors.New("unknown qualification fact")
	}
	seen := map[string]bool{}
	for _, c := range q.Choices {
		if c.ID == "" || c.Label == "" || seen[c.ID] || len(c.Effects) == 0 {
			return errors.New("invalid choice")
		}
		seen[c.ID] = true
		for _, effect := range c.Effects {
			if err := ValidateEffect(effect); err != nil {
				return err
			}
			if q.Kind == "qualification" && effect.Field != "qualification."+q.Fact || q.Kind == "search" && strings.HasPrefix(effect.Field, "qualification.") {
				return errors.New("effect does not belong to question kind")
			}
		}
	}
	return nil
}

func ValidateEffect(e Effect) error {
	for _, field := range e.Clear {
		if !slices.Contains([]string{"min_rooms", "max_rooms", "min_bedrooms"}, field) || strings.HasPrefix(e.Field, "qualification.") {
			return errors.New("unsupported field replacement")
		}
	}
	switch e.Field {
	case "rooms_exact", "min_bedrooms":
		n, err := strconv.Atoi(e.Value)
		if err != nil || n < 1 || n > 20 {
			return errors.New("invalid room count")
		}
	case "required_attribute", "preferred_attribute", "excluded_attribute":
		t, v, ok := strings.Cut(e.Value, "=")
		if !ok || !listing.AllowsValue(t, v) {
			return errors.New("unsupported attribute")
		}
	case "currency":
		if !slices.Contains(search.Currencies, e.Value) {
			return errors.New("unsupported currency")
		}
	case "max_price":
		if f, err := strconv.ParseFloat(e.Value, 64); err != nil || f <= 0 {
			return errors.New("invalid price")
		}
	case "qualification.guarantee":
		if !slices.Contains([]string{"propietaria", "caucion"}, e.Value) {
			return errors.New("unsupported guarantee")
		}
	case "qualification.income_band":
		if !slices.Contains([]string{"0-1000000", "1000000-2000000", "2000000-3000000", "3000000-"}, e.Value) {
			return errors.New("unsupported income band")
		}
	case "qualification.caucion_quoted":
		if e.Value != "yes" && e.Value != "no" {
			return errors.New("unsupported quote state")
		}
	default:
		return fmt.Errorf("unsupported effect field %q", e.Field)
	}
	return nil
}

// Apply returns a fresh plan. A selected option replaces a conflicting draft
// value, including an invalid amenity=any placeholder, without replanning text.
func Apply(plan intake.Plan, effects []Effect) (intake.Plan, error) {
	plan.Branches = slices.Clone(plan.Branches)
	plan.Qualification = cloneQualification(plan.Qualification)
	for i := range plan.Branches {
		b := &plan.Branches[i]
		b.RequiredAttributes = slices.Clone(b.RequiredAttributes)
		b.PreferredAttributes = slices.Clone(b.PreferredAttributes)
		b.ExcludedAttributes = slices.Clone(b.ExcludedAttributes)
	}
	for _, e := range effects {
		if err := ValidateEffect(e); err != nil {
			return intake.Plan{}, err
		}
		if e.Branch != nil && (*e.Branch < 0 || *e.Branch >= len(plan.Branches) || strings.HasPrefix(e.Field, "qualification.")) {
			return intake.Plan{}, errors.New("invalid branch target")
		}
		if strings.HasPrefix(e.Field, "qualification.") {
			fact := strings.TrimPrefix(e.Field, "qualification.")
			plan.Qualification[fact] = []string{e.Value}
			continue
		}
		for i := range plan.Branches {
			if e.Branch != nil && i != *e.Branch {
				continue
			}
			b := &plan.Branches[i]
			for _, field := range e.Clear {
				switch field {
				case "min_rooms":
					b.MinRooms = nil
				case "max_rooms":
					b.MaxRooms = nil
				case "min_bedrooms":
					b.MinBedrooms = nil
				}
			}
			switch e.Field {
			case "currency":
				b.Currency = e.Value
			case "max_price":
				f, _ := strconv.ParseFloat(e.Value, 64)
				b.MaxPrice = &f
			case "rooms_exact":
				n, _ := strconv.Atoi(e.Value)
				b.MinRooms, b.MaxRooms = &n, &n
			case "min_bedrooms":
				n, _ := strconv.Atoi(e.Value)
				b.MinBedrooms = &n
			case "required_attribute", "preferred_attribute", "excluded_attribute":
				t, v, _ := strings.Cut(e.Value, "=")
				f := search.AttributeFilter{Type: t, Value: v}
				// Replace invalid placeholders of the same type; keep distinct
				// selected amenities so multi-select means AND.
				b.RequiredAttributes = slices.DeleteFunc(b.RequiredAttributes, func(x search.AttributeFilter) bool { return x.Type == t && !listing.AllowsValue(x.Type, x.Value) })
				b.PreferredAttributes = slices.DeleteFunc(b.PreferredAttributes, func(x search.AttributeFilter) bool { return x.Type == t && !listing.AllowsValue(x.Type, x.Value) })
				b.ExcludedAttributes = slices.DeleteFunc(b.ExcludedAttributes, func(x search.AttributeFilter) bool { return x.Type == t && !listing.AllowsValue(x.Type, x.Value) })
				switch e.Field {
				case "required_attribute":
					if !slices.Contains(b.RequiredAttributes, f) {
						b.RequiredAttributes = append(b.RequiredAttributes, f)
					}
				case "preferred_attribute":
					if !slices.Contains(b.PreferredAttributes, f) {
						b.PreferredAttributes = append(b.PreferredAttributes, f)
					}
				case "excluded_attribute":
					if !slices.Contains(b.ExcludedAttributes, f) {
						b.ExcludedAttributes = append(b.ExcludedAttributes, f)
					}
				}
			}
		}
	}
	return plan, nil
}

func cloneQualification(q eligibility.Qualification) eligibility.Qualification {
	out := eligibility.Qualification{}
	for k, values := range q {
		out[k] = slices.Clone(values)
	}
	return out
}
