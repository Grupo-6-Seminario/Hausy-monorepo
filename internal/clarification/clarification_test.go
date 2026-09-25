package clarification_test

import (
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/clarification"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

func number(n int) *int { return &n }

func TestTypedAnswerPreservesAnIndependentRoomCondition(t *testing.T) {
	plan := intake.Plan{Branches: []search.Query{{MinRooms: number(3), MaxRooms: number(3)}}}
	resolved, err := clarification.Apply(plan, []clarification.Effect{{Field: "min_bedrooms", Value: "2"}})
	if err != nil || *resolved.Branches[0].MinRooms != 3 || *resolved.Branches[0].MaxRooms != 3 || *resolved.Branches[0].MinBedrooms != 2 {
		t.Fatalf("two bedrooms must not erase an independent three-room requirement: %+v %v", resolved, err)
	}
	if plan.Branches[0].MinBedrooms != nil {
		t.Fatal("applying an answer mutated the tentative plan")
	}
}

func TestTypedAnswerCanReplaceOnlyTheDraftFieldsTiedToItsAmbiguity(t *testing.T) {
	plan := intake.Plan{Branches: []search.Query{{MinRooms: number(2), MaxRooms: number(2)}}}
	resolved, err := clarification.Apply(plan, []clarification.Effect{{Field: "min_bedrooms", Value: "2", Clear: []string{"min_rooms", "max_rooms"}}})
	if err != nil || resolved.Branches[0].MinRooms != nil || resolved.Branches[0].MaxRooms != nil || *resolved.Branches[0].MinBedrooms != 2 {
		t.Fatalf("the selected meaning should replace only its tentative room reading: %+v %v", resolved, err)
	}
}
