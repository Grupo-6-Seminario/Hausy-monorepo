package quality_test

import (
	"context"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/jev"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
)

type quoteModel struct{ response string }

func (m quoteModel) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: m.response}, nil
}

func TestAuditWithholdsOnlyVerifiedQuoteBackedSameDetailConflict(t *testing.T) {
	item := listing.Listing{Description: "Departamento muy luminoso. El mismo departamento recibe poca luz natural."}
	calls := 0
	evaluate := func(_ context.Context, _ any, qs map[string]jev.Question) (map[string]jev.Answer, error) {
		calls++
		if _, ok := qs["conflict"]; ok {
			return map[string]jev.Answer{"conflict": {Type: "boolean", Probability: 0.98}}, nil
		}
		return map[string]jev.Answer{"pair": {Type: "boolean", Probability: 0.99}}, nil
	}
	review, err := quality.Audit(context.Background(), item, evaluate, quoteModel{`{"first":"muy luminoso","second":"recibe poca luz natural","detail":"natural_light"}`})
	if err != nil || review.Status != quality.Withheld || len(review.Conflicts) != 1 || calls != 2 {
		t.Fatalf("want verified conflict, got %+v, %v, calls=%d", review, err, calls)
	}

	_, err = quality.Audit(context.Background(), item, evaluate, quoteModel{`{"first":"muy luminoso","second":"invented quote","detail":"natural_light"}`})
	if err == nil {
		t.Fatal("unsupported quote must leave listing pending")
	}
}

func TestAuditKeepsRoomScopeAndTemporalChangesVisible(t *testing.T) {
	item := listing.Listing{Description: "Dormitorio luminoso. Living con poca luz. Antes tenía dos ambientes; hoy tiene tres."}
	evaluate := func(_ context.Context, _ any, qs map[string]jev.Question) (map[string]jev.Answer, error) {
		return map[string]jev.Answer{"conflict": {Type: "boolean", Probability: 0.02}}, nil
	}
	review, err := quality.Audit(context.Background(), item, evaluate, quoteModel{})
	if err != nil || review.Status != quality.Passed {
		t.Fatalf("different rooms and time are not contradictions: %+v, %v", review, err)
	}
}

func TestBatchScreensSeveralStoredListingsInOneJevCall(t *testing.T) {
	items := []listing.Listing{{Description: "Luminoso."}, {Description: "Silencioso."}}
	calls := 0
	evaluate := func(_ context.Context, state any, qs map[string]jev.Question) (map[string]jev.Answer, error) {
		calls++
		if len(qs) != 2 {
			t.Fatalf("want one question per listing, got %d", len(qs))
		}
		return map[string]jev.Answer{"listing_0": {Type: "boolean", Probability: 0.01}, "listing_1": {Type: "boolean", Probability: 0.02}}, nil
	}
	reviews, errs := quality.AuditBatch(context.Background(), items, evaluate, quoteModel{})
	if calls != 1 || len(reviews) != 2 || reviews[0].Status != quality.Passed || reviews[1].Status != quality.Passed || errs[0] != nil || errs[1] != nil {
		t.Fatalf("batch = %+v, errors = %v, calls = %d", reviews, errs, calls)
	}
}

func TestNeighborhoodLabelsNeedAddressVerificationBeforeWithholding(t *testing.T) {
	item := listing.Listing{Neighborhood: "palermo", Description: "Ubicado en Las Cañitas."}
	evaluate := func(_ context.Context, _ any, qs map[string]jev.Question) (map[string]jev.Answer, error) {
		if _, ok := qs["conflict"]; ok {
			return map[string]jev.Answer{"conflict": {Type: "boolean", Probability: 0.95}}, nil
		}
		return map[string]jev.Answer{"pair": {Type: "boolean", Probability: 0.95}}, nil
	}
	review, err := quality.Audit(context.Background(), item, evaluate, quoteModel{`{"first":"Las Cañitas","second":"neighborhood: palermo","detail":"neighborhood"}`})
	if err == nil || review.Status == quality.Withheld {
		t.Fatalf("district label alone must need review, got %+v, %v", review, err)
	}
}

func TestSaleAndRentalClaimsNeedExclusivityBeforeWithholding(t *testing.T) {
	item := listing.Listing{Operation: "alquiler", Description: "Departamento a la venta."}
	evaluate := func(_ context.Context, _ any, qs map[string]jev.Question) (map[string]jev.Answer, error) {
		if _, ok := qs["conflict"]; ok {
			return map[string]jev.Answer{"conflict": {Type: "boolean", Probability: 0.95}}, nil
		}
		return map[string]jev.Answer{"pair": {Type: "boolean", Probability: 0.95}}, nil
	}
	review, err := quality.Audit(context.Background(), item, evaluate, quoteModel{`{"first":"a la venta","second":"operation: alquiler","detail":"operation"}`})
	if err == nil || review.Status == quality.Withheld {
		t.Fatalf("sale and rental can coexist: %+v, %v", review, err)
	}
}
