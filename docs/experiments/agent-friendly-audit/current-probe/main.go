// This probe observes the actual buyer and matching interfaces with deterministic adapters.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

type planner struct{ plan intake.Plan }

func (p planner) Plan(context.Context, []string, intake.Plan) (intake.Plan, error) {
	return p.plan, nil
}

type inventory []eligibility.Candidate

func (i inventory) Candidates(_ context.Context, q search.Query) ([]eligibility.Candidate, error) {
	// Literal neighborhood control prevents this mock from pretending to verify SQL gates.
	if len(q.Neighborhoods) != 1 || q.Neighborhoods[0] != "palermo" {
		return nil, fmt.Errorf("probe only supports Palermo")
	}
	return i, nil
}
func (inventory) Facts(context.Context) (eligibility.Catalog, error) {
	return eligibility.Catalog{}, nil
}

type writer struct{ packet buyer.Packet }

func (w *writer) Write(_ context.Context, p buyer.Packet, _ func(string)) (string, error) {
	w.packet = p
	return "recorded", nil
}

type classifier struct{}

func (classifier) Classify(_ context.Context, c matching.Candidate, qs []matching.Criterion) ([]matching.Assessment, error) {
	answer := "insufficient_evidence"
	refs := []string{}
	if c.ID == "z-supported" {
		answer, refs = "supported", []string{"description"}
	}
	if c.ID == "a-contradicted" {
		answer, refs = "contradicted", []string{"description"}
	}
	out := make([]matching.Assessment, len(qs))
	for i, q := range qs {
		out[i] = matching.Assessment{CriterionID: q.ID, Assessment: answer, EvidenceRefs: refs, Status: "evaluated"}
	}
	return out, nil
}

func candidate(id, description string, attributes ...listing.Attribute) eligibility.Candidate {
	return eligibility.Candidate{Listing: listing.Listing{URL: id, Neighborhood: "palermo", Description: description, Attributes: attributes}}
}

func main() {
	ctx := context.Background()
	output := map[string]any{}
	for _, required := range []bool{false, true} {
		q := search.Query{Neighborhoods: []string{"palermo"}, Operation: "alquiler"}
		filter := []search.AttributeFilter{{Type: "natural_light", Value: "high"}}
		if required {
			q.RequiredAttributes = filter
		} else {
			q.PreferredAttributes = filter
		}
		p := intake.Plan{Intent: "new_search", Sort: "relevance", Branches: []search.Query{q}}
		w := &writer{}
		stock := inventory{
			candidate("z-supported", "Muy luminoso."),
			candidate("a-contradicted", "Poca luz natural."),
			candidate("b-unknown", "Sin información de luz."),
			candidate("c-hint", "Al frente.", listing.Attribute{Type: "exposure", Value: "frente", Provenance: listing.Stated}),
		}
		agent := buyer.NewAgent(planner{p}, stock, w, buyer.WithMatching(classifier{}))
		response, err := agent.HandleMessage(ctx, "probe", "Palermo con buena luz natural", nil, buyer.Events{})
		if err != nil {
			panic(err)
		}
		name := "preferred-light"
		if required {
			name = "required-light"
		}
		output[name] = map[string]any{"results": response.Listings, "writer_packet": w.packet}
	}
	baseline, err := matching.New(matching.Baseline{}).Evaluate(ctx, matching.Request{
		Criteria:   []matching.Criterion{{ID: "pets", AttributeType: "pets_allowed", AttributeValue: "yes"}},
		Candidates: []matching.Candidate{{ID: "inferred-only", URL: "inferred-only", Evidence: []matching.Evidence{{ID: "hint", Text: "Inferencia del parser", Provenance: "inferred", Type: "pets_allowed", Value: "yes"}}}},
	})
	if err != nil {
		panic(err)
	}
	output["inferred-only-baseline"] = baseline
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		panic(err)
	}
}
