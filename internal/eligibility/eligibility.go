// Package eligibility decides whether a searcher can rent a property: one of
// four states, from the property's eligibility requirements and the searcher's
// qualification. It is pure and deterministic (docs/adr/0001).
//
// Fact names and instruments are data, never enums (idea.md §8): "guarantee"
// and "income_band" are strings the rules and the qualification agree on.
package eligibility

import (
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

// Qualification is what the searcher declared: fact name -> values.
type Qualification map[string][]string

// Rule is one eligibility requirement of a property.
type Rule struct {
	Fact       string   `json:"fact"`
	Operator   string   `json:"operator"`
	Values     []string `json:"values"`
	Hardness   string   `json:"hardness"`             // hard | discretionary
	Visibility string   `json:"visibility,omitempty"` // public (default) | private
	Source     string   `json:"source,omitempty"`     // parsed | declared | observed
	Evidence   string   `json:"evidence,omitempty"`   // the listing's own words
}

// Condition names a rule that kept the verdict from plain "eligible", in the
// property's own words (Rule.Evidence).
type Condition struct {
	Rule   Rule   `json:"rule"`
	Reason string `json:"reason"` // missing | unverifiable | not_met | discretionary | near_line
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
func Assess(q Qualification, rent listing.Money, rules []Rule, admissible map[string]bool) Verdict {
	v := Verdict{State: Unknown}
	met := false
	for _, r := range rules {
		if !admissible[r.Fact] {
			v.Ignored = append(v.Ignored, r)
			continue
		}
		reason := outcome(q, rent, r)
		// The owner decides a discretionary rule case by case, so neither
		// clearing nor failing it is final.
		if r.Hardness == "discretionary" && reason != "missing" {
			reason = "discretionary"
		}
		switch reason {
		case "met":
			met = true
			v.Met = append(v.Met, r)
		default:
			v.Conditions = append(v.Conditions, Condition{Rule: r, Reason: reason})
		}
	}
	switch {
	case hasReason(v.Conditions, "not_met"):
		v.State = Ineligible
	case hasReason(v.Conditions, "missing"), hasReason(v.Conditions, "unverifiable"):
		v.State = Unknown
	case hasReason(v.Conditions, "discretionary"), hasReason(v.Conditions, "near_line"):
		v.State = ConditionallyEligible
	case met:
		v.State = Eligible
	}
	return v
}

func outcome(q Qualification, rent listing.Money, r Rule) string {
	declared := q[r.Fact]
	if len(declared) == 0 {
		return "missing"
	}
	if r.Operator == "income_multiple" {
		return incomeOutcome(declared[0], rent, r)
	}
	if slices.ContainsFunc(declared, func(v string) bool { return slices.Contains(r.Values, v) }) {
		return "met"
	}
	return "not_met"
}

// incomeOutcome compares a declared band "min-max" (monthly ARS; "min-" is
// open-ended) with multiple × rent. A band straddling the threshold is the
// "near the line" case of idea.md §4.
func incomeOutcome(band string, rent listing.Money, r Rule) string {
	multiple, err := strconv.ParseFloat(firstOr(r.Values, ""), 64)
	lo, hi, ok := strings.Cut(band, "-")
	min, errMin := strconv.ParseFloat(lo, 64)
	// Income bands are pesos; a rent in another currency has no rate to compare.
	if err != nil || !ok || errMin != nil || rent.Amount == nil || rent.Currency != "ARS" {
		return "unverifiable"
	}
	threshold := multiple * *rent.Amount
	if min >= threshold {
		return "met"
	}
	if max, err := strconv.ParseFloat(hi, 64); err == nil && max < threshold {
		return "not_met"
	}
	return "near_line"
}

func firstOr(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return values[0]
}

func hasReason(cs []Condition, reason string) bool {
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
func Relaxations(q Qualification, candidates []Candidate, admissible map[string]bool) []Relaxation {
	counts := map[[2]string]int{}
	for _, c := range candidates {
		if Assess(q, c.Listing.Price, c.Rules, admissible).State != Ineligible {
			continue
		}
		tried := map[[2]string]bool{}
		for _, r := range c.Rules {
			if r.Operator != "one_of" || !admissible[r.Fact] {
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
				if Assess(relaxed, c.Listing.Price, c.Rules, admissible).State != Ineligible {
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
