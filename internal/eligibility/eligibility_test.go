package eligibility_test

import (
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

var catalog = eligibility.Catalog{"guarantee": {Admissible: true}, "income_band": {Admissible: true}}

// From a snapshot listing: "Requisitos: | Ingresos comprobables | Garantía propietaria".
var onlyPropietaria = eligibility.Rule{Fact: "guarantee", Operator: "one_of", Values: []string{"propietaria"}, Hardness: "hard", Evidence: "Garantía propietaria"}

func rent(amount float64) listing.Money { return listing.Money{Amount: &amount, Currency: "ARS"} }

// The searcher is told why they can apply, in the listing's own words.
func TestEligibleVerdictNamesTheRequirementTheSearcherClears(t *testing.T) {
	got := eligibility.Assess(eligibility.Qualification{"guarantee": {"propietaria"}}, rent(800000), []eligibility.Rule{onlyPropietaria}, catalog)
	if got.State != eligibility.Eligible || len(got.Met) != 1 || got.Met[0].Evidence != "Garantía propietaria" {
		t.Fatalf("want eligible naming the cleared requirement, got %+v", got)
	}
}

func TestHardGuaranteeTheSearcherLacksIsIneligible(t *testing.T) {
	got := eligibility.Assess(eligibility.Qualification{"guarantee": {"caucion"}}, rent(800000), []eligibility.Rule{onlyPropietaria}, catalog)
	if got.State != eligibility.Ineligible || len(got.Conditions) != 1 || got.Conditions[0].Reason != "not_met" {
		t.Fatalf("want ineligible naming the rule, got %+v", got)
	}
}

func TestUndeclaredFactIsUnknownNeverEligible(t *testing.T) {
	got := eligibility.Assess(eligibility.Qualification{}, rent(800000), []eligibility.Rule{onlyPropietaria}, catalog)
	if got.State != eligibility.Unknown || len(got.Conditions) != 1 || got.Conditions[0].Reason != "missing" {
		t.Fatalf("want unknown naming the missing fact, got %+v", got)
	}
}

func TestNoPublishedRulesIsUnknown(t *testing.T) {
	if got := eligibility.Assess(eligibility.Qualification{"guarantee": {"propietaria"}}, rent(800000), nil, catalog); got.State != eligibility.Unknown {
		t.Fatalf("want unknown, got %+v", got)
	}
}

// From a snapshot listing: "1 garantía Caba o seguro de caución (ver cuales permite la propietaria)".
var ownerDecides = eligibility.Rule{Fact: "guarantee", Operator: "one_of", Values: []string{"propietaria", "caucion"}, Hardness: "discretionary", Evidence: "1 garantía Caba o seguro de caución (ver cuales permite la propietaria)"}

func TestDiscretionaryRuleIsConditionalWithTheConditionNamed(t *testing.T) {
	for _, declared := range [][]string{{"caucion"}, {"recibo_sueldo"}} {
		got := eligibility.Assess(eligibility.Qualification{"guarantee": declared}, rent(800000), []eligibility.Rule{ownerDecides}, catalog)
		if got.State != eligibility.ConditionallyEligible || len(got.Conditions) != 1 || got.Conditions[0].Rule.Evidence != ownerDecides.Evidence {
			t.Fatalf("%v: want conditionally eligible naming the owner's condition, got %+v", declared, got)
		}
	}
}

func TestIncomeMultipleAgainstADeclaredBand(t *testing.T) {
	threeTimes := eligibility.Rule{Fact: "income_band", Operator: "income_multiple", Values: []string{"3"}, Hardness: "hard", Evidence: "ingresos que tripliquen el valor del alquiler"}
	usd := 700.0
	for _, tc := range []struct {
		band   string
		rent   listing.Money
		want   eligibility.State
		reason eligibility.Reason
	}{
		{"3000000-", rent(800000), eligibility.Eligible, ""},
		{"1000000-2000000", rent(800000), eligibility.Ineligible, "not_met"},
		{"2000000-3000000", rent(800000), eligibility.ConditionallyEligible, "near_line"},
		{"3000000-", listing.Money{Amount: &usd, Currency: "USD"}, eligibility.Unknown, "unverifiable"},
	} {
		got := eligibility.Assess(eligibility.Qualification{"income_band": {tc.band}}, tc.rent, []eligibility.Rule{threeTimes}, catalog)
		if got.State != tc.want || (tc.reason != "" && (len(got.Conditions) != 1 || got.Conditions[0].Reason != tc.reason)) {
			t.Errorf("band %s rent %v: want %s/%s, got %+v", tc.band, tc.rent.Currency, tc.want, tc.reason, got)
		}
	}
}

func TestRulesOnInadmissibleFactsAreIgnoredAndReported(t *testing.T) {
	age := eligibility.Rule{Fact: "age", Operator: "one_of", Values: []string{"20+"}, Hardness: "hard"}
	got := eligibility.Assess(eligibility.Qualification{"guarantee": {"propietaria"}, "age": {"19"}}, rent(800000), []eligibility.Rule{onlyPropietaria, age}, catalog)
	if got.State != eligibility.Eligible || len(got.Ignored) != 1 || got.Ignored[0].Fact != "age" {
		t.Fatalf("an inadmissible rule must not decide eligibility, got %+v", got)
	}
}

func TestRelaxationsCountWhatAnotherInstrumentWouldBringBack(t *testing.T) {
	onlyCaucion := eligibility.Rule{Fact: "guarantee", Operator: "one_of", Values: []string{"caucion"}, Hardness: "hard"}
	threeTimes := eligibility.Rule{Fact: "income_band", Operator: "income_multiple", Values: []string{"3"}, Hardness: "hard"}
	candidates := []eligibility.Candidate{
		{Listing: listing.Listing{URL: "a", Price: rent(800000)}, Rules: []eligibility.Rule{onlyCaucion}},
		{Listing: listing.Listing{URL: "b", Price: rent(800000)}, Rules: []eligibility.Rule{onlyCaucion}},
		{Listing: listing.Listing{URL: "c", Price: rent(800000)}, Rules: []eligibility.Rule{onlyPropietaria}},
		// Ineligible for income: no instrument brings it back.
		{Listing: listing.Listing{URL: "d", Price: rent(800000)}, Rules: []eligibility.Rule{onlyPropietaria, threeTimes}},
	}
	q := eligibility.Qualification{"guarantee": {"propietaria"}, "income_band": {"1000000-2000000"}}
	got := eligibility.Relaxations(q, candidates, catalog)
	if len(got) != 1 || got[0] != (eligibility.Relaxation{Fact: "guarantee", Value: "caucion", Count: 2}) {
		t.Fatalf("want 'si conseguís caución, vuelven 2', got %+v", got)
	}
}

// Bad data never makes a property eligible: an operator the evaluator does not
// know is unverifiable, even when its values happen to match what was declared.
func TestUnknownOperatorIsUnverifiableNeverEligible(t *testing.T) {
	for _, hardness := range []eligibility.Hardness{"hard", "discretionary"} {
		between := eligibility.Rule{Fact: "guarantee", Operator: "between", Values: []string{"propietaria"}, Hardness: hardness}
		// Undeclared too: asking the searcher for the fact would not make the rule readable.
		for _, q := range []eligibility.Qualification{{"guarantee": {"propietaria"}}, {}} {
			got := eligibility.Assess(q, rent(800000), []eligibility.Rule{between}, catalog)
			if got.State != eligibility.Unknown || len(got.Conditions) != 1 || got.Conditions[0].Reason != "unverifiable" {
				t.Errorf("%s %v: want unknown/unverifiable, got %+v", hardness, q, got)
			}
		}
	}
}

// The glossary's own example of unknown: an income multiple on a rent in
// dollars. The owner's discretion does not make an unreadable comparison
// conditional.
func TestDiscretionaryIncomeOnADollarRentIsUnverifiable(t *testing.T) {
	usd := 700.0
	threeTimes := eligibility.Rule{Fact: "income_band", Operator: "income_multiple", Values: []string{"3"}, Hardness: "discretionary"}
	got := eligibility.Assess(eligibility.Qualification{"income_band": {"3000000-"}}, listing.Money{Amount: &usd, Currency: "USD"}, []eligibility.Rule{threeTimes}, catalog)
	if got.State != eligibility.Unknown || len(got.Conditions) != 1 || got.Conditions[0].Reason != "unverifiable" {
		t.Fatalf("want unknown/unverifiable, got %+v", got)
	}
}

func TestValidateRefusesRulesTheEvaluatorCannotRead(t *testing.T) {
	for _, tc := range []struct {
		rule eligibility.Rule
		want string
	}{
		{eligibility.Rule{Fact: "guarantee", Operator: "one_of", Hardness: "hard"}, ""},
		{eligibility.Rule{Fact: "income_band", Operator: "income_multiple", Hardness: "discretionary"}, ""},
		{eligibility.Rule{Fact: "guarantee", Operator: "between", Hardness: "hard"}, `unknown eligibility operator "between"`},
		{eligibility.Rule{Fact: "guarantee", Operator: "one_of", Hardness: "soft"}, `unknown eligibility hardness "soft"`},
	} {
		err := tc.rule.Validate()
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || err.Error() != tc.want) {
			t.Errorf("%s/%s: want %q, got %v", tc.rule.Operator, tc.rule.Hardness, tc.want, err)
		}
	}
}
