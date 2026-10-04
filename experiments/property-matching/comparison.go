package comparison

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

type RecordedAnswer struct {
	Assessment   string   `json:"assessment"`
	Status       string   `json:"status"`
	EvidenceRefs []string `json:"evidence_refs"`
}
type Case struct {
	TargetURL               string                               `json:"target_url,omitempty"`
	Topic                   string                               `json:"topic,omitempty"`
	ExpectedRankingComplete map[string]bool                      `json:"expected_ranking_complete,omitempty"`
	ID                      string                               `json:"id"`
	Split                   string                               `json:"split"`
	Question                string                               `json:"question"`
	Plan                    intake.Plan                          `json:"plan"`
	BranchIDs               []string                             `json:"branch_ids"`
	Criteria                [][]matching.FitCriterion            `json:"criteria"`
	Qualification           eligibility.Qualification            `json:"qualification"`
	Candidates              []eligibility.Candidate              `json:"candidates"`
	SnapshotURLs            []string                             `json:"snapshot_urls,omitempty"`
	Answers                 map[string]map[string]RecordedAnswer `json:"answers"`
	Failures                map[string]string                    `json:"failures,omitempty"`
	Expected                map[string][]string                  `json:"expected"`
}
type Row struct {
	PeerInPacket bool                `json:"comparison_peer_in_packet"`
	Rank         int                 `json:"rank"`
	BranchID     string              `json:"branch_id"`
	Listing      listing.Listing     `json:"listing"`
	Eligibility  eligibility.Verdict `json:"eligibility"`
	Fit          matching.FitMatch   `json:"fit"`
	Reason       Reason              `json:"reason"`
}
type CriterionDifference struct {
	CriterionID string                `json:"criterion_id"`
	Before      matching.Contribution `json:"before"`
	After       matching.Contribution `json:"after"`
}
type Reason struct {
	Differences          []CriterionDifference `json:"criterion_differences,omitempty"`
	Rule                 string                `json:"rule"`
	BeforeURL            string                `json:"before_url"`
	AfterURL             string                `json:"after_url,omitempty"`
	BeforeRank           int                   `json:"before_rank"`
	AfterRank            int                   `json:"after_rank,omitempty"`
	CriterionIDs         []string              `json:"criterion_ids,omitempty"`
	SubstantiveAdvantage bool                  `json:"substantive_advantage"`
}
type Branch struct {
	ID                     string   `json:"id"`
	Neighborhoods          []string `json:"neighborhoods"`
	HardFilterMatches      int      `json:"hard_filter_matches"`
	RetrievalComplete      bool     `json:"retrieval_complete_within_fixture"`
	RankingComplete        bool     `json:"ranking_complete"`
	Confirmed              int      `json:"confirmed"`
	Unconfirmed            int      `json:"unconfirmed"`
	HiddenIneligible       int      `json:"hidden_ineligible"`
	ExcludedContradictions int      `json:"excluded_contradictions"`
	Shown                  []Row    `json:"shown"`
	Excluded               []Row    `json:"excluded"`
}
type Projection struct {
	Branches    []Branch                 `json:"branches"`
	Relaxations []eligibility.Relaxation `json:"relaxations"`
}

// Record owns the final ordering. Projections cannot mutate its evidence,
// contributions or deciding rules.
type Record struct{ sealed []byte }

func (r Record) Project() Projection {
	var out Projection
	// sealed is valid JSON produced once by the checked finalizer. Decoding a
	// fresh value isolates both caller input and every consumer projection.
	_ = json.Unmarshal(r.sealed, &out)
	return out
}

