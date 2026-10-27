//go:build formal

package eligibility_test

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

// TestFormalEligibilityVerdicts checks eight promises of Assess and
// Relaxations against every input inside a bound, not a handful of examples.
// The bound and what it leaves out are in .claude/skills/eligibility-verdicts.
// Each promise is written from CONTEXT.md, ADR 0001 and specs 004 and 008,
// not from the code: a failure is a counterexample, told in the code's terms.
func TestFormalEligibilityVerdicts(t *testing.T) {
	failures := map[string]string{}
	fail := func(property, story string) {
		if _, seen := failures[property]; !seen {
			failures[property] = story
		}
	}
	checked := 0
	for _, item := range boundedListings() {
		for _, rules := range boundedRuleLists() {
			var facts []string
			for _, r := range rules {
				if formalCatalog.Admissible(r.Fact) && !slices.Contains(facts, r.Fact) {
					facts = append(facts, r.Fact)
				}
			}
			for _, q := range boundedQualifications(t, facts) {
				checkVerdict(q, item, rules, fail)
				checked++
			}
		}
	}
	for _, property := range slices.Sorted(maps.Keys(failures)) {
		t.Errorf("%s\n%s", property, failures[property])
	}
	t.Logf("checked %d (qualification, listing, rules) inputs", checked)
}

// formalCatalog is the eligibility_facts table after migrations 0004, 0007
// and 0008, with age standing for every inadmissible fact.
var formalCatalog = eligibility.Catalog{
	"guarantee":         {Admissible: true, Multiple: true, Choices: choices("propietaria", "caucion", "recibos_garante", "none")},
	"income_documented": {Admissible: true, Choices: choices("yes", "no")},
	"pets":              {Admissible: true, Multiple: true, Choices: choices("none", "dog", "cat", "other")},
	"income_band":       {Admissible: true, Choices: choices("0-1000000", "1000000-2000000", "2000000-3000000", "3000000-")},
	"age":               {Admissible: false},
}

// instruments are the guarantee values a rule can name (eligibility.Instruments).
var instruments = []string{"propietaria", "caucion", "recibos_garante"}

func choices(values ...string) []eligibility.Choice {
	out := make([]eligibility.Choice, len(values))
	for i, v := range values {
		out[i] = eligibility.Choice{Value: v}
	}
	return out
}

// subsets are the non-empty subsets of values, in a fixed order.
func subsets(values []string) [][]string {
	var out [][]string
	for mask := 1; mask < 1<<len(values); mask++ {
		var s []string
		for i, v := range values {
			if mask&(1<<i) != 0 {
				s = append(s, v)
			}
		}
		out = append(out, s)
	}
	return out
}

// boundedQualifications is every qualification ValidateQualification accepts
// under formalCatalog that declares nothing beyond facts, each of those
// undeclared or declared. Assess reads a qualification only at the facts its
// rules name (outcome's q[r.Fact]), so the others stay undeclared: all four
// facts would be 1215 qualifications per input and a 40-second run.
func boundedQualifications(t *testing.T, facts []string) []eligibility.Qualification {
	out := []eligibility.Qualification{{}}
	for _, fact := range facts {
		var values []string
		for _, c := range formalCatalog[fact].Choices {
			values = append(values, c.Value)
		}
		var next []eligibility.Qualification
		for _, q := range out {
			next = append(next, q)
			for _, s := range subsets(values) {
				if (formalCatalog[fact].Multiple || len(s) == 1) && (len(s) == 1 || !slices.Contains(s, "none")) {
					declared := maps.Clone(q)
					declared[fact] = s
					next = append(next, declared)
				}
			}
		}
		out = next
	}
	for _, q := range out {
		if err := eligibility.ValidateQualification(formalCatalog, q); err != nil {
			t.Fatalf("the bound generated an invalid qualification %v: %v", q, err)
		}
	}
	return out
}

