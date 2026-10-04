// Package receipt is a disposable design experiment, not a matching implementation.
package receipt

import (
	"cmp"
	"fmt"
	"slices"
)

type Assessment struct {
	State, Provenance, Quote string
	Weight                   int
}

// Candidate already passed retrieval and eligibility. This experiment isolates preference ordering.
type Candidate struct {
	ID          string
	Assessments []Assessment
}

type Item struct {
	ID                           string
	Rank, Numerator, Denominator int
	Reason                       string
	Assessments                  []Assessment
}

// Ranking keeps the authoritative ordering private. Callers receive copied projections.
type Ranking struct{ items []Item }

func contribution(a Assessment) int {
	if a.Provenance == "inferred" {
		return 0
	}
	switch a.State {
	case "supported":
		return a.Weight
	case "contradicted":
		return -a.Weight
	default:
		return 0
	}
}

func New(candidates []Candidate) (Ranking, error) {
	items := make([]Item, len(candidates))
	seen := map[string]bool{}
	for i, candidate := range candidates {
		if candidate.ID == "" || seen[candidate.ID] {
			return Ranking{}, fmt.Errorf("invalid candidate ID")
		}
		seen[candidate.ID] = true
		item := Item{ID: candidate.ID, Assessments: slices.Clone(candidate.Assessments)}
		for _, a := range candidate.Assessments {
			if a.Weight != 1 && a.Weight != 2 {
				return Ranking{}, fmt.Errorf("invalid weight")
			}
			if a.State != "supported" && a.State != "contradicted" && a.State != "unknown" {
				return Ranking{}, fmt.Errorf("invalid state")
			}
			if a.Provenance != "published" && a.Provenance != "stated" && a.Provenance != "inferred" {
				return Ranking{}, fmt.Errorf("invalid provenance")
			}
			if a.State != "unknown" && a.Quote == "" {
				return Ranking{}, fmt.Errorf("missing evidence")
			}
			item.Numerator += contribution(a)
			item.Denominator += a.Weight
		}
		items[i] = item
	}
	slices.SortFunc(items, func(a, b Item) int {
		if c := compareScore(a, b); c != 0 {
			return -c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	for i := range items {
		items[i].Rank = i + 1
		items[i].Reason = "lone-result"
		if i+1 < len(items) {
			items[i].Reason = "stable-tie"
			if compareScore(items[i], items[i+1]) != 0 {
				items[i].Reason = "preference-score"
			}
		} else if len(items) > 1 {
			items[i].Reason = "last-result"
		}
	}
	return Ranking{items: items}, nil
}

func compareScore(a, b Item) int {
	ad, bd := max(1, a.Denominator), max(1, b.Denominator)
	return cmp.Compare(a.Numerator*bd, b.Numerator*ad)
}

func (r Ranking) Packet() []Item {
	out := slices.Clone(r.items)
	for i := range out {
		out[i].Assessments = slices.Clone(out[i].Assessments)
	}
	return out
}

// Mutable creates the alternative public record shape, with the same initial behavior.
// Its caller owns ordering and projection; exporting both makes later divergence representable.
func Mutable(candidates []Candidate) ([]Item, error) {
	r, err := New(candidates)
	return r.Packet(), err
}