type Comparison struct {
	ID                 string             `json:"id"`
	Split              string             `json:"split"`
	Baseline           buyer.TurnResponse `json:"baseline"`
	BaselinePacket     buyer.Packet       `json:"baseline_packet"`
	BaselineClassifier []ClassifierCall   `json:"baseline_classifier"`
	Proposed           Projection         `json:"proposed"`
	ProposedFallback   string             `json:"proposed_fallback"`
	BaselineWords      int                `json:"baseline_words"`
	ProposedWords      int                `json:"proposed_words"`
	Correct            bool               `json:"literal_expected_order_pass"`
	Findings           []string           `json:"findings"`
}
type ClassifierCall struct {
	Candidate matching.Candidate    `json:"candidate"`
	Criteria  []matching.Criterion  `json:"criteria"`
	Answers   []matching.Assessment `json:"answers"`
	Failure   string                `json:"failure,omitempty"`
}
type frozenClassifier struct {
	c     Case
	calls []ClassifierCall
}

func (f *frozenClassifier) Classify(ctx context.Context, c matching.Candidate, qs []matching.Criterion) ([]matching.Assessment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if failure := f.c.Failures[c.ID]; failure != "" {
		f.calls = append(f.calls, ClassifierCall{Candidate: c, Criteria: qs, Failure: failure})
		return nil, fmt.Errorf("frozen provider failure")
	}
	answers := []matching.Assessment{}
	for _, q := range qs {
		a, ok := f.c.Answers[c.ID][q.AttributeType+"="+q.AttributeValue]
		if !ok {
			return nil, fmt.Errorf("missing frozen answer for %s %s", c.ID, q.ID)
		}
		answers = append(answers, matching.Assessment{CriterionID: q.ID, Assessment: a.Assessment, Status: a.Status, EvidenceRefs: a.EvidenceRefs})
	}
	f.calls = append(f.calls, ClassifierCall{Candidate: c, Criteria: qs, Answers: answers})
	return answers, nil
}

type frozenPlanner struct{ plan intake.Plan }

func (p frozenPlanner) Plan(context.Context, []string, intake.Plan) (intake.Plan, error) {
	return p.plan, nil
}

type recordingWriter struct{ packet buyer.Packet }

func (w *recordingWriter) Write(_ context.Context, p buyer.Packet, _ func(string)) (string, error) {
	w.packet = p
	return "", fmt.Errorf("recording baseline fallback")
}

// RunBaseline executes the current buyer Agent, including its real fallback.
func RunBaseline(ctx context.Context, c Case) (buyer.TurnResponse, buyer.Packet, []ClassifierCall, error) {
	writer := &recordingWriter{}
	classifier := &frozenClassifier{c: c}
	agent := buyer.NewAgent(frozenPlanner{c.Plan}, inventory{c.Candidates}, writer, buyer.WithMatching(classifier))
	result, err := agent.HandleMessage(ctx, "comparison", c.Question, c.Qualification, buyer.Events{})
	if err != nil {
		return buyer.TurnResponse{}, buyer.Packet{}, nil, err
	}
	return *result, writer.packet, classifier.calls, nil
}

