package matching

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// FitCriterion carries prototype priorities without changing conversational intake.
type FitCriterion struct {
	Criterion
	Strength       string `json:"strength"`
	Weight         int    `json:"weight,omitempty"`
	SourceQuote    string `json:"source_quote,omitempty"`
	PrioritySource string `json:"priority_source,omitempty"`
}
type FitRequest struct {
	Criteria   []FitCriterion `json:"criteria"`
	Candidates []Candidate    `json:"candidates"`
}
type Contribution struct {
	Criterion           FitCriterion `json:"criterion"`
	Assessment          Assessment   `json:"assessment"`
	EffectiveAssessment string       `json:"effective_assessment"`
	Evidence            []Evidence   `json:"evidence"`
	Points              int          `json:"points"`
}
type FitMatch struct {
	CandidateID           string         `json:"candidate_id"`
	RequiredFit           string         `json:"required_fit"`
	Contributions         []Contribution `json:"contributions"`
	Numerator             int            `json:"numerator"`
	Denominator           int            `json:"denominator"`
	Score                 float64        `json:"score"`
	NoPreferenceAdvantage bool           `json:"no_preference_advantage"`
	Failure               string         `json:"failure,omitempty"`
}
type FitResult struct {
	PolicyVersion string     `json:"policy_version"`
	Matches       []FitMatch `json:"matches"`
}

// UnavailableFit keeps a timed-out assessment batch in the search with no
// claimed semantic evidence. Deterministic listing fields are still scored.
func UnavailableFit(req FitRequest, reason string) FitResult {
	result, err := New(nil).AssessFit(context.Background(), req)
	if err != nil {
		return FitResult{PolicyVersion: "weighted-net-fit-v1", Matches: []FitMatch{}}
	}
	for i := range result.Matches {
		match := &result.Matches[i]
		match.Failure = reason
		match.Numerator = 0
		match.Denominator = 0
		match.RequiredFit = "confirmed"
		for j := range match.Contributions {
			contribution := &match.Contributions[j]
			if semanticQuality(contribution.Criterion.Criterion) {
				contribution.Assessment = Assessment{CriterionID: contribution.Criterion.ID, Status: "unavailable", EvidenceRefs: []string{}}
				contribution.EffectiveAssessment = "unavailable"
				contribution.Evidence = []Evidence{}
				contribution.Points = 0
			}
			if contribution.Criterion.Strength == "requirement" {
				if contribution.EffectiveAssessment != "supported" && match.RequiredFit != "contradicted" {
					match.RequiredFit = "unconfirmed"
				}
			} else {
				match.Denominator += contribution.Criterion.Weight
				match.Numerator += contribution.Points
			}
		}
		if match.Denominator > 0 {
			match.Score = float64(match.Numerator) / float64(match.Denominator)
		}
		match.NoPreferenceAdvantage = match.Denominator == 0
	}
	return result
}

func semanticQuality(q Criterion) bool {
	return q.AttributeType == "" || q.AttributeType == "natural_light" && q.AttributeValue == "high" || q.AttributeType == "noise_level" && q.AttributeValue == "quiet"
}