// boundedRules are the rule shapes the loaders produce (Extract,
// PublishedRules), with every value set and both hardnesses, plus one rule
// with an operator the evaluator does not know and one on an inadmissible fact.
func boundedRules() []eligibility.Rule {
	var out []eligibility.Rule
	for _, h := range []eligibility.Hardness{eligibility.Hard, eligibility.Discretionary} {
		for _, v := range subsets(instruments) {
			out = append(out,
				eligibility.Rule{Fact: "guarantee", Operator: eligibility.OneOf, Values: v, Hardness: h},
				eligibility.Rule{Fact: "guarantee", Operator: eligibility.NoneOf, Values: v, Hardness: h})
		}
		for _, v := range subsets([]string{"none", "dog", "cat", "other"}) {
			out = append(out, eligibility.Rule{Fact: "pets", Operator: eligibility.SubsetOf, Values: v, Hardness: h})
		}
		for _, v := range subsets([]string{"yes", "no"}) {
			out = append(out, eligibility.Rule{Fact: "income_documented", Operator: eligibility.OneOf, Values: v, Hardness: h})
		}
		// "2" puts a band edge exactly on the threshold; "x" is unreadable.
		for _, m := range []string{"2", "3", "x"} {
			out = append(out,
				eligibility.Rule{Fact: "income_band", Operator: eligibility.IncomeMultiple, Values: []string{m}, Hardness: h},
				eligibility.Rule{Fact: "income_band", Operator: eligibility.IncomeMultipleWithExpenses, Values: []string{m}, Hardness: h})
		}
	}
	return append(out,
		eligibility.Rule{Fact: "guarantee", Operator: "between", Values: []string{"propietaria"}, Hardness: eligibility.Hard},
		eligibility.Rule{Fact: "age", Operator: eligibility.OneOf, Values: []string{"30"}, Hardness: eligibility.Hard})
}

// boundedRuleLists is no rules, every single rule and every ordered pair.
func boundedRuleLists() [][]eligibility.Rule {
	rules := boundedRules()
	out := [][]eligibility.Rule{nil}
	for _, a := range rules {
		out = append(out, []eligibility.Rule{a})
	}
	for _, a := range rules {
		for _, b := range rules {
			out = append(out, []eligibility.Rule{a, b})
		}
	}
	return out
}

// boundedListings vary only what Assess reads from a listing: rent and
// expensas. With rent 500000 and expensas 200000, a multiple of 3 falls
// inside a band on rent alone (1.5M) and moves to another with expensas (2.1M).
func boundedListings() []listing.Listing {
	money := func(amount float64, currency string) listing.Money {
		return listing.Money{Amount: &amount, Currency: currency}
	}
	return []listing.Listing{
		{URL: "rent ARS 500000, expensas ARS 200000", Price: money(500000, "ARS"), Expenses: money(200000, "ARS")},
		{URL: "rent USD 500, expensas ARS 200000", Price: money(500, "USD"), Expenses: money(200000, "ARS")},
		{URL: "rent ARS 500000, no expensas", Price: money(500000, "ARS")},
		{URL: "no rent published"},
	}
}

