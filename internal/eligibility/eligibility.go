// Package eligibility decides whether a searcher can rent a property: one of
// four states, from the property's eligibility requirements and the searcher's
// qualification. It is pure and deterministic (docs/adr/0001).
//
// Fact names and instruments are data, never enums (idea.md §8): "guarantee"
// and "income_band" are strings the rules and the qualification agree on.
package eligibility

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

type State string

const (
	Eligible              State = "eligible"
	ConditionallyEligible State = "conditionally_eligible"
	Unknown               State = "unknown"
	Ineligible            State = "ineligible"
)

// Operator is how a rule compares the declared values with its own.
type Operator string

const (
	OneOf          Operator = "one_of"
	SubsetOf       Operator = "subset_of"
	IncomeMultiple Operator = "income_multiple"
	// IncomeMultipleWithExpenses applies the multiple to rent plus expensas.
	IncomeMultipleWithExpenses Operator = "income_multiple_with_expenses"
)

// Hardness says whether a rule is final or the owner decides case by case.
type Hardness string

const (
	Hard          Hardness = "hard"
	Discretionary Hardness = "discretionary"
)

// Reason is what one rule concluded about the searcher.
type Reason string

const (
	Met               Reason = "met"
	NotMet            Reason = "not_met"
	Missing           Reason = "missing"
	Unverifiable      Reason = "unverifiable"
	DiscretionaryCall Reason = "discretionary"
	NearLine          Reason = "near_line"
)

// Qualification is what the searcher declared: fact name -> values.
type Qualification map[string][]string

// Catalog is every fact rules and qualifications may name, by fact name
// (the eligibility_facts table).
type Catalog map[string]Fact

// Fact is one catalog entry. Protected characteristics are facts that are
// not admissible.
type Fact struct {
	Admissible bool
	// Label is the form's question, in Spanish because the searcher reads it.
	Label    string
	Priority string // high | medium | low
	// Multiple facts take several values; the rest take one.
	Multiple bool
	// Position orders the form.
	Position int
	// Choices are the values the qualification form offers.
	Choices []Choice
}

// Choice is one value a fact accepts and the label the form shows for it.
type Choice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// QualificationError is why a declared qualification was refused. Code is
// stable for clients: unknown_fact, inadmissible_fact, invalid_value or
// too_many_values.
type QualificationError struct {
	Code string
	Fact string
}

func (e *QualificationError) Error() string {
	return "eligibility: " + e.Code + " on qualification fact " + strconv.Quote(e.Fact)
}

// retired are facts the form no longer asks: caucion_quoted left with spec
// 007, since no rule reads it.
var retired = []string{"caucion_quoted"}

// DropRetired removes retired facts from q in place, so a saved profile or an
// open tab that still sends one is not refused for it.
func DropRetired(q Qualification) {
	for _, fact := range retired {
		delete(q, fact)
	}
}

// ValidateQualification checks a declared qualification against the catalog
// and returns the first problem, in fact-name order so the same input always
// gets the same answer. An empty qualification is valid.
func ValidateQualification(c Catalog, q Qualification) error {
	facts := slices.Sorted(maps.Keys(q))
	for _, name := range facts {
		fact, ok := c[name]
		switch {
		case !ok:
			return &QualificationError{Code: "unknown_fact", Fact: name}
		case !fact.Admissible:
			return &QualificationError{Code: "inadmissible_fact", Fact: name}
		case !fact.Multiple && len(q[name]) > 1:
			return &QualificationError{Code: "too_many_values", Fact: name}
		// "none" says the searcher has none of the others, so it stands alone:
		// "no pets" next to "dog" would clear a refuse-pets rule for a dog owner.
		case len(q[name]) > 1 && slices.Contains(q[name], "none"):
			return &QualificationError{Code: "invalid_value", Fact: name}
		}
		for _, value := range q[name] {
			if !slices.ContainsFunc(fact.Choices, func(c Choice) bool { return c.Value == value }) {
				return &QualificationError{Code: "invalid_value", Fact: name}
			}
		}
	}
	return nil
}

// Offers reports whether the catalog lists value among the choices of fact.
func (c Catalog) Offers(fact, value string) bool {
	return slices.ContainsFunc(c[fact].Choices, func(choice Choice) bool { return choice.Value == value })
}

// ChoiceLabel is the label the catalog gives value of fact, or "" when the
// catalog does not offer it.
func (c Catalog) ChoiceLabel(fact, value string) string {
	for _, choice := range c[fact].Choices {
		if choice.Value == value {
			return choice.Label
		}
	}
	return ""
}

