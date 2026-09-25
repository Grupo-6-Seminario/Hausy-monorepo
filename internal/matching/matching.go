// Package matching classifies listing evidence independently of chat providers.
// Preference fit never establishes rental eligibility.
package matching

import (
	"context"
	"fmt"
	"math"
	"sort"
)

type Criterion struct {
	ID             string `json:"id"`
	Text           string `json:"text"`
	Priority       string `json:"priority"`
	AttributeType  string `json:"attribute_type,omitempty"`
	AttributeValue string `json:"attribute_value,omitempty"`
}

type Evidence struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Provenance string `json:"provenance"`
	Type       string `json:"type,omitempty"`
	Value      string `json:"value,omitempty"`
}

type Candidate struct {
	ID       string     `json:"id"`
	URL      string     `json:"url"`
	Evidence []Evidence `json:"evidence"`
}

// Request is one comparable group. The caller retrieves and hard-filters it.
type Request struct {
	Criteria   []Criterion `json:"criteria"`
	Candidates []Candidate `json:"candidates"`
}

type Assessment struct {
	CriterionID  string   `json:"criterion_id"`
	Assessment   string   `json:"assessment,omitempty"`
	EvidenceRefs []string `json:"evidence_refs"`
	// EvidenceAssessment is what per-record answers imply, set when it disagrees.
	EvidenceAssessment string             `json:"evidence_assessment,omitempty"`
	Status             string             `json:"evaluation_status"`
	Probabilities      map[string]float64 `json:"probabilities,omitempty"`
}

type Match struct {
	CandidateID string       `json:"candidate_id"`
	Rank        int          `json:"rank"`
	Assessments []Assessment `json:"assessments"`
	Supported   int          `json:"supported"`
	Unknown     int          `json:"unknown"`
	NeedsReview int          `json:"needs_review"`
}

type Result struct {
	PolicyVersion string  `json:"policy_version"`
	Matches       []Match `json:"matches"`
}

// ProbabilityTolerance absorbs providers rounding each probability to two
// decimals: four rounded options can sum to 0.98–1.02 and tie the chosen one.
const ProbabilityTolerance = 0.02

// Classifier is the replaceable evidence classification adapter, not a chat model.
type Classifier interface {
	Classify(context.Context, Candidate, []Criterion) ([]Assessment, error)
}

// BatchClassifier can assess a whole comparable group in one provider call.
type BatchClassifier interface {
	ClassifyBatch(context.Context, []Candidate, []Criterion) ([][]Assessment, error)
}

type Evaluator struct{ classifier Classifier }

func New(classifier Classifier) *Evaluator { return &Evaluator{classifier: classifier} }

func (e *Evaluator) Evaluate(ctx context.Context, req Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	ids := map[string]bool{}
	for _, q := range req.Criteria {
		if q.ID == "" || ids[q.ID] || (q.Priority != "" && q.Priority != "primary" && q.Priority != "secondary") {
			return Result{}, fmt.Errorf("matching: invalid or duplicate criterion")
		}
		ids[q.ID] = true
	}
	ids = map[string]bool{}
	seenURLs := map[string]bool{}
	for _, c := range req.Candidates {
		if c.ID == "" || c.URL == "" || ids[c.ID] || seenURLs[c.URL] {
			return Result{}, fmt.Errorf("matching: invalid or duplicate candidate")
		}
		ids[c.ID] = true
		seenURLs[c.URL] = true
		evidence := map[string]bool{}
		for _, r := range c.Evidence {
			if r.ID == "" || r.Text == "" || evidence[r.ID] || (r.Provenance != "stated" && r.Provenance != "inferred" && r.Provenance != "published") {
				return Result{}, fmt.Errorf("matching: invalid evidence")
			}
			evidence[r.ID] = true
		}
	}
	result := Result{PolicyVersion: "evidence-tiers-v1", Matches: []Match{}}
	urls := map[string]string{}
	var batched [][]Assessment
	if batch, ok := e.classifier.(BatchClassifier); ok && len(req.Criteria) > 0 && len(req.Candidates) > 1 {
		var err error
		batched, err = batch.ClassifyBatch(ctx, req.Candidates, req.Criteria)
		if err != nil {
			return Result{}, err
		}
		if len(batched) != len(req.Candidates) {
			return Result{}, fmt.Errorf("matching: incomplete batch")
		}
	}
	for i, candidate := range req.Candidates {
		var assessments []Assessment
		var err error
		if len(req.Criteria) > 0 {
			if e.classifier == nil {
				return Result{}, fmt.Errorf("matching: missing classifier")
			}
			if batched != nil {
				assessments = batched[i]
			} else {
				assessments, err = e.classifier.Classify(ctx, candidate, req.Criteria)
			}
		}
		if err != nil {
			return Result{}, err
		}
		assessments, err = validateAssessments(candidate, req.Criteria, assessments)
		if err != nil {
			return Result{}, err
		}
		match := Match{CandidateID: candidate.ID, Assessments: assessments}
		for _, a := range assessments {
			if a.Status == "needs_review" {
				match.NeedsReview++
				continue
			}
			if a.Assessment == "supported" {
				match.Supported++
			}
			if a.Assessment == "insufficient_evidence" || a.Assessment == "conflicting_evidence" {
				match.Unknown++
			}
		}
		urls[candidate.ID] = candidate.URL
		result.Matches = append(result.Matches, match)
	}
	score := func(m Match) [4]int {
		var s [4]int
		for i, a := range m.Assessments {
			if a.Status != "evaluated" {
				continue
			}
			offset := 0
			if req.Criteria[i].Priority == "secondary" {
				offset = 2
			}
			if a.Assessment == "supported" {
				s[offset]++
			}
			if a.Assessment == "contradicted" {
				s[offset+1]--
			}
		}
		return s
	}
	sort.SliceStable(result.Matches, func(i, j int) bool {
		a, b := score(result.Matches[i]), score(result.Matches[j])
		for k := range a {
			if a[k] != b[k] {
				return a[k] > b[k]
			}
		}
		return urls[result.Matches[i].CandidateID] < urls[result.Matches[j].CandidateID]
	})
	for i := range result.Matches {
		result.Matches[i].Rank = i + 1
	}
	return result, nil
}

