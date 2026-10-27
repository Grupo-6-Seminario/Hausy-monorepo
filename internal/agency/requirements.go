package agency

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
)

// declarable is what an agency may declare about its own property: each fact
// with the operators the evaluator reads for it. A refusal (none_of) is left
// out because it never says what the property accepts.
var declarable = map[string][]eligibility.Operator{
	"guarantee":         {eligibility.OneOf},
	"income_band":       {eligibility.IncomeMultiple, eligibility.IncomeMultipleWithExpenses},
	"income_documented": {eligibility.OneOf},
	"pets":              {eligibility.SubsetOf},
}

// PrepareRequirements checks declared requirements against the fact catalog
// and words each one for the searcher, since a declared requirement has no
// listing text to quote. At most one requirement per fact.
func PrepareRequirements(rules []eligibility.Rule, facts eligibility.Catalog) ([]eligibility.Rule, error) {
	prepared := make([]eligibility.Rule, 0, len(rules))
	for _, r := range rules {
		field := "requirements." + r.Fact
		if !slices.Contains(declarable[r.Fact], r.Operator) || !facts.Admissible(r.Fact) {
			return nil, ValidationError{Field: "requirements", Reason: "no es un requisito que se pueda declarar"}
		}
		if slices.ContainsFunc(prepared, func(p eligibility.Rule) bool { return p.Fact == r.Fact }) {
			return nil, ValidationError{Field: field, Reason: "está repetido"}
		}
		// Someone without pets is never refused for them.
		if r.Fact == "pets" && !slices.Contains(r.Values, "none") {
			r.Values = append([]string{"none"}, r.Values...)
		}
		switch {
		case r.Fact == "income_band":
			multiple, err := strconv.ParseFloat(strings.TrimSpace(strings.Join(r.Values, "")), 64)
			if len(r.Values) != 1 || err != nil || math.IsNaN(multiple) || multiple <= 0 || multiple > 10 {
				return nil, ValidationError{Field: field, Reason: "debe ser un número de veces el alquiler"}
			}
			r.Values = []string{strconv.FormatFloat(multiple, 'f', -1, 64)}
		case r.Fact == "income_documented" && !slices.Equal(r.Values, []string{"yes"}),
			r.Fact == "guarantee" && slices.Contains(r.Values, "none"),
			slices.ContainsFunc(r.Values, func(v string) bool { return !facts.Offers(r.Fact, v) }):
			return nil, ValidationError{Field: field, Reason: "no es una opción válida"}
		}
		r.Visibility, r.Source = "public", "declared"
		if err := r.Validate(); err != nil {
			return nil, ValidationError{Field: field, Reason: "no es válido"}
		}
		r.Evidence = describe(r, facts)
		prepared = append(prepared, r)
	}
	return prepared, nil
}

// describe words a declared requirement the way the result card quotes a
// listing: "Acepta garantía propietaria o seguro de caución".
func describe(r eligibility.Rule, facts eligibility.Catalog) string {
	var labels []string
	for _, v := range r.Values {
		switch {
		case v == "none":
		case r.Fact == "pets" && v == "other":
			labels = append(labels, "otras mascotas")
		default:
			labels = append(labels, lowerFirst(facts.ChoiceLabel(r.Fact, v)))
		}
	}
	switch r.Fact {
	case "guarantee":
		return "Acepta " + join(labels, " o ")
	case "income_band":
		text := "Ingresos de " + strings.ReplaceAll(r.Values[0], ".", ",") + " veces el alquiler"
		if r.Operator == eligibility.IncomeMultipleWithExpenses {
			text += " más expensas"
		}
		return text
	case "income_documented":
		return "Ingresos comprobables"
	case "pets":
		if len(labels) == 0 {
			return "No admite mascotas"
		}
		return "Admite " + join(labels, " y ")
	}
	return r.Fact
}

// join lists items the Spanish way: "a", "a o b", "a, b o c".
func join(items []string, last string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + last + items[len(items)-1]
}

func lowerFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}