// Admissible reports whether rules and qualifications may reference fact.
func (c Catalog) Admissible(fact string) bool { return c[fact].Admissible }

// Rule is one eligibility requirement of a property.
type Rule struct {
	Fact       string   `json:"fact"`
	Operator   Operator `json:"operator"`
	Values     []string `json:"values"`
	Hardness   Hardness `json:"hardness"`
	Visibility string   `json:"visibility,omitempty"` // public (default) | private
	Source     string   `json:"source,omitempty"`     // parsed | declared | observed
	Evidence   string   `json:"evidence,omitempty"`   // the listing's own words
}

// animal is a pets word, whole: "obligatorio" holds "gato" and is not one.
const animal = `\b(?:mascotas?|perr[oa]s?|gat[oa]s?|animal(?:es)?)\b`

var (
	namesAnimal = regexp.MustCompile(`(?i)` + animal)
	namesDog    = regexp.MustCompile(`(?i)\bperr[oa]s?\b`)
	namesCat    = regexp.MustCompile(`(?i)\bgat[oa]s?\b`)
	// Only the animals named up to the next clause: "sólo gatos", "únicamente
	// perros y gatos"; in "sólo gatos, no perros" the dogs are outside it.
	onlyAnimals = regexp.MustCompile(`(?i)(?:s[oó]lo|solamente|[uú]nicamente)\b[^.,;]{0,30}`)
	// The owner keeps the call: a preference, or a limit on the animal's size.
	ownerDecides = regexp.MustCompile(`(?i)\bpreferentemente\b|` + animal + `\s+(?:\S+\s+)?(?:grandes?|pequeñ[oa]s?|chic[oa]s?)\b`)
)

// PublishedRules are the rules a listing's own words state, so loading derives
// them without a model: its pets rules, and its request for proof of income
// (incomeProofRule). The rule lists the pets
// choices the listing admits, and every declared pet must be among them
// (subset_of): a refusal admits every choice but the animals it names, or
// only "none" when it names no species; "sólo gatos" admits only cats. A
// preference or a size limit, refusing or welcoming, leaves the owner to
// decide, so that rule is discretionary and admits only "none" outright. An
// inferred attribute is the parser's reading, not the listing's, and derives
// nothing; so does evidence that never names an animal, since the committed
// parse holds two refusals that quote accessibility notes instead.
//
// The parser can also leave pets_allowed unset on a listing whose description
// refuses pets ("sin mascota"). Without a stated attribute that names an
// animal, an explicit refusal in the description is read as one, and the
// refusing clause is the evidence.
func PublishedRules(item listing.Listing, catalog Catalog) []Rule {
	var stated []listing.Attribute
	for _, a := range item.Attributes {
		if a.Type == "pets_allowed" && a.Provenance == listing.Stated && namesAnimal.MatchString(a.Evidence) {
			stated = append(stated, a)
		}
	}
	if len(stated) == 0 {
		if clause := refusesPets.FindString(item.Description); clause != "" {
			stated = []listing.Attribute{{Type: "pets_allowed", Value: "no", Provenance: listing.Stated, Evidence: strings.TrimSpace(clause)}}
		}
	}
	var rules []Rule
	for _, a := range stated {
		if r, ok := petsRule(a, catalog); ok {
			rules = append(rules, r)
		}
	}
	if r, ok := incomeProofRule(item.Description, catalog); ok {
		rules = append(rules, r)
	}
	return rules
}

// incomeProof is a request to prove the tenant's income: "ingresos
// comprobables", "demostración de ingresos en blanco", the tenant's "recibos
// de sueldo" (specs/007-income-proof-rule).
var incomeProof = regexp.MustCompile(`(?i)ingresos?\s+(?:\p{L}+\s+){0,2}?(?:comprobables?|demostrables?|justificables?|verificables?|en\s+blanco)|(?:demostraci[oó]n|justificaci[oó]n|comprobaci[oó]n|comprobantes?)\s+de\s+ingresos?|(?:demostrar|acreditar|justificar|comprobar)\s+(?:sus\s+|tus\s+|los\s+)?ingresos?|\brecibos?\s+de\s+(?:sueldo|haberes|ingresos)`)

// payslip is the cue that is a guarantee instead when a guarantee leads its
// sentence: "Garantía: Propietaria, recibo de sueldo o seguro de caución".
var payslip = regexp.MustCompile(`(?i)^recibos?`)

