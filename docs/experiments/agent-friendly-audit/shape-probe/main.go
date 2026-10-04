package main

import (
	"encoding/json"
	"os"
	"reflect"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/docs/experiments/agent-friendly-audit/receipt"
)

func main() {
	cases := []receipt.Candidate{
		{ID: "supported", Assessments: []receipt.Assessment{{State: "supported", Provenance: "published", Quote: "Muy luminoso", Weight: 1}, {State: "unknown", Provenance: "published", Weight: 1}}},
		{ID: "contradicted", Assessments: []receipt.Assessment{{State: "contradicted", Provenance: "published", Quote: "Poca luz", Weight: 1}, {State: "unknown", Provenance: "published", Weight: 1}}},
		{ID: "hint", Assessments: []receipt.Assessment{{State: "supported", Provenance: "inferred", Quote: "Al frente", Weight: 1}, {State: "unknown", Provenance: "published", Weight: 1}}},
		{ID: "unknown", Assessments: []receipt.Assessment{{State: "unknown", Provenance: "published", Weight: 1}, {State: "unknown", Provenance: "published", Weight: 1}}},
	}
	sealed, err := receipt.New(cases)
	if err != nil {
		panic(err)
	}
	mutable, err := receipt.Mutable(cases)
	if err != nil {
		panic(err)
	}
	expectedIDs := []string{"supported", "hint", "unknown", "contradicted"}
	expectedNumerators := []int{1, 0, 0, -1}
	expectedReasons := []string{"preference-score", "stable-tie", "preference-score", "last-result"}
	initial := sealed.Packet()
	for i, item := range initial {
		if item.ID != expectedIDs[i] || item.Numerator != expectedNumerators[i] || item.Denominator != 2 || item.Reason != expectedReasons[i] {
			panic("independent literal expectation failed")
		}
	}
	if !reflect.DeepEqual(initial, mutable) {
		panic("shapes must agree before fault injection")
	}
	projection := sealed.Packet()
	projection[0].Rank = 99
	projection[0].Assessments[0].Quote = "invented"
	if !reflect.DeepEqual(sealed.Packet(), initial) {
		panic("writer projection mutated authority")
	}
	mutable[0].Numerator = 99
	// The caller can make the score disagree with its retained contribution and reason.
	output := map[string]any{
		"correct_initial_result":                        initial,
		"sealed_authority_survives_projection_mutation": reflect.DeepEqual(sealed.Packet(), initial),
		"public_shape_accepts_inconsistent_numerator":   mutable[0].Numerator == 99 && mutable[0].Assessments[0].Weight == 1,
		"scope": "preference accounting, stable ties, evidence retention and projection mutation only",
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	if err := e.Encode(output); err != nil {
		panic(err)
	}
}
