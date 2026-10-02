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
	IncomeMultiple Operator = "income_multiple"
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

var (
	namesAnimal = regexp.MustCompile(`(?i)mascota|perr[oa]|gat[oa]|animal`)
	namesDog    = regexp.MustCompile(`(?i)perr[oa]`)
	namesCat    = regexp.MustCompile(`(?i)gat[oa]`)
	// The owner keeps the call: a preference, or a limit on size.
	ownerDecides = regexp.MustCompile(`(?i)preferentemente|grande`)
)

// PublishedRules are the eligibility rules a listing's own published
// attributes state, so loading derives them without a model. A pets refusal
// admits a searcher without pets and, when it names only dogs or only cats,
// the other animal too ("gatos no" admits a dog). A preference or a size
// limit leaves the owner to decide, so that rule is discretionary. An inferred
// attribute is the parser's reading, not the listing's, and derives nothing;
// so does a refusal whose evidence never names an animal, since the committed
// parse holds two that quote accessibility notes instead.
func PublishedRules(attributes []listing.Attribute) []Rule {
	var rules []Rule
	for _, a := range attributes {
		if a.Type != "pets_allowed" || a.Value != "no" || a.Provenance != listing.Stated || !namesAnimal.MatchString(a.Evidence) {
			continue
		}
		admitted := []string{"none"}
		dog, cat := namesDog.MatchString(a.Evidence), namesCat.MatchString(a.Evidence)
		switch {
		case dog && !cat:
			admitted = append(admitted, "cat")
		case cat && !dog:
			admitted = append(admitted, "dog")
		}
		hardness := Hard
		if ownerDecides.MatchString(a.Evidence) {
			hardness = Discretionary
		}
		rules = append(rules, Rule{Fact: "pets", Operator: OneOf, Values: admitted, Hardness: hardness, Visibility: "public", Source: "parsed", Evidence: a.Evidence})
	}
	return rules
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
	return nil
}

// Condition names a rule that kept the verdict from plain "eligible", in the
// property's own words (Rule.Evidence).
type Condition struct {
	Rule   Rule   `json:"rule"`
	Reason Reason `json:"reason"`
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
func Assess(q Qualification, rent listing.Money, rules []Rule, catalog Catalog) Verdict {
	v := Verdict{State: Unknown}
	met := false
	for _, r := range rules {
		if !catalog.Admissible(r.Fact) {
			v.Ignored = append(v.Ignored, r)
			continue
		}
		reason := outcome(q, rent, r)
		switch reason {
		case Met:
			met = true
			v.Met = append(v.Met, r)
		default:
			v.Conditions = append(v.Conditions, Condition{Rule: r, Reason: reason})
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
func comparison(op Operator) func(declared []string, rent listing.Money, r Rule) Reason {
	switch op {
	case OneOf:
		return oneOf
	case IncomeMultiple:
		return incomeMultiple
	}
	return nil
}

func outcome(q Qualification, rent listing.Money, r Rule) Reason {
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
	reason := compare(declared, rent, r)
	// The owner decides a discretionary rule case by case, so neither
	// clearing nor failing it is final. A comparison nobody can make stays
	// unverifiable: discretion cannot vouch for it.
	if r.Hardness == Discretionary && reason != Unverifiable {
		return DiscretionaryCall
	}
	return reason
}

func oneOf(declared []string, _ listing.Money, r Rule) Reason {
	if slices.ContainsFunc(declared, func(v string) bool { return slices.Contains(r.Values, v) }) {
		return Met
	}
	return NotMet
}

// incomeMultiple compares a declared band "min-max" (monthly ARS; "min-" is
// open-ended) with multiple × rent. A band straddling the threshold is the
// "near the line" case of idea.md §4.
func incomeMultiple(declared []string, rent listing.Money, r Rule) Reason {
	multiple, err := strconv.ParseFloat(firstOr(r.Values, ""), 64)
	lo, hi, ok := strings.Cut(declared[0], "-")
	min, errMin := strconv.ParseFloat(lo, 64)
	// Income bands are pesos; a rent in another currency has no rate to compare.
	if err != nil || !ok || errMin != nil || rent.Amount == nil || rent.Currency != "ARS" {
		return Unverifiable
	}
	threshold := multiple * *rent.Amount
	if min >= threshold {
		return Met
	}
	if max, err := strconv.ParseFloat(hi, 64); err == nil && max < threshold {
		return NotMet
	}
	return NearLine
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
	Count int    `json:"count"`
}

// Relaxations tries, for every ineligible candidate, each guarantee instrument
// its rules accept that the searcher has not declared. Only guarantees: they
// are what a searcher can go and get, while a pets rule would only suggest
// giving the pet up.
func Relaxations(q Qualification, candidates []Candidate, catalog Catalog) []Relaxation {
	counts := map[[2]string]int{}
	for _, c := range candidates {
		if Assess(q, c.Listing.Price, c.Rules, catalog).State != Ineligible {
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
				if Assess(relaxed, c.Listing.Price, c.Rules, catalog).State != Ineligible {
					counts[key]++
				}
			}
		}
	}
	out := make([]Relaxation, 0, len(counts))
	for key, n := range counts {
		out = append(out, Relaxation{Fact: key[0], Value: key[1], Count: n})
	}
	slices.SortFunc(out, func(a, b Relaxation) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Fact+a.Value, b.Fact+b.Value)
	})
	return out
}
