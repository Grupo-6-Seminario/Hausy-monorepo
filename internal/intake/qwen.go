package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

// Qwen fills the plan with one local-model call: no tools, JSON out. It is the
// default planner and Jev's fallback.
type Qwen struct{ Client llm.Client }

func (q Qwen) Plan(ctx context.Context, turns []string) (Plan, error) {
	var user strings.Builder
	for i, t := range turns {
		fmt.Fprintf(&user, "Mensaje %d del usuario: %s\n", i+1, t)
	}
	resp, err := q.Client.Chat(ctx, llm.ChatRequest{
		Messages:    []llm.Message{{Role: "system", Content: qwenPrompt}, {Role: "user", Content: user.String()}},
		Temperature: 0,
		MaxTokens:   800,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("intake: local model: %w", err)
	}
	content := strings.TrimSpace(resp.Content)
	content = strings.TrimPrefix(strings.TrimPrefix(content, "```json"), "```")
	content = strings.TrimSpace(strings.TrimSuffix(content, "```"))
	var plan Plan
	if err := decodeQwenPlan([]byte(content), &plan); err != nil {
		return Plan{}, fmt.Errorf("intake: local model did not return a plan: %w", err)
	}
	for i := range plan.Branches {
		repair(&plan.Branches[i])
		// Seen live (experiments/clarification, 2026-09-25): a stated guarantee
		// as a required listing attribute. It is the searcher's qualification;
		// declaredOnly below keeps it only if the user said it.
		b := &plan.Branches[i]
		b.RequiredAttributes = slices.DeleteFunc(b.RequiredAttributes, func(f search.AttributeFilter) bool {
			if f.Type != "guarantee" {
				return false
			}
			if plan.Qualification == nil {
				plan.Qualification = eligibility.Qualification{}
			}
			plan.Qualification["guarantee"] = append(plan.Qualification["guarantee"], f.Value)
			return true
		})
	}
	plan.Qualification = declaredOnly(plan.Qualification, strings.Join(turns, "\n"))
	return plan, nil
}

// The local model sometimes writes one declared qualification value as a
// string. Normalize that JSON shape before decoding the typed plan; no fact is
// added or inferred here.
func decodeQwenPlan(content []byte, plan *Plan) error {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		return err
	}
	if raw, ok := document["qualification"]; ok {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		for fact, value := range fields {
			var single string
			if err := json.Unmarshal(value, &single); err == nil {
				wrapped, _ := json.Marshal([]string{single})
				fields[fact] = wrapped
			}
		}
		document["qualification"], _ = json.Marshal(fields)
	}
	normalized, err := json.Marshal(document)
	if err != nil {
		return err
	}
	return json.Unmarshal(normalized, plan)
}

var (
	incomeCue       = regexp.MustCompile(`(?i)\bgan(o|amos)\b|ingreso|sueldo|\bcobr|facturo`)
	incomeAmountCue = regexp.MustCompile(`(?i)\d|mill[oó]n|palo|luca`)
	quoteCue        = regexp.MustCompile(`(?i)cotiz|cotic`)
)