// orInstead is an alternative right after the request that is not proof of
// income: "ingresos comprobables o seguro de caución", "ingresos/estudios".
// One rule cannot say "or" across facts, so such a listing gets none.
var orInstead = regexp.MustCompile(`(?i)^\s*(?:/|\bo\b)\s*(?:\p{L}+\s+){0,2}?(?:seguro|cauci|garant|estudi)`)

var namesGuarantee = regexp.MustCompile(`(?i)garant`)

// incomeProofRule reads the first clause of a description that asks the
// tenant to prove their income into a hard income_documented rule.
func incomeProofRule(description string, catalog Catalog) (Rule, bool) {
	if !catalog.Admissible("income_documented") || !catalog.Offers("income_documented", "yes") {
		return Rule{}, false
	}
	evidence := ""
	for _, at := range incomeProof.FindAllStringIndex(description, -1) {
		start, end := sentence(description, at[0])
		// Pay slips after a guarantee are the guarantee, not a request.
		if payslip.MatchString(description[at[0]:at[1]]) && namesGuarantee.MatchString(description[start:at[0]]) {
			continue
		}
		// An alternative anywhere means the listing does not require the proof.
		if orInstead.MatchString(description[at[1]:]) {
			return Rule{}, false
		}
		if evidence == "" {
			// The clause starts after a label such as "Requisitos:".
			from := start + strings.LastIndex(description[start:at[0]], ":") + 1
			evidence = strings.TrimFunc(description[from:end], func(r rune) bool {
				return unicode.IsSpace(r) || strings.ContainsRune("-•~*", r)
			})
		}
	}
	if evidence == "" {
		return Rule{}, false
	}
	return Rule{Fact: "income_documented", Operator: OneOf, Values: []string{"yes"}, Hardness: Hard, Visibility: "public", Source: "parsed", Evidence: evidence}, true
}

// sentence is the span around i up to a sentence end: a newline, a semicolon
// or a period that is not inside an amount ("$ 2.000.000").
func sentence(text string, i int) (int, int) {
	ends := func(j int) bool {
		switch text[j] {
		case '\n', ';':
			return true
		case '.':
			return !(j > 0 && j+1 < len(text) && isDigit(text[j-1]) && isDigit(text[j+1]))
		}
		return false
	}
	start := i
	for start > 0 && !ends(start-1) {
		start--
	}
	end := i
	for end < len(text) && !ends(end) {
		end++
	}
	return start, end
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }

// refusesPets is an explicit refusal in a description, up to the end of its
// clause so a size limit or "sólo gatos" after it still counts: "sin
// mascota", "no se aceptan gatos", "no apto mascotas grandes".
var refusesPets = regexp.MustCompile(`(?i)\b(?:sin|no\s+(?:se\s+)?(?:aceptan?|permiten?|admiten?)|no\s+(?:es\s+)?apt[oa]\s+(?:para\s+)?|prohibid[oa]s?\s+(?:las\s+|los\s+)?)\s*` + animal + `[^.;:\n]*`)

// petsRule reads one stated pets_allowed attribute into its rule.
func petsRule(a listing.Attribute, catalog Catalog) (Rule, bool) {
	dog, cat := namesDog.MatchString(a.Evidence), namesCat.MatchString(a.Evidence)
	hardness := Hard
	if ownerDecides.MatchString(a.Evidence) {
		hardness = Discretionary
	}
	only := onlyAnimals.FindString(a.Evidence)
	onlyDog, onlyCat := namesDog.MatchString(only), namesCat.MatchString(only)
	var admits func(choice string) bool
	switch {
	case onlyDog || onlyCat:
		admits = func(c string) bool { return c == "none" || c == "dog" && onlyDog || c == "cat" && onlyCat }
	case hardness == Discretionary:
		admits = func(c string) bool { return c == "none" }
	case a.Value != "no":
		return Rule{}, false
	case dog || cat:
		admits = func(c string) bool { return !(c == "dog" && dog || c == "cat" && cat) }
	default:
		admits = func(c string) bool { return c == "none" }
	}
	var admitted []string
	for _, choice := range catalog["pets"].Choices {
		if admits(choice.Value) {
			admitted = append(admitted, choice.Value)
		}
	}
	return Rule{Fact: "pets", Operator: SubsetOf, Values: admitted, Hardness: hardness, Visibility: "public", Source: "parsed", Evidence: a.Evidence}, true
}

