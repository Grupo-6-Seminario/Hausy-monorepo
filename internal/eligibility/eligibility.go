// Package eligibility decides whether a searcher can rent a property: one of
// four states, from the property's eligibility requirements and the searcher's
// qualification. It is pure and deterministic (docs/adr/0001).
//
// Fact names and instruments are data, never enums (idea.md §8): "guarantee"
// and "income_band" are strings the rules and the qualification agree on.
package eligibility

import (
	"fmt"
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
	// Choices are the values the qualification form offers.
	Choices []string
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

// Relaxations tries, for every ineligible candidate, each instrument its
// one_of rules accept that the searcher has not declared.
func Relaxations(q Qualification, candidates []Candidate, catalog Catalog) []Relaxation {
	counts := map[[2]string]int{}
	for _, c := range candidates {
		if Assess(q, c.Listing.Price, c.Rules, catalog).State != Ineligible {
			continue
		}
		tried := map[[2]string]bool{}
		for _, r := range c.Rules {
			if r.Operator != OneOf || !catalog.Admissible(r.Fact) {
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