// RunProposed preserves branch membership and each branch's own denominator.
// It never pools incompatible rubrics or applies the baseline's global cap.
func RunProposed(ctx context.Context, c Case) (Record, error) {
	if err := validateCase(c); err != nil {
		return Record{}, err
	}
	if len(c.Plan.Branches) != len(c.Criteria) || len(c.BranchIDs) != len(c.Criteria) {
		return Record{}, fmt.Errorf("comparison: mismatched branch metadata")
	}
	for _, criteria := range c.Criteria {
		for _, criterion := range criteria {
			if criterion.Weight == 2 && (criterion.PrioritySource == "" || !strings.Contains(c.Question, criterion.PrioritySource)) {
				return Record{}, fmt.Errorf("comparison: priority source not in original question")
			}
		}
	}
	if c.Plan.Sort == "price_asc" || c.Plan.Sort == "price_desc" {
		for _, q := range c.Plan.Branches {
			if q.Currency == "" {
				return Record{}, fmt.Errorf("comparison: price ordering needs an explicit currency")
			}
		}
	}
	if err := validateFrozen(c); err != nil {
		return Record{}, err
	}
	qualification := eligibility.Qualification{}
	for _, source := range []eligibility.Qualification{c.Qualification, c.Plan.Qualification} {
		for fact, values := range source {
			for _, value := range values {
				if !slices.Contains(qualification[fact], value) {
					qualification[fact] = append(qualification[fact], value)
				}
			}
		}
	}
	out := Projection{Branches: []Branch{}, Relaxations: []eligibility.Relaxation{}}
	classifier := &frozenClassifier{c: c}
	all := map[string]eligibility.Candidate{}
	for i, q := range c.Plan.Branches {
		// Criteria are assessed after published numeric/location gates. Remove only
		// the attributes this branch explicitly owns, preserving all other gates.
		query := q
		query.RequiredAttributes = slices.DeleteFunc(slices.Clone(q.RequiredAttributes), func(f search.AttributeFilter) bool {
			return slices.ContainsFunc(c.Criteria[i], func(k matching.FitCriterion) bool {
				return k.AttributeType == f.Type && k.AttributeValue == f.Value && k.Strength == "requirement"
			})
		})
		candidates, err := (inventory{c.Candidates}).Candidates(ctx, query)
		if err != nil {
			return Record{}, err
		}
		branch := Branch{ID: c.BranchIDs[i], Neighborhoods: q.Neighborhoods, HardFilterMatches: len(candidates), RetrievalComplete: true, RankingComplete: true, Shown: []Row{}, Excluded: []Row{}}
		req := matching.FitRequest{Criteria: c.Criteria[i], Candidates: []matching.Candidate{}}
		for _, candidate := range candidates {
			req.Candidates = append(req.Candidates, matchingCandidate(candidate.Listing))
		}
		fit, err := matching.New(classifier).AssessFit(ctx, req)
		if err != nil {
			return Record{}, err
		}
		for j, candidate := range candidates {
			if fit.Matches[j].RequiredFit != "contradicted" {
				all[candidate.Listing.URL] = candidate
			}
			row := Row{BranchID: branch.ID, Listing: candidate.Listing, Eligibility: eligibility.Assess(qualification, candidate.Listing.Price, candidate.Rules, catalog), Fit: fit.Matches[j]}
			if row.Fit.Failure != "" {
				branch.RankingComplete = false
			}
			switch {
			case row.Eligibility.State == eligibility.Ineligible:
				branch.HiddenIneligible++
				row.Reason.Rule = "ineligible"
				branch.Excluded = append(branch.Excluded, row)
			case row.Fit.RequiredFit == "contradicted":
				branch.ExcludedContradictions++
				row.Reason.Rule = "required_contradiction"
				branch.Excluded = append(branch.Excluded, row)
			default:
				if row.Fit.RequiredFit == "confirmed" {
					branch.Confirmed++
				} else {
					branch.Unconfirmed++
				}
				branch.Shown = append(branch.Shown, row)
			}
		}
		finalize(&branch, c.Plan.Sort)
		out.Branches = append(out.Branches, branch)
	}
	allRows := []eligibility.Candidate{}
	for _, v := range all {
		allRows = append(allRows, v)
	}
	slices.SortFunc(allRows, func(a, b eligibility.Candidate) int { return cmp.Compare(a.Listing.URL, b.Listing.URL) })
	out.Relaxations = eligibility.Relaxations(qualification, allRows, catalog)
	data, err := json.Marshal(out)
	if err != nil {
		return Record{}, fmt.Errorf("comparison: seal record: %w", err)
	}
	return Record{sealed: data}, nil
}
func matchingCandidate(item listing.Listing) matching.Candidate {
	c := matching.Candidate{ID: item.URL, URL: item.URL, Evidence: []matching.Evidence{}}
	if item.Description != "" {
		c.Evidence = append(c.Evidence, matching.Evidence{ID: "description", Text: item.Description, Provenance: "published"})
	}
	for i, a := range item.Attributes {
		if a.Evidence != "" {
			id := fmt.Sprintf("attribute:%d", i)

			c.Evidence = append(c.Evidence, matching.Evidence{ID: id, Text: a.Evidence, Provenance: string(a.Provenance), Type: a.Type, Value: a.Value})
		}
	}
	return c
}