func checkVerdict(q eligibility.Qualification, item listing.Listing, rules []eligibility.Rule, fail func(property, story string)) {
	v := eligibility.Assess(q, item, rules, formalCatalog)
	story := func(format string, args ...any) string {
		return fmt.Sprintf("  searcher: %v\n  listing:  %s\n  rules:    %s\n  verdict:  %s\n  %s",
			q, item.URL, describe(rules), v.State, fmt.Sprintf(format, args...))
	}
	var admissible []eligibility.Rule
	for _, r := range rules {
		if formalCatalog.Admissible(r.Fact) {
			admissible = append(admissible, r)
		}
	}

	// 1. A rule on an inadmissible fact (age, nationality) never changes the verdict.
	if len(admissible) < len(rules) {
		without := eligibility.Assess(q, item, admissible, formalCatalog)
		if summary(without) != summary(v) {
			fail("1. a rule on an inadmissible fact changed the verdict", story("without it: %s", without.State))
		}
	}

	// 2. A fact the searcher never declared is never "Calificás" or
	// "Calificás con condiciones"; a listing that publishes nothing is "A confirmar".
	for _, r := range admissible {
		if r.Operator != "between" && len(q[r.Fact]) == 0 && (v.State == eligibility.Eligible || v.State == eligibility.ConditionallyEligible) {
			fail("2. an undeclared fact did not leave the verdict unknown or ineligible", story("undeclared: %s", r.Fact))
		}
	}
	if len(admissible) == 0 && v.State != eligibility.Unknown {
		fail("2. a listing with no admissible rules was not unknown", story(""))
	}

	// 3. "No califica" exactly when the searcher fails a hard rule.
	failsHard := slices.ContainsFunc(admissible, func(r eligibility.Rule) bool {
		return r.Hardness == eligibility.Hard && fails(q, item, r)
	})
	if failsHard != (v.State == eligibility.Ineligible) {
		fail("3. ineligible does not match failing a hard rule", story("fails a hard rule: %v", failsHard))
	}

	// 4. "Calificás" needs every rule cleared, at least one of them by a rule
	// that accepts something: a refusal never confirms a guarantee alone. A
	// refusal that does not name all the searcher's values is settled by any
	// other cleared rule, whatever its fact (decided 2026-10-26).
	if v.State == eligibility.Eligible {
		cleared := len(v.Conditions) == 0 && len(v.Met) > 0
		for _, m := range v.Met {
			cleared = cleared && m.Operator != eligibility.NoneOf
		}
		for _, r := range admissible {
			if r.Operator == eligibility.NoneOf {
				cleared = cleared && len(q[r.Fact]) > 0 && !fails(q, item, r)
			} else {
				cleared = cleared && slices.ContainsFunc(v.Met, func(m eligibility.Rule) bool { return sameRule(m, r) })
			}
		}
		if !cleared {
			fail("4. eligible without clearing every rule", story("verdict in full: %s", summary(v)))
		}
	}

	// 8. A searcher who clears every rule, all of them hard, sees "Calificás":
	// each rule is accepted outright, or is a refusal that does not refuse
	// everything they declared, and at least one accepts.
	settled := len(admissible) > 0
	accepts := false
	for _, r := range admissible {
		accepts = accepts || clears(q, item, r)
		settled = settled && r.Hardness == eligibility.Hard && len(q[r.Fact]) > 0 &&
			(clears(q, item, r) || r.Operator == eligibility.NoneOf && !fails(q, item, r))
	}
	if settled && accepts && v.State != eligibility.Eligible {
		fail("8. a searcher who clears every hard rule is not eligible", story("verdict in full: %s", summary(v)))
	}

	// 5. The order of the rules never changes the verdict.
	if len(rules) > 1 {
		reversed := slices.Clone(rules)
		slices.Reverse(reversed)
		if other := eligibility.Assess(q, item, reversed, formalCatalog); other.State != v.State {
			fail("5. reordering the rules changed the verdict", story("reversed: %s", other.State))
		}
	}

	// 6. Declaring one more guarantee never hides a listing that was shown.
	if g := q["guarantee"]; len(g) > 0 && !slices.Contains(g, "none") && v.State != eligibility.Ineligible {
		for _, in := range instruments {
			if slices.Contains(g, in) {
				continue
			}
			more := withGuarantee(q, in)
			if after := eligibility.Assess(more, item, rules, formalCatalog); after.State == eligibility.Ineligible {
				fail("6. declaring one more guarantee hid a shown listing", story("adding %s: %s", in, after.State))
			}
		}
	}

	// 7. "Si conseguís X, vuelven N" names exactly the instruments a rule
	// accepts whose declaration (replacing "No tengo") shows a hidden listing.
	var want []string
	if v.State == eligibility.Ineligible {
		for _, in := range instruments {
			accepted := slices.ContainsFunc(admissible, func(r eligibility.Rule) bool {
				return r.Fact == "guarantee" && r.Operator == eligibility.OneOf && slices.Contains(r.Values, in)
			})
			if accepted && !slices.Contains(q["guarantee"], in) &&
				eligibility.Assess(withGuarantee(q, in), item, rules, formalCatalog).State != eligibility.Ineligible {
				want = append(want, "guarantee="+in+"×1")
			}
		}
	}
	var got []string
	for _, r := range eligibility.Relaxations(q, []eligibility.Candidate{{Listing: item, Rules: rules}}, formalCatalog) {
		got = append(got, r.Fact+"="+r.Value+"×"+strconv.Itoa(r.Count))
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		fail("7. the zero-results line suggests the wrong instruments", story("want %v, got %v", want, got))
	}
}

