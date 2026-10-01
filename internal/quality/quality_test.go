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

// auditOne audits a single listing through the batch screen.
func auditOne(item listing.Listing, evaluate jev.Evaluator, model llm.Client) (quality.Review, error) {
	reviews, errs := quality.AuditBatch(context.Background(), []listing.Listing{item}, evaluate, model)
	return reviews[0], errs[0]
}

// screen answers the conflict screen with screenP and the pair check with pairP.
func screen(screenP, pairP float64, calls *int) jev.Evaluator {
	return func(_ context.Context, _ any, qs map[string]jev.Question) (map[string]jev.Answer, error) {
		*calls++
		if _, ok := qs["listing_0"]; ok {
			return map[string]jev.Answer{"listing_0": {Type: "boolean", Probability: screenP}}, nil
		}
		return map[string]jev.Answer{"pair": {Type: "boolean", Probability: pairP}}, nil
	}
}

func TestAuditWithholdsOnlyVerifiedQuoteBackedSameDetailConflict(t *testing.T) {
	item := listing.Listing{Description: "Departamento muy luminoso. El mismo departamento recibe poca luz natural."}
	calls := 0
	evaluate := screen(0.98, 0.99, &calls)
	review, err := auditOne(item, evaluate, quoteModel{`{"first":"muy luminoso","second":"recibe poca luz natural","detail":"natural_light"}`})
	if err != nil || review.Status != quality.Withheld || len(review.Conflicts) != 1 || calls != 2 {
		t.Fatalf("want verified conflict, got %+v, %v, calls=%d", review, err, calls)
	}

	_, err = auditOne(item, evaluate, quoteModel{`{"first":"muy luminoso","second":"invented quote","detail":"natural_light"}`})
	if err == nil {
		t.Fatal("unsupported quote must leave listing pending")
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

// A published label quoted against prose is not proof of a conflict: a district
// sits inside a barrio, and sale and rental can coexist.
func TestLabelQuotesNeedMoreThanJevBeforeWithholding(t *testing.T) {
	for name, tc := range map[string]struct {
		item  listing.Listing
		quote string
	}{
		"neighborhood": {listing.Listing{Neighborhood: "palermo", Description: "Ubicado en Las Cañitas."}, `{"first":"Las Cañitas","second":"neighborhood: palermo","detail":"neighborhood"}`},
		"operation":    {listing.Listing{Operation: "alquiler", Description: "Departamento a la venta."}, `{"first":"a la venta","second":"operation: alquiler","detail":"operation"}`},
	} {
		calls := 0
		review, err := auditOne(tc.item, screen(0.95, 0.95, &calls), quoteModel{tc.quote})
		if err == nil || review.Status == quality.Withheld {
			t.Errorf("%s: label quote alone must need review, got %+v, %v", name, review, err)
		}
	}
}
