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
		if q.AttributeType == "natural_light" && q.AttributeValue != "high" || q.AttributeType == "noise_level" && q.AttributeValue != "quiet" {
			return FitResult{}, fmt.Errorf("matching fit: prototype supports natural_light=high and noise_level=quiet only")
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
		switch q.AttributeType {
		case "natural_light":
			qs[i].Text += ". Sólo una afirmación directa sobre la propiedad completa o sus ambientes principales respalda buena luz natural. Poca luz explícita la contradice. Luz de un solo cuarto, frente, orientación o ventanas son indicios insuficientes."
		case "noise_level":
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
	result := FitResult{PolicyVersion: "weighted-net-fit-v1", Matches: []FitMatch{}}
	for _, c := range req.Candidates {
		answers := make([]Assessment, len(qs))
		failure := ""
		semantic := []Criterion{}
		positions := []int{}
		for i, q := range qs {
			answers[i] = Assessment{CriterionID: q.ID, Status: "unavailable", EvidenceRefs: []string{}}
			if q.AttributeType != "" && q.AttributeType != "natural_light" && q.AttributeType != "noise_level" {
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
				semantic = append(semantic, q)
				positions = append(positions, i)
			}
		}
		if len(semantic) > 0 {
			if e.classifier == nil {
				failure = "provider_unavailable"
			} else {
				classified, err := e.classifier.Classify(ctx, c, semantic)
				if ctx.Err() != nil {
					return FitResult{}, ctx.Err()
				}
				if err != nil {
					failure = "provider_error"
				} else {
					classified, err = validateAssessments(c, semantic, classified)
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