// fails says, from the glossary, whether the searcher's declared values fall
// short of the rule. An undeclared fact or an unreadable rule fails nothing:
// it is unknown.
func fails(q eligibility.Qualification, item listing.Listing, r eligibility.Rule) bool {
	declared := q[r.Fact]
	if len(declared) == 0 {
		return false
	}
	admitted := func(v string) bool { return slices.Contains(r.Values, v) }
	switch r.Operator {
	case eligibility.OneOf:
		return !slices.ContainsFunc(declared, admitted)
	case eligibility.SubsetOf:
		return !allOf(declared, admitted)
	case eligibility.NoneOf:
		return allOf(declared, admitted)
	case eligibility.IncomeMultiple, eligibility.IncomeMultipleWithExpenses:
		threshold, ok := formalThreshold(item, r)
		_, top, _ := strings.Cut(declared[0], "-")
		most, err := strconv.ParseFloat(top, 64)
		return ok && err == nil && most < threshold
	}
	return false
}

// clears says, from the glossary, whether the searcher's declared values
// meet a rule that accepts something. A refusal accepts nothing, so it never
// clears.
func clears(q eligibility.Qualification, item listing.Listing, r eligibility.Rule) bool {
	declared := q[r.Fact]
	if len(declared) == 0 {
		return false
	}
	admitted := func(v string) bool { return slices.Contains(r.Values, v) }
	switch r.Operator {
	case eligibility.OneOf:
		return slices.ContainsFunc(declared, admitted)
	case eligibility.SubsetOf:
		return allOf(declared, admitted)
	case eligibility.IncomeMultiple, eligibility.IncomeMultipleWithExpenses:
		threshold, ok := formalThreshold(item, r)
		bottom, _, _ := strings.Cut(declared[0], "-")
		least, err := strconv.ParseFloat(bottom, 64)
		return ok && err == nil && least >= threshold
	}
	return false
}

// formalThreshold is the monthly ARS income an income rule asks for: the
// multiple times the rent, or the rent plus expensas. Bands are pesos, so a
// rent or expensas in another currency, or missing, has none.
func formalThreshold(item listing.Listing, r eligibility.Rule) (float64, bool) {
	multiple, err := strconv.ParseFloat(r.Values[0], 64)
	if err != nil || item.Price.Amount == nil || item.Price.Currency != "ARS" {
		return 0, false
	}
	base := *item.Price.Amount
	if r.Operator == eligibility.IncomeMultipleWithExpenses {
		if item.Expenses.Amount == nil || item.Expenses.Currency != "ARS" {
			return 0, false
		}
		base += *item.Expenses.Amount
	}
	return multiple * base, true
}

func allOf(values []string, ok func(string) bool) bool {
	for _, v := range values {
		if !ok(v) {
			return false
		}
	}
	return true
}

// withGuarantee is q after the searcher also declares instrument; it replaces
// "No tengo", which stands alone.
func withGuarantee(q eligibility.Qualification, instrument string) eligibility.Qualification {
	out := maps.Clone(q)
	held := slices.DeleteFunc(slices.Clone(q["guarantee"]), func(v string) bool { return v == "none" })
	out["guarantee"] = append(held, instrument)
	return out
}

func sameRule(a, b eligibility.Rule) bool {
	return a.Fact == b.Fact && a.Operator == b.Operator && a.Hardness == b.Hardness && slices.Equal(a.Values, b.Values)
}

func describe(rules []eligibility.Rule) string {
	if len(rules) == 0 {
		return "none"
	}
	parts := make([]string, len(rules))
	for i, r := range rules {
		parts[i] = fmt.Sprintf("%s %s %v (%s)", r.Fact, r.Operator, r.Values, r.Hardness)
	}
	return strings.Join(parts, "; ")
}

// summary is a verdict without its ignored rules, with thresholds as values
// rather than pointers, so two verdicts compare by what a reader sees.
func summary(v eligibility.Verdict) string {
	parts := []string{string(v.State), "met: " + describe(v.Met)}
	for _, c := range v.Conditions {
		threshold := "-"
		if c.Threshold != nil {
			threshold = strconv.FormatFloat(*c.Threshold, 'f', -1, 64)
		}
		parts = append(parts, fmt.Sprintf("%s: %s at %s", c.Reason, describe([]eligibility.Rule{c.Rule}), threshold))
	}
	return strings.Join(parts, " | ")
}