// Validate rejects a rule the evaluator cannot read: an operator missing from
// the table or an unknown hardness. Stores call it when reading rules.
func (r Rule) Validate() error {
	if comparison(r.Operator) == nil {
		return fmt.Errorf("unknown eligibility operator %q", r.Operator)
	}
	if r.Hardness != Hard && r.Hardness != Discretionary {
		return fmt.Errorf("unknown eligibility hardness %q", r.Hardness)
	}
	// An empty list admits nobody, so the listing would be hidden from every
	// searcher.
	if (r.Operator == OneOf || r.Operator == SubsetOf) && len(r.Values) == 0 {
		return fmt.Errorf("eligibility rule on %q admits no values", r.Fact)
	}
	return nil
}

// Condition names a rule that kept the verdict from plain "eligible", in the
// property's own words (Rule.Evidence).
type Condition struct {
	Rule   Rule   `json:"rule"`
	Reason Reason `json:"reason"`
	// Threshold is the monthly income (ARS) a near-the-line band was compared
	// with, so a reader can name it without recomputing it.
	Threshold *float64 `json:"threshold,omitempty"`
}

type Verdict struct {
	State      State       `json:"state"`
	Conditions []Condition `json:"conditions,omitempty"`
	// Met are the requirements the searcher clears, so an eligible verdict
	// says why in the property's own words (Rule.Evidence).
	Met []Rule `json:"met,omitempty"`
	// Ignored are rules on facts not marked admissible (never age, nationality…).
	Ignored []Rule `json:"ignored,omitempty"`
}

// Assess evaluates rules against a qualification. Rules on facts that are not
// admissible are ignored. Precedence: a failed hard rule makes the property
// ineligible; otherwise a missing fact makes it unknown; with no published
// rules it is unknown too, because nothing says the searcher clears anything.
func Assess(q Qualification, item listing.Listing, rules []Rule, catalog Catalog) Verdict {
	v := Verdict{State: Unknown}
	met := false
	for _, r := range rules {
		if !catalog.Admissible(r.Fact) {
			v.Ignored = append(v.Ignored, r)
			continue
		}
		reason := outcome(q, item, r)
		switch reason {
		case Met:
			met = true
			v.Met = append(v.Met, r)
		default:
			c := Condition{Rule: r, Reason: reason}
			if reason == NearLine {
				if threshold, ok := incomeThreshold(item, r); ok {
					c.Threshold = &threshold
				}
			}
			v.Conditions = append(v.Conditions, c)
		}
	}
	switch {
	case hasReason(v.Conditions, NotMet):
		v.State = Ineligible
	case hasReason(v.Conditions, Missing), hasReason(v.Conditions, Unverifiable):
		v.State = Unknown
	case hasReason(v.Conditions, DiscretionaryCall), hasReason(v.Conditions, NearLine):
		v.State = ConditionallyEligible
	case met:
		v.State = Eligible
	}
	return v
}

// comparison is the operator table: it returns the function that compares the
// declared values with a rule of that operator, or nil for an operator the
// evaluator does not know. A new operator is a new case here. A switch, not a
// map: matching constant strings skips hashing on every rule, and a map
// measured 6% slower on BenchmarkAssess.
func comparison(op Operator) func(declared []string, item listing.Listing, r Rule) Reason {
	switch op {
	case OneOf:
		return oneOf
	case SubsetOf:
		return subsetOf
	case IncomeMultiple, IncomeMultipleWithExpenses:
		return incomeMultiple
	}
	return nil
}

func outcome(q Qualification, item listing.Listing, r Rule) Reason {
	compare := comparison(r.Operator)
	// Unverifiable even when discretionary or undeclared: neither the owner's
	// discretion nor the searcher's answer makes a rule nobody can read.
	if compare == nil {
		return Unverifiable
	}
	declared := q[r.Fact]
	if len(declared) == 0 {
		return Missing
	}
	reason := compare(declared, item, r)
	// The owner decides a discretionary rule case by case, so neither
	// clearing nor failing it is final. A comparison nobody can make stays
	// unverifiable: discretion cannot vouch for it. A searcher who declared
	// having none of the fact ("no tengo mascotas") is outside what the owner
	// decides about, so clearing the rule is final for them.
	if r.Hardness == Discretionary && reason != Unverifiable && !(reason == Met && slices.Equal(declared, []string{"none"})) {
		return DiscretionaryCall
	}
	return reason
}

func oneOf(declared []string, _ listing.Listing, r Rule) Reason {
	if slices.ContainsFunc(declared, func(v string) bool { return slices.Contains(r.Values, v) }) {
		return Met
	}
	return NotMet
}

// subsetOf needs every declared value admitted: a dog owner who also has a
// cat fails "gatos no".
func subsetOf(declared []string, _ listing.Listing, r Rule) Reason {
	for _, v := range declared {
		if !slices.Contains(r.Values, v) {
			return NotMet
		}
	}
	return Met
}

