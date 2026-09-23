package buyer

import (
	"context"
	"fmt"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
)

// Matcher is independent of the model used to interpret or explain a request.
type Matcher interface {
	Evaluate(context.Context, matching.Request) (matching.Result, error)
}

// RetrievedBranch holds already hard-filtered, comparable candidates.
// Complete must be false when retrieval was capped. This does not run SQL.
type RetrievedBranch struct {
	ID       string
	Complete bool
	Request  matching.Request
}

type Selection struct {
	Candidate matching.Candidate `json:"candidate"`
	Match     matching.Match     `json:"match"`
}

type ExplanationBranch struct {
	ID             string               `json:"id"`
	Complete       bool                 `json:"retrieval_complete"`
	CandidateCount int                  `json:"candidate_count"`
	Criteria       []matching.Criterion `json:"criteria"`
	Selections     []Selection          `json:"selections"`
}

// ExplanationPacket contains selected evidence only, with branch-local ranks.
// It is not yet the frontend's flat, globally ranked TurnResponse.
type ExplanationPacket struct {
	Branches []ExplanationBranch `json:"branches"`
}

// BuildShortlist prepares each-branch explanation input for the experiment.
// The caller owns parsing, retrieval, and plan revision checks. It must not
// construct branches by collecting arbitrary historic search tool calls.
func BuildShortlist(ctx context.Context, matcher Matcher, branches []RetrievedBranch, perBranch int) (ExplanationPacket, error) {
	if perBranch < 1 || perBranch > 5 || len(branches) > 10 || matcher == nil {
		return ExplanationPacket{}, fmt.Errorf("buyer: invalid shortlist limits or matcher")
	}
	seen := map[string]bool{}
	for _, b := range branches {
		if b.ID == "" || seen[b.ID] {
			return ExplanationPacket{}, fmt.Errorf("buyer: invalid branch ID")
		}
		seen[b.ID] = true
	}
	packet := ExplanationPacket{Branches: []ExplanationBranch{}}
	for _, branch := range branches {
		if err := ctx.Err(); err != nil {
			return ExplanationPacket{}, err
		}
		result, err := matcher.Evaluate(ctx, branch.Request)
		if err != nil {
			return ExplanationPacket{}, fmt.Errorf("buyer: branch %s: %w", branch.ID, err)
		}
		candidates := map[string]matching.Candidate{}
		for _, c := range branch.Request.Candidates {
			candidates[c.ID] = c
		}
		out := ExplanationBranch{ID: branch.ID, Complete: branch.Complete, CandidateCount: len(branch.Request.Candidates), Criteria: branch.Request.Criteria, Selections: []Selection{}}
		ids := map[string]bool{}
		if len(result.Matches) != len(candidates) {
			return ExplanationPacket{}, fmt.Errorf("buyer: incomplete matching result")
		}
		for i, m := range result.Matches {
			c, ok := candidates[m.CandidateID]
			if !ok || ids[m.CandidateID] || m.Rank != i+1 {
				return ExplanationPacket{}, fmt.Errorf("buyer: invalid matching result")
			}
			ids[m.CandidateID] = true
			if i >= perBranch {
				continue
			}
			// Explanation consumes conclusions and references, not probability arrays.
			assessments := append([]matching.Assessment(nil), m.Assessments...)
			for j := range assessments {
				assessments[j].Probabilities = nil
			}
			m.Assessments = assessments
			out.Selections = append(out.Selections, Selection{Candidate: c, Match: m})
		}
		packet.Branches = append(packet.Branches, out)
	}
	return packet, nil
}