func validateAssessments(c Candidate, criteria []Criterion, answers []Assessment) ([]Assessment, error) {
	if len(answers) != len(criteria) {
		return nil, fmt.Errorf("matching: incomplete assessments")
	}
	allowed := map[string]bool{"supported": true, "contradicted": true, "insufficient_evidence": true, "conflicting_evidence": true}
	evidence := map[string]bool{}
	for _, e := range c.Evidence {
		evidence[e.ID] = true
	}
	byID := map[string]Assessment{}
	for _, a := range answers {
		if _, exists := byID[a.CriterionID]; exists {
			return nil, fmt.Errorf("matching: duplicate assessment")
		}
		if !allowed[a.Assessment] || (a.Status != "evaluated" && a.Status != "needs_review") {
			return nil, fmt.Errorf("matching: invalid assessment")
		}
		if a.Status == "evaluated" && a.Assessment != "insufficient_evidence" && len(a.EvidenceRefs) == 0 {
			return nil, fmt.Errorf("matching: ungrounded assessment")
		}
		refs := map[string]bool{}
		for _, ref := range a.EvidenceRefs {
			if !evidence[ref] || refs[ref] {
				return nil, fmt.Errorf("matching: invalid evidence reference")
			}
			refs[ref] = true
		}
		if a.Probabilities != nil {
			sum := 0.0
			if len(a.Probabilities) != 4 {
				return nil, fmt.Errorf("matching: incomplete probabilities")
			}
			for key, p := range a.Probabilities {
				if !allowed[key] || math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
					return nil, fmt.Errorf("matching: invalid probability")
				}
				sum += p
				if p > a.Probabilities[a.Assessment]+ProbabilityTolerance {
					return nil, fmt.Errorf("matching: choice disagrees with probabilities")
				}
			}
			if math.Abs(sum-1) > ProbabilityTolerance {
				return nil, fmt.Errorf("matching: probabilities do not sum to one")
			}
		}
		byID[a.CriterionID] = a
	}
	out := make([]Assessment, 0, len(criteria))
	for _, q := range criteria {
		a, ok := byID[q.ID]
		if !ok {
			return nil, fmt.Errorf("matching: missing criterion %s", q.ID)
		}
		out = append(out, a)
	}
	return out, nil
}

// Baseline recognizes exact attributes only. Missing attributes are unknown.
type Baseline struct{}

func (Baseline) Classify(ctx context.Context, c Candidate, criteria []Criterion) ([]Assessment, error) {
	out := make([]Assessment, 0, len(criteria))
	for _, criterion := range criteria {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		a := Assessment{CriterionID: criterion.ID, Assessment: "insufficient_evidence", Status: "evaluated", EvidenceRefs: []string{}}
		published := false
		for _, e := range c.Evidence {
			if e.Type == criterion.AttributeType && e.Provenance == "published" {
				published = true
			}
		}
		supports, opposes := false, false
		for _, e := range c.Evidence {
			if published && e.Provenance != "published" {
				continue
			}
			if criterion.AttributeType != "" && e.Type == criterion.AttributeType && e.Value == criterion.AttributeValue {
				supports = true
				a.EvidenceRefs = append(a.EvidenceRefs, e.ID)
			}
			// Only explicit boolean opposites are deterministic contradictions;
			// a different amenity or a nuanced quality is not an opposite.
			if criterion.AttributeType != "" && e.Type == criterion.AttributeType && ((criterion.AttributeValue == "yes" && e.Value == "no") || (criterion.AttributeValue == "no" && e.Value == "yes")) {
				opposes = true
				a.EvidenceRefs = append(a.EvidenceRefs, e.ID)
			}
		}
		if supports {
			a.Assessment = "supported"
		}
		if opposes {
			a.Assessment = "contradicted"
		}
		if supports && opposes {
			a.Assessment = "conflicting_evidence"
		}
		out = append(out, a)
	}
	return out, nil
}
