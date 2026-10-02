package eligibility_test

import (
	"errors"
	"reflect"
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

// The catalog the form offers; only a declared qualification inside it is used.
var form = eligibility.Catalog{
	"guarantee":         {Admissible: true, Multiple: true, Choices: []eligibility.Choice{{Value: "propietaria"}, {Value: "caucion"}}},
	"income_documented": {Admissible: true, Choices: []eligibility.Choice{{Value: "yes"}, {Value: "no"}}},
	"pets":              {Admissible: true, Multiple: true, Choices: []eligibility.Choice{{Value: "none"}, {Value: "dog"}, {Value: "cat"}}},
	"age":               {Admissible: false},
}

func TestValidateQualificationReturnsTheFirstTypedError(t *testing.T) {
	for _, tc := range []struct {
		name string
		q    eligibility.Qualification
		want *eligibility.QualificationError
	}{
		{"empty is valid", eligibility.Qualification{}, nil},
		{"declared choices", eligibility.Qualification{"guarantee": {"propietaria", "caucion"}, "income_documented": {"no"}}, nil},
		{"unknown fact", eligibility.Qualification{"zodiac": {"leo"}}, &eligibility.QualificationError{Code: "unknown_fact", Fact: "zodiac"}},
		{"inadmissible fact", eligibility.Qualification{"age": {"30"}}, &eligibility.QualificationError{Code: "inadmissible_fact", Fact: "age"}},
		{"value outside the choices", eligibility.Qualification{"guarantee": {"aval_bancario"}}, &eligibility.QualificationError{Code: "invalid_value", Fact: "guarantee"}},
		{"several pets", eligibility.Qualification{"pets": {"dog", "cat"}}, nil},
		// "No tengo" next to "Perro" would clear a refuse-pets rule for a dog owner.
		{"none with another value", eligibility.Qualification{"pets": {"none", "dog"}}, &eligibility.QualificationError{Code: "invalid_value", Fact: "pets"}},
		{"two values for a single choice", eligibility.Qualification{"income_documented": {"yes", "no"}}, &eligibility.QualificationError{Code: "too_many_values", Fact: "income_documented"}},
		// Facts are checked in name order, so the same body always gets the same error.
		{"first by fact name", eligibility.Qualification{"income_documented": {"maybe"}, "age": {"30"}}, &eligibility.QualificationError{Code: "inadmissible_fact", Fact: "age"}},
	} {
		err := eligibility.ValidateQualification(form, tc.q)
		if tc.want == nil {
			if err != nil {
				t.Errorf("%s: want valid, got %v", tc.name, err)
			}
			continue
		}
		var got *eligibility.QualificationError
		if !errors.As(err, &got) || *got != *tc.want {
			t.Errorf("%s: want %+v, got %v", tc.name, *tc.want, err)
		}
	}
}

// "Si conseguís none, vuelven 1" is no advice: relaxations name a guarantee
// the searcher could get, never a pet to give up.
func TestRelaxationsNeverSuggestGivingUpAPet(t *testing.T) {
	noPets := eligibility.Rule{Fact: "pets", Operator: "one_of", Values: []string{"none"}, Hardness: "hard"}
	withPets := eligibility.Catalog{"pets": {Admissible: true}, "guarantee": {Admissible: true}}
	candidates := []eligibility.Candidate{{Listing: listing.Listing{URL: "a", Price: rent(800000)}, Rules: []eligibility.Rule{noPets}}}
	if got := eligibility.Relaxations(eligibility.Qualification{"pets": {"dog"}}, candidates, withPets); len(got) != 0 {
		t.Fatalf("want no relaxation, got %+v", got)
	}
}

// Each phrase is a refusal from the committed parse. A refusal names the
// animals it refuses, so the rule admits the rest; an owner's preference is
// discretionary, not final.
func TestPublishedRulesAdmitTheAnimalsARefusalLeavesOut(t *testing.T) {
	refusal := func(evidence string) []listing.Attribute {
		return []listing.Attribute{{Type: "pets_allowed", Value: "no", Provenance: listing.Stated, Evidence: evidence}}
	}
	for _, tc := range []struct {
		evidence string
		values   []string
		hardness eligibility.Hardness
	}{
		{"No se aceptan mascotas", []string{"none"}, "hard"},
		{"Máximo para 2 personas, sin mascotas.", []string{"none"}, "hard"},
		{"El departamento es apto mascotas ( gatos no)", []string{"none", "dog"}, "hard"},
		{"No se permite perro", []string{"none", "cat"}, "hard"},
		{"Mascotas: preferentemente NO", []string{"none"}, "discretionary"},
		{"El inmueble no es apto para mascotas grandes por expresa solicitud de sus propietarios", []string{"none"}, "discretionary"},
	} {
		got := eligibility.PublishedRules(refusal(tc.evidence))
		want := []eligibility.Rule{{Fact: "pets", Operator: "one_of", Values: tc.values, Hardness: tc.hardness, Visibility: "public", Source: "parsed", Evidence: tc.evidence}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q:\n got %+v\nwant %+v", tc.evidence, got, want)
		}
	}
}