func Run(ctx context.Context, c Case) (Comparison, error) {
	baseline, packet, calls, err := RunBaseline(ctx, c)
	if err != nil {
		return Comparison{}, err
	}
	record, err := RunProposed(ctx, c)
	if err != nil {
		return Comparison{}, err
	}
	fallback := record.Fallback(c.TargetURL, c.Topic)
	result := Comparison{ID: c.ID, Split: c.Split, Baseline: baseline, BaselinePacket: packet, BaselineClassifier: calls, Proposed: record.Project(), ProposedFallback: fallback, BaselineWords: wordCount(baseline.Reply), ProposedWords: wordCount(fallback), Correct: true, Findings: []string{}}
	for _, b := range result.Proposed.Branches {
		expectedComplete := true
		if v, ok := c.ExpectedRankingComplete[b.ID]; ok {
			expectedComplete = v
		}
		if b.RankingComplete != expectedComplete {
			result.Correct = false
			result.Findings = append(result.Findings, fmt.Sprintf("%s ranking complete expected %t got %t", b.ID, expectedComplete, b.RankingComplete))
		}
		urls := []string{}
		for _, row := range b.Shown {
			urls = append(urls, row.Listing.URL)
		}
		if !reflect.DeepEqual(urls, c.Expected[b.ID]) {
			result.Correct = false
			result.Findings = append(result.Findings, fmt.Sprintf("%s expected %v got %v", b.ID, c.Expected[b.ID], urls))
		}
		for _, row := range b.Excluded {
			if row.Fit.RequiredFit == "contradicted" && slices.ContainsFunc(baseline.Listings, func(r buyer.Result) bool { return r.URL == row.Listing.URL }) {
				result.Findings = append(result.Findings, "baseline shows mandatory contradiction "+row.Listing.URL)
			}
		}
		for _, row := range b.Shown {
			old := 0
			for _, r := range baseline.Listings {
				if r.URL == row.Listing.URL {
					old = r.Rank
				}
			}
			if old != row.Rank {
				result.Findings = append(result.Findings, fmt.Sprintf("%s %s baseline global rank %d -> proposed branch rank %d; deciding rule %s", b.ID, row.Listing.URL, old, row.Rank, row.Reason.Rule))
			}
		}
	}
	return result, nil
}

// compareRows is the single ordering rule used by sort and consecutive reasons.
func compareRows(a, b Row, userSort string) (int, string) {
	if n := cmp.Compare(qualityGroup(a), qualityGroup(b)); n != 0 {
		return n, "required_evidence"
	}
	section := map[eligibility.State]int{eligibility.Eligible: 0, eligibility.ConditionallyEligible: 1, eligibility.Unknown: 2}
	if n := cmp.Compare(section[a.Eligibility.State], section[b.Eligibility.State]); n != 0 {
		return n, "eligibility"
	}
	switch userSort {
	case "price_asc", "price_desc":
		dir := 1
		if userSort == "price_desc" {
			dir = -1
		}
		if n := numeric(a.Listing.Price.Amount, b.Listing.Price.Amount, dir); n != 0 {
			return n, userSort
		}
	case "area_desc":
		if n := numeric(a.Listing.TotalAreaM2, b.Listing.TotalAreaM2, -1); n != 0 {
			return n, userSort
		}
	default:
		if n := cmp.Compare(b.Fit.Score, a.Fit.Score); n != 0 {
			return n, "preference_score"
		}
	}
	return cmp.Compare(a.Listing.URL, b.Listing.URL), "stable_tie"
}
func numeric(a, b *float64, dir int) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}
	return dir * cmp.Compare(*a, *b)
}
func finalize(branch *Branch, sort string) {
	slices.SortFunc(branch.Shown, func(a, b Row) int { n, _ := compareRows(a, b, sort); return n })
	for i := range branch.Shown {
		row := &branch.Shown[i]
		row.Rank = i + 1
		reason := Reason{Rule: "lone_fit", BeforeURL: row.Listing.URL, BeforeRank: row.Rank}
		if i+1 < len(branch.Shown) {
			next := branch.Shown[i+1]
			_, reason.Rule = compareRows(*row, next, sort)
			reason.AfterURL = next.Listing.URL
			reason.AfterRank = i + 2
			reason.SubstantiveAdvantage = reason.Rule != "stable_tie"
			if reason.Rule == "preference_score" {
				for j, c := range row.Fit.Contributions {
					if j < len(next.Fit.Contributions) && c.Points != next.Fit.Contributions[j].Points {
						reason.CriterionIDs = append(reason.CriterionIDs, c.Criterion.ID)
						reason.Differences = append(reason.Differences, CriterionDifference{CriterionID: c.Criterion.ID, Before: c, After: next.Fit.Contributions[j]})
					}
				}
			}
		} else if len(branch.Shown) > 1 {
			reason.Rule = "last_fit"
		}
		row.Reason = reason
	}
}

