package buyer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
)

type matcherFunc func(context.Context, matching.Request) (matching.Result, error)

func (f matcherFunc) Evaluate(ctx context.Context, r matching.Request) (matching.Result, error) {
	return f(ctx, r)
}

func TestShortlistDoesNotHideFailedSecondBranch(t *testing.T) {
	matcher := matcherFunc(func(_ context.Context, r matching.Request) (matching.Result, error) {
		if len(r.Candidates) > 0 {
			return matching.Result{}, errors.New("classification unavailable")
		}
		return matching.Result{}, nil
	})
	_, err := buyer.BuildShortlist(context.Background(), matcher, []buyer.RetrievedBranch{{ID: "empty"}, {ID: "failed", Request: matching.Request{Candidates: []matching.Candidate{{ID: "p", URL: "url"}}}}}, 3)
	if err == nil {
		t.Fatal("returned a misleading partial shortlist")
	}
}

func TestExplanationPacketKeepsEveryBranchAndOnlySelectedEvidence(t *testing.T) {
	branches := []buyer.RetrievedBranch{
		{ID: "a", Complete: true, Request: matching.Request{Candidates: []matching.Candidate{{ID: "p1", URL: "a"}, {ID: "p2", URL: "b", Evidence: []matching.Evidence{{ID: "unused", Text: "Must not enter explanation", Provenance: "stated"}}}}}},
		{ID: "b", Complete: true, Request: matching.Request{Candidates: []matching.Candidate{{ID: "p3", URL: "c"}}}},
		{ID: "empty", Complete: true},
	}
	packet, err := buyer.BuildShortlist(context.Background(), matching.New(matching.Baseline{}), branches, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Branches) != 3 {
		t.Fatalf("lost a branch: %+v", packet)
	}
	if len(packet.Branches[0].Selections) != 1 || packet.Branches[0].Selections[0].Candidate.ID != "p1" {
		t.Fatalf("wrong shortlist: %+v", packet)
	}
	if packet.Branches[1].Selections[0].Candidate.ID != "p3" || len(packet.Branches[2].Selections) != 0 {
		t.Fatalf("lost independent branch results: %+v", packet)
	}
	if packet.Branches[0].CandidateCount != 2 {
		t.Fatal("lost population count")
	}
}