// incomeMultiple compares a declared band "min-max" (monthly ARS; "min-" is
// open-ended) with the rule's threshold. A band straddling it is the "near the
// line" case of idea.md §4.
func incomeMultiple(declared []string, item listing.Listing, r Rule) Reason {
	threshold, ok := incomeThreshold(item, r)
	lo, hi, cut := strings.Cut(declared[0], "-")
	min, errMin := strconv.ParseFloat(lo, 64)
	if !ok || !cut || errMin != nil {
		return Unverifiable
	}
	if min >= threshold {
		return Met
	}
	if max, err := strconv.ParseFloat(hi, 64); err == nil && max < threshold {
		return NotMet
	}
	return NearLine
}

// incomeThreshold is the monthly income an income rule asks for: multiple ×
// rent, or × rent plus expensas. Income bands are pesos, so a rent or
// expensas in another currency, or missing, has no threshold.
func incomeThreshold(item listing.Listing, r Rule) (float64, bool) {
	if r.Operator != IncomeMultiple && r.Operator != IncomeMultipleWithExpenses {
		return 0, false
	}
	multiple, err := strconv.ParseFloat(firstOr(r.Values, ""), 64)
	if err != nil || item.Price.Amount == nil || item.Price.Currency != "ARS" {
		return 0, false
	}
	base := *item.Price.Amount
	if r.Operator == IncomeMultipleWithExpenses {
		if item.Expenses.Amount == nil || item.Expenses.Currency != "ARS" {
			return 0, false
		}
		base += *item.Expenses.Amount
	}
	return multiple * base, true
}

func firstOr(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return values[0]
}

func hasReason(cs []Condition, reason Reason) bool {
	return slices.ContainsFunc(cs, func(c Condition) bool { return c.Reason == reason })
}

// Candidate is a listing with the eligibility requirements it publishes.
type Candidate struct {
	Listing listing.Listing `json:"listing"`
	Rules   []Rule          `json:"rules,omitempty"`
}

// Relaxation is the zero-results line: declaring Value for Fact would bring
// Count ineligible listings back ("si conseguís caución, vuelven 14").
type Relaxation struct {
	Fact  string `json:"fact"`
	Value string `json:"value"`
	// Label is the catalog's name for Value, so every reader of the line
	// names the instrument the same way.
	Label string `json:"label,omitempty"`
	Count int    `json:"count"`
}

// Relaxations tries, for every ineligible candidate, each guarantee instrument
// its rules accept that the searcher has not declared. Only guarantees: they
// are what a searcher can go and get, while a pets rule would only suggest
// giving the pet up.
func Relaxations(q Qualification, candidates []Candidate, catalog Catalog) []Relaxation {
	counts := map[[2]string]int{}
	for _, c := range candidates {
		if Assess(q, c.Listing, c.Rules, catalog).State != Ineligible {
			continue
		}
		tried := map[[2]string]bool{}
		for _, r := range c.Rules {
			if r.Fact != "guarantee" || r.Operator != OneOf || !catalog.Admissible(r.Fact) {
				continue
			}
			for _, value := range r.Values {
				key := [2]string{r.Fact, value}
				if tried[key] || slices.Contains(q[r.Fact], value) {
					continue
				}
				tried[key] = true
				relaxed := Qualification{}
				for k, v := range q {
					relaxed[k] = v
				}
				relaxed[r.Fact] = append(slices.Clone(q[r.Fact]), value)
				if Assess(relaxed, c.Listing, c.Rules, catalog).State != Ineligible {
					counts[key]++
				}
			}
		}
	}
	out := make([]Relaxation, 0, len(counts))
	for key, n := range counts {
		out = append(out, Relaxation{Fact: key[0], Value: key[1], Label: catalog.ChoiceLabel(key[0], key[1]), Count: n})
	}
	slices.SortFunc(out, func(a, b Relaxation) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Fact+a.Value, b.Fact+b.Value)
	})
	return out
}

// namesExpensas is an income requirement that counts expensas with the rent:
// "tripliquen el valor del alquiler y expensas".
var namesExpensas = regexp.MustCompile(`(?i)\bexpensas\b`)

// IncomeBase reads, from each income rule's own words, whether its multiple
// covers rent plus expensas, wherever rules are built: at load for the
// committed rules and in Extract for new ones.
func IncomeBase(rules []Rule) []Rule {
	for i, r := range rules {
		if r.Operator == IncomeMultiple && namesExpensas.MatchString(r.Evidence) {
			rules[i].Operator = IncomeMultipleWithExpenses
		}
	}
	return rules
}