func qualityGroup(r Row) int {
	if r.Fit.RequiredFit == "confirmed" {
		return 0
	}
	return 1
}

func validateCase(c Case) error {
	for _, candidate := range c.Candidates {
		l := candidate.Listing
		for _, v := range []*float64{l.Price.Amount, l.Expenses.Amount, l.TotalAreaM2, l.CoveredAreaM2} {
			if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0)) {
				return fmt.Errorf("comparison: non-finite published value")
			}
		}
	}
	switch c.Plan.Sort {
	case "", "relevance", "price_asc", "price_desc", "area_desc":
	default:
		return fmt.Errorf("comparison: invalid sort")
	}
	ids := map[string]bool{}
	for _, id := range c.BranchIDs {
		if id == "" || ids[id] {
			return fmt.Errorf("comparison: invalid branch ID")
		}
		ids[id] = true
	}
	return nil
}

func validateFrozen(c Case) error {
	for i, q := range c.Plan.Branches {
		query := q
		query.RequiredAttributes = slices.DeleteFunc(slices.Clone(q.RequiredAttributes), func(f search.AttributeFilter) bool {
			return slices.ContainsFunc(c.Criteria[i], func(k matching.FitCriterion) bool {
				return k.AttributeType == f.Type && k.AttributeValue == f.Value && k.Strength == "requirement"
			})
		})
		candidates, err := (inventory{c.Candidates}).Candidates(context.Background(), query)
		if err != nil {
			return err
		}
		for _, candidate := range candidates {
			id := candidate.Listing.URL
			if c.Failures[id] != "" {
				if c.Failures[id] != "provider_error" {
					return fmt.Errorf("comparison: unsupported simulated failure")
				}
				continue
			}
			evidence := matchingCandidate(candidate.Listing).Evidence
			for _, criterion := range c.Criteria[i] {
				if criterion.AttributeType != "" && criterion.AttributeType != "natural_light" && criterion.AttributeType != "noise_level" {
					continue
				}
				answer, ok := c.Answers[id][criterion.AttributeType+"="+criterion.AttributeValue]
				if !ok {
					return fmt.Errorf("comparison: missing frozen assessment for %s %s", id, criterion.ID)
				}
				if answer.Status != "evaluated" && answer.Status != "needs_review" {
					return fmt.Errorf("comparison: invalid frozen assessment status")
				}
				switch answer.Assessment {
				case "supported", "contradicted", "insufficient_evidence", "conflicting_evidence":
				default:
					return fmt.Errorf("comparison: invalid frozen assessment")
				}
				if answer.Status == "evaluated" && answer.Assessment != "insufficient_evidence" && len(answer.EvidenceRefs) == 0 {
					return fmt.Errorf("comparison: frozen assessment lacks evidence")
				}
				seen := map[string]bool{}
				for _, ref := range answer.EvidenceRefs {
					if seen[ref] || !slices.ContainsFunc(evidence, func(e matching.Evidence) bool { return e.ID == ref }) {
						return fmt.Errorf("comparison: invalid frozen evidence reference %s", ref)
					}
					seen[ref] = true
				}
			}
		}
	}
	return nil
}