// declaredOnly keeps the qualification facts the user actually stated. The
// model was seen inventing an income band; a declared fact changes eligibility.
func declaredOnly(q eligibility.Qualification, text string) eligibility.Qualification {
	out := eligibility.Qualification{}
	for _, in := range eligibility.Instruments {
		if slices.Contains(q["guarantee"], in.Name) && in.Pattern.MatchString(text) {
			out["guarantee"] = append(out["guarantee"], in.Name)
		}
	}
	if len(q["income_band"]) > 0 && hasDeclaredIncomeAmount(text) {
		out["income_band"] = q["income_band"][:1]
	}
	if len(q["caucion_quoted"]) > 0 && quoteCue.MatchString(text) {
		out["caucion_quoted"] = q["caucion_quoted"][:1]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func hasDeclaredIncomeAmount(text string) bool {
	for _, clause := range strings.FieldsFunc(text, func(r rune) bool { return r == '.' || r == ';' || r == '?' || r == '!' || r == '\n' }) {
		for _, at := range incomeCue.FindAllStringIndex(clause, -1) {
			end := min(len(clause), at[1]+48)
			if incomeAmountCue.MatchString(clause[at[1]:end]) {
				return true
			}
		}
	}
	return false
}

// repair undoes what the local model was seen doing (experiments/intake,
// 2026-09-23): copying every field of the JSON shape with 0, which would reach
// SQL as a real bound, naming an amenity as a type ("cochera"), and inventing
// a value ("amenities" as amenity=any). "sin amenities" came back as excluded
// and preferred amenity=any, a withdrawal, so those are dropped; a required one
// is a demand the query cannot express, and stays for validation to reject
// rather than vanish.
func repair(q *search.Query) {
	for _, f := range []**float64{&q.MinPrice, &q.MaxPrice, &q.MaxExpensesARS, &q.MinTotalAreaM2} {
		if *f != nil && **f == 0 {
			*f = nil
		}
	}
	for _, n := range []**int{&q.MinRooms, &q.MaxRooms, &q.MinBedrooms, &q.MinBathrooms, &q.MinParkingSpaces, &q.MaxAgeYears} {
		if *n != nil && **n == 0 {
			*n = nil
		}
	}
	q.RequiredAttributes = repairFilters(q.RequiredAttributes)
	q.PreferredAttributes = slices.DeleteFunc(repairFilters(q.PreferredAttributes), invented)
	q.ExcludedAttributes = slices.DeleteFunc(repairFilters(q.ExcludedAttributes), invented)
}

func invented(f search.AttributeFilter) bool { return !listing.AllowsValue(f.Type, f.Value) }

func repairFilters(filters []search.AttributeFilter) []search.AttributeFilter {
	var out []search.AttributeFilter
	for _, f := range filters {
		if _, known := listing.Vocabulary[f.Type]; !known {
			for typ, values := range listing.Vocabulary {
				if slices.Contains(values, f.Type) {
					f = search.AttributeFilter{Type: typ, Value: f.Type}
					break
				}
			}
		}
		if f.Type != "" && f.Value != "" {
			out = append(out, f)
		}
	}
	return out
}

var qwenPrompt = `Convertí la conversación de alguien que busca propiedad en CABA en un plan de búsqueda JSON.
Devolvé sólo JSON válido, sin texto ni explicaciones, con esta forma:
{"intent": "new_search|refine|ask_about_listing|other",
 "sort": "relevance|price_asc|price_desc|area_desc",
 "branches": [{"neighborhoods": ["palermo"], "operation": "alquiler|alquiler_temporal|venta",
   "currency": "ARS|USD", "max_price": 800000, "min_rooms": 2,
   "required_attributes": [{"type": "amenity", "value": "cochera"}],
   "preferred_attributes": [{"type": "natural_light", "value": "high"}],
   "excluded_attributes": [{"type": "exposure", "value": "interno"}]}],
 (otros campos numéricos posibles: min_price, max_expenses_ars, max_rooms, min_bedrooms, min_bathrooms, min_total_area_m2)
 "qualification": {"guarantee": ["propietaria|caucion"], "income_band": ["0-1000000|1000000-2000000|2000000-3000000|3000000-"]}}

Reglas:
- Describí la búsqueda que el usuario quiere después del ÚLTIMO mensaje: los mensajes nuevos reemplazan o retiran lo anterior.
- Omití los campos que el usuario no mencionó; nunca pongas 0 en un campo que no se dijo. Barrios en minúscula con guiones bajos ("villa_crespo").
- Todo precio lleva currency. "800 mil", "500 lucas" = pesos; "1 palo" = 1000000.
- required: lo exige ("necesito", "sí o sí", "que tenga"). preferred: lo prefiere o es negociable. excluded: no lo quiere ("sin", "que no sea", "olvidate de X").
- Usá una sola rama con todos los barrios, salvo que el usuario ate requisitos distintos a barrios distintos: ahí, una rama por grupo de barrios.
- qualification: sólo lo que el usuario dice que TIENE (su garantía, sus ingresos mensuales).
- Si el último mensaje pregunta por una propiedad ya mostrada, intent = "ask_about_listing".

Atributos válidos (type: values):
` + vocabularyLines()

func vocabularyLines() string {
	types := make([]string, 0, len(listing.Vocabulary))
	for t := range listing.Vocabulary {
		types = append(types, t)
	}
	sort.Strings(types)
	var b strings.Builder
	for _, t := range types {
		fmt.Fprintf(&b, "- %s: %s\n", t, strings.Join(listing.Vocabulary[t], ", "))
	}
	return b.String()
}