// AssessFit classifies evidence and accounts for fit. Retrieval, eligibility and
// final ordering belong to the caller. A provider failure retains unavailable
// assessments rather than claiming the listing omitted information.
func (e *Evaluator) AssessFit(ctx context.Context, req FitRequest) (FitResult, error) {
	if err := ctx.Err(); err != nil {
		return FitResult{}, err
	}
	qs := make([]Criterion, len(req.Criteria))
	ids := map[string]bool{}
	criteria := make([]FitCriterion, len(req.Criteria))
	for i, q := range req.Criteria {
		if q.ID == "" || ids[q.ID] || (q.Strength != "requirement" && q.Strength != "preference") {
			return FitResult{}, fmt.Errorf("matching fit: invalid criterion")
		}
		ids[q.ID] = true
		if q.Weight == 0 {
			q.Weight = 1
		}
		if q.Weight != 1 && q.Weight != 2 {
			return FitResult{}, fmt.Errorf("matching fit: weight must be 1 or 2")
		}
		if q.Weight == 2 && (q.Strength != "preference" || strings.TrimSpace(q.PrioritySource) == "" || !strings.Contains(q.SourceQuote, q.PrioritySource)) {
			return FitResult{}, fmt.Errorf("matching fit: priority needs its source phrase")
		}
		criteria[i] = q
		qs[i] = q.Criterion
		switch {
		case q.AttributeType == "natural_light" && q.AttributeValue == "high":
			qs[i].Text += ". Sólo una afirmación directa sobre la propiedad completa o sus ambientes principales respalda buena luz natural. Poca luz explícita la contradice. Luz de un solo cuarto, frente, orientación o ventanas son indicios insuficientes."
		case q.AttributeType == "noise_level" && q.AttributeValue == "quiet":
			qs[i].Text += ". Sólo una afirmación directa de silencio o ausencia de ruido dentro de la propiedad respalda silencio. Ruido explícito dentro de la propiedad lo contradice. Contrafrente, piso, barrio o doble vidrio aislados son indicios insuficientes."
		}
	}
	ids = map[string]bool{}
	urls := map[string]bool{}
	for _, c := range req.Candidates {
		if c.ID == "" || c.URL == "" || ids[c.ID] || urls[c.URL] {
			return FitResult{}, fmt.Errorf("matching fit: invalid candidate")
		}
		ids[c.ID] = true
		urls[c.URL] = true
		refs := map[string]bool{}
		for _, r := range c.Evidence {
			if r.ID == "" || r.Text == "" || refs[r.ID] || (r.Provenance != "published" && r.Provenance != "stated" && r.Provenance != "inferred") {
				return FitResult{}, fmt.Errorf("matching fit: invalid evidence")
			}
			refs[r.ID] = true
		}
	}
	semantic := []Criterion{}
	positions := []int{}
	for i, q := range qs {
		if semanticQuality(q) {
			semantic = append(semantic, q)
			positions = append(positions, i)
		}
	}
	var batched [][]Assessment
	batchFailure := ""
	if len(semantic) > 0 && len(req.Candidates) > 1 {
		if batch, ok := e.classifier.(BatchClassifier); ok {
			candidates := make([]Candidate, len(req.Candidates))
			for i, c := range req.Candidates {
				candidates[i] = semanticCandidate(c, semantic)
			}
			result, err := batch.ClassifyBatch(ctx, candidates, semantic)
			if ctx.Err() != nil {
				return FitResult{}, ctx.Err()
			}
			if err != nil {
				batchFailure = "provider_error"
			} else if len(result) != len(req.Candidates) {
				batchFailure = "invalid_assessment"
			} else {
				batched = result
			}
		}
	}
	result := FitResult{PolicyVersion: "weighted-net-fit-v1", Matches: []FitMatch{}}
	for _, c := range req.Candidates {
		answers := make([]Assessment, len(qs))
		failure := ""
		for i, q := range qs {
			answers[i] = Assessment{CriterionID: q.ID, Status: "unavailable", EvidenceRefs: []string{}}
			if !semanticQuality(q) {
				exactCandidate := c
				if slices.ContainsFunc(c.Evidence, func(e Evidence) bool { return e.Type == q.AttributeType && e.Provenance != "inferred" }) {
					exactCandidate.Evidence = slices.DeleteFunc(slices.Clone(c.Evidence), func(e Evidence) bool { return e.Type == q.AttributeType && e.Provenance == "inferred" })
				}
				exact, err := (Baseline{}).Classify(ctx, exactCandidate, []Criterion{q})
				if err != nil {
					return FitResult{}, err
				}
				answers[i] = exact[0]
			} else {
				continue
			}
		}
		if len(semantic) > 0 {
			if batchFailure != "" {
				failure = batchFailure
			} else if e.classifier == nil {
				classified, err := (Baseline{}).Classify(ctx, c, semantic)
				if err != nil {
					return FitResult{}, err
				}
				for i, a := range classified {
					hasDescription := slices.ContainsFunc(c.Evidence, func(e Evidence) bool { return e.ID == "description" })
					if a.Assessment == "insufficient_evidence" && hasDescription {
						a.Assessment, a.Status = "", "unavailable"
					}
					if a.Status == "unavailable" {
						failure = "provider_unavailable"
					}
					answers[positions[i]] = a
				}
			} else {
				var classified []Assessment
				var err error
				semanticInput := semanticCandidate(c, semantic)
				if batched != nil {
					classified = batched[len(result.Matches)]
				} else {
					classified, err = e.classifier.Classify(ctx, semanticInput, semantic)
				}
				if ctx.Err() != nil {
					return FitResult{}, ctx.Err()
				}
				if err != nil {
					failure = "provider_error"
				} else {
					classified, err = validateAssessments(semanticInput, semantic, classified)
					if err != nil {
						failure = "invalid_assessment"
					} else {
						for i, a := range classified {
							answers[positions[i]] = a
						}
					}
				}
			}
		}
		m := FitMatch{CandidateID: c.ID, RequiredFit: "confirmed", Contributions: []Contribution{}, Failure: failure}
		evidence := map[string]Evidence{}
		for _, r := range c.Evidence {
			evidence[r.ID] = r
		}
		for i, q := range criteria {
			a := answers[i]
			contribution := Contribution{Criterion: q, Assessment: a, EffectiveAssessment: a.Assessment, Evidence: []Evidence{}}
			direct := false
			for _, ref := range a.EvidenceRefs {
				r := evidence[ref]
				contribution.Evidence = append(contribution.Evidence, r)
				if r.Provenance != "inferred" {
					direct = true
				}
			}
			if a.Status != "evaluated" {
				contribution.EffectiveAssessment = a.Status
			} else if (a.Assessment == "supported" || a.Assessment == "contradicted") && !direct {
				contribution.EffectiveAssessment = "hint"
			}
			outcome := contribution.EffectiveAssessment
			if q.Strength == "requirement" {
				if outcome == "contradicted" {
					m.RequiredFit = "contradicted"
				} else if outcome != "supported" && m.RequiredFit != "contradicted" {
					m.RequiredFit = "unconfirmed"
				}
			} else {
				m.Denominator += q.Weight
				if outcome == "supported" {
					contribution.Points = q.Weight
				}
				if outcome == "contradicted" {
					contribution.Points = -q.Weight
				}
				m.Numerator += contribution.Points
			}
			m.Contributions = append(m.Contributions, contribution)
		}
		if m.Denominator > 0 {
			m.Score = float64(m.Numerator) / float64(m.Denominator)
		}
		m.NoPreferenceAdvantage = m.Denominator == 0
		result.Matches = append(result.Matches, m)
	}
	return result, nil
}

func semanticCandidate(c Candidate, criteria []Criterion) Candidate {
	out := c
	out.Evidence = make([]Evidence, 0, len(c.Evidence))
	for _, e := range c.Evidence {
		keep := e.ID == "description"
		for _, q := range criteria {
			switch q.AttributeType {
			case "natural_light":
				keep = keep || e.Type == "natural_light" || e.Type == "exposure" || e.Type == "orientation"
			case "noise_level":
				keep = keep || e.Type == "noise_level" || e.Type == "exposure" || e.Type == "floor" || e.Type == "glazing"
			default:
				keep = true
			}
		}
		if keep {
			out.Evidence = append(out.Evidence, e)
		}
	}
	return out
}
