package buyer

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/clarification"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/matching"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

// Planner turns the conversation into a plan; intake.Planner fits.
type Planner interface {
	Plan(ctx context.Context, turns []string, previous intake.Plan) (intake.Plan, error)
}

// Inventory is the read side the pipeline needs; postgres.Store fits.
type Inventory interface {
	Candidates(ctx context.Context, query search.Query) ([]eligibility.Candidate, error)
	Facts(ctx context.Context) (eligibility.Catalog, error)
}

// Writer renders a reply after ranking. It cannot search or reorder. reply,
// when not nil, receives the text as it is written.
type Writer interface {
	Write(ctx context.Context, p Packet, reply func(delta string)) (string, error)
}

// Packet is everything the writer may use, and nothing else.
type Packet struct {
	Intent       string                   `json:"intent"`
	Question     string                   `json:"question"`
	Sort         string                   `json:"sort"`
	Requirements []Requirement            `json:"requirements"`
	Branches     []BranchReport           `json:"branches"`
	Shown        []Result                 `json:"shown"`
	Hidden       int                      `json:"hidden_ineligible"`
	Relaxations  []eligibility.Relaxation `json:"relaxations,omitempty"`
}

// BranchReport keeps an empty branch visible ("Caballito: 0 avisos").
type BranchReport struct {
	Neighborhoods  []string `json:"neighborhoods"`
	CandidateCount int      `json:"candidate_count"`
	// Matches confirm required qualitative criteria. Unconfirmed listings are
	// shown after them; Contradicted listings are withheld.
	Matches      int `json:"matches"`
	Unconfirmed  int `json:"unconfirmed,omitempty"`
	Contradicted int `json:"contradicted,omitempty"`
}

type pipeline struct {
	planner   Planner
	inventory Inventory
	writer    Writer
	matcher   matching.Classifier
	clarifier clarification.Proposer
}

// WithClarifier adds a generative clarification pass before search.
func WithClarifier(proposer clarification.Proposer) Option {
	return func(agent *DefaultAgent) { agent.pipeline.clarifier = proposer }
}

var ErrPendingClarification = errors.New("a clarification is pending")
var ErrStaleClarification = errors.New("stale clarification answer")

// WithMatching supplies the grounded qualitative assessor. A missing or
// unavailable assessor leaves a prose-only requirement unconfirmed.
func WithMatching(classifier matching.Classifier) Option {
	return func(agent *DefaultAgent) { agent.pipeline.matcher = classifier }
}

// maxShown matches the old search page size.
const maxShown = 10
const maxFitBatch = 4

var section = map[eligibility.State]int{eligibility.Eligible: 0, eligibility.ConditionallyEligible: 1, eligibility.Unknown: 2}

func (a *DefaultAgent) handlePipeline(ctx context.Context, sessionID, message string, declared eligibility.Qualification, events Events, resumed *pendingClarification) (*TurnResponse, error) {
	p := a.pipeline
	logger := logging.FromContext(ctx)
	turnStarted := time.Now()
	turnOutcome := "error"
	plannerName := "unknown"
	writerOutcome := "not_started"
	shown, hidden := 0, 0
	defer func() {
		logger.LogAttrs(ctx, slog.LevelInfo, "buyer_turn",
			slog.String("outcome", turnOutcome), slog.String("planner", plannerName),
			slog.String("writer", writerOutcome), slog.Int("shown", shown), slog.Int("hidden", hidden),
			slog.Int64("duration_ms", time.Since(turnStarted).Milliseconds()))
	}()
	a.mu.Lock()
	sess, ok := a.sessions[sessionID]
	if !ok {
		sess = &session{id: sessionID}
		a.sessions[sessionID] = sess
	}
	if (sess.pending != nil || sess.inFlight) && resumed == nil {
		a.mu.Unlock()
		return nil, ErrPendingClarification
	}
	sess.inFlight = true
	defer func() { a.mu.Lock(); sess.inFlight = false; a.mu.Unlock() }()
	turns := append(slices.Clone(sess.turns), message)
	previous := sess.plan
	confirmed := slices.Clone(sess.confirmed)
	if resumed != nil {
		turns = slices.Clone(resumed.turns)
	}
	a.mu.Unlock()

	var plan intake.Plan
	var err error
	planStarted := time.Now()
	if resumed != nil {
		plan = resumed.plan
	} else {
		plan, err = p.planner.Plan(ctx, turns, previous)
		if plan.PlannedBy != "" {
			plannerName = plan.PlannedBy
		}
		if err == nil && plan.Intent != "ask_about_listing" && len(confirmed) > 0 {
			plan, confirmed = preserveConfirmed(plan, confirmed, message)
			a.mu.Lock()
			sess.confirmed = confirmed
			a.mu.Unlock()
		}
		if err != nil && p.clarifier == nil {
			logger.LogAttrs(ctx, slog.LevelWarn, "buyer_plan", slog.String("outcome", "error"),
				slog.String("error_class", logging.ErrorClass(err)), slog.Int64("duration_ms", time.Since(planStarted).Milliseconds()))
			return nil, fmt.Errorf("buyer: plan: %w", err)
		}
		if p.clarifier != nil && (plan.Intent == "new_search" || plan.Intent == "refine" || err != nil) {
			question, blocked, qerr := a.prepareClarification(ctx, sess, turns, plan, declared, err)
			if qerr != nil {
				return nil, qerr
			}
			if blocked {
				turnOutcome = "clarification"
				return &TurnResponse{Clarification: question}, nil
			}
		}
		if err != nil {
			return nil, fmt.Errorf("buyer: plan: %w", err)
		}
	}
	plannerName = plan.PlannedBy
	if plannerName == "" {
		plannerName = "unspecified"
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "buyer_plan", slog.String("outcome", "success"),
		slog.String("planner", plannerName), slog.Int("branches", len(plan.Branches)),
		slog.Int64("duration_ms", time.Since(planStarted).Milliseconds()))
	a.mu.RLock()
	declined := sess.declinedFacts
	a.mu.RUnlock()
	if len(declined) > 0 {
		clean := eligibility.Qualification{}
		for fact, values := range declared {
			if !declined[fact] {
				clean[fact] = slices.Clone(values)
			}
		}
		declared = clean
		for fact := range declined {
			delete(plan.Qualification, fact)
		}
	}
	q := mergeQualification(declared, plan.Qualification)

	packet := Packet{Intent: plan.Intent, Question: message, Sort: plan.Sort}
	var results []Result
	var relaxations []eligibility.Relaxation
	searchStarted := time.Now()
	if len(plan.Branches) == 0 {
		// Nothing to search (a greeting): keep what is already on screen.
		a.mu.RLock()
		results = sess.results
		a.mu.RUnlock()
	} else {
		catalog, err := p.inventory.Facts(ctx)
		if err != nil {
			logger.LogAttrs(ctx, slog.LevelWarn, "buyer_search", slog.String("outcome", "error"),
				slog.String("error_class", logging.ErrorClass(err)), slog.Int64("duration_ms", time.Since(searchStarted).Milliseconds()))
			return nil, err
		}
		results, relaxations, packet.Branches, packet.Hidden, err = p.rank(ctx, plan, q, catalog)
		if err != nil {
			logger.LogAttrs(ctx, slog.LevelWarn, "buyer_search", slog.String("outcome", "error"),
				slog.String("error_class", logging.ErrorClass(err)), slog.Int64("duration_ms", time.Since(searchStarted).Milliseconds()))
			return nil, err
		}
		for _, b := range plan.Branches {
			packet.Requirements = append(packet.Requirements, requirementsFromQuery(b)...)
		}
		packet.Requirements = slices.CompactFunc(sortedRequirements(packet.Requirements), func(a, b Requirement) bool { return a == b })
	}
	shown, hidden = len(results), packet.Hidden
	searchOutcome := "success"
	if len(plan.Branches) == 0 {
		searchOutcome = "skipped"
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "buyer_search", slog.String("outcome", searchOutcome),
		slog.Int("branches", len(plan.Branches)), slog.Int("shown", shown), slog.Int("hidden", hidden),
		slog.Int("relaxations", len(relaxations)), slog.Int64("duration_ms", time.Since(searchStarted).Milliseconds()))
	packet.Relaxations = relaxations
	packet.Shown = results
	if plan.Intent != "ask_about_listing" && len(results) > 3 {
		packet.Shown = results[:3]
	}

	if events.Results != nil {
		events.Results(TurnResponse{Requirements: packet.Requirements, Listings: results, Relaxations: relaxations})
	}
	writerStarted := time.Now()
	reply, err := p.writer.Write(ctx, packet, events.Reply)
	writerOutcome = "rendered"
	if err != nil {
		// The ranking is already done; a writer outage must not lose it.
		reply = templateReply(packet)
		writerOutcome = "fallback"
	}
	writerLevel := slog.LevelInfo
	if writerOutcome == "fallback" {
		writerLevel = slog.LevelWarn
	}
	logger.LogAttrs(ctx, writerLevel, "buyer_writer", slog.String("outcome", writerOutcome),
		slog.Int64("duration_ms", time.Since(writerStarted).Milliseconds()))

	a.mu.Lock()
	sess.turns, sess.plan, sess.results = turns, plan, results
	a.mu.Unlock()
	turnOutcome = "success"
	return &TurnResponse{Reply: reply, Requirements: packet.Requirements, Listings: results, Relaxations: relaxations}, nil
}

type scored struct {
	result    Result
	candidate eligibility.Candidate
	section   int
	branch    int
	fit       matching.FitMatch
}

// rank retrieves every branch, assesses each listing and orders the merged
// list by confirmed mandatory qualities, rental eligibility, then explicit
// user sort or evidence-backed preference fit.
func (p *pipeline) rank(ctx context.Context, plan intake.Plan, q eligibility.Qualification, catalog eligibility.Catalog) ([]Result, []eligibility.Relaxation, []BranchReport, int, error) {
	logger := logging.FromContext(ctx)
	var relaxable []eligibility.Candidate
	var rows []scored
	var reports []BranchReport
	seen := map[string]bool{}
	seenRelaxable := map[string]bool{}
	hidden := 0
	for branchIndex, branch := range plan.Branches {
		storedQuery, criteria := separateQualitative(branch)
		candidateStarted := time.Now()
		candidates, err := p.inventory.Candidates(ctx, storedQuery)
		if err != nil {
			logger.LogAttrs(ctx, slog.LevelWarn, "buyer_candidates", slog.String("outcome", "error"),
				slog.Int("branch", branchIndex), slog.String("error_class", logging.ErrorClass(err)),
				slog.Int64("duration_ms", time.Since(candidateStarted).Milliseconds()))
			return nil, nil, nil, 0, err
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "buyer_candidates", slog.String("outcome", "success"),
			slog.Int("branch", branchIndex), slog.Int("count", len(candidates)),
			slog.Int64("duration_ms", time.Since(candidateStarted).Milliseconds()))
		eligibilityStarted := time.Now()
		previousHidden := hidden
		branchRows := []scored{}
		fitCandidates := []matching.Candidate{}
		branchSeen := map[string]bool{}
		for _, c := range candidates {
			if branchSeen[c.Listing.URL] {
				continue
			}
			branchSeen[c.Listing.URL] = true
			verdict := eligibility.Assess(q, c.Listing.Price, c.Rules, catalog)
			if !seen[c.Listing.URL] && verdict.State == eligibility.Ineligible {
				hidden++
			}
			seen[c.Listing.URL] = true
			branchRows = append(branchRows, scored{result: Result{Listing: c.Listing, Eligibility: &verdict, Matched: matched(c.Listing, branch)}, candidate: c, section: section[verdict.State]})
			fitCandidates = append(fitCandidates, matchingCandidate(c.Listing))
		}
		fitStarted := time.Now()
		fitResult := matching.FitResult{Matches: []matching.FitMatch{}}
		for start := 0; start < len(fitCandidates); start += maxFitBatch {
			end := min(start+maxFitBatch, len(fitCandidates))
			candidateCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			batch, err := matching.New(p.matcher).AssessFit(candidateCtx, matching.FitRequest{Criteria: criteria, Candidates: fitCandidates[start:end]})
			cancel()
			if err != nil {
				if ctx.Err() != nil || !errors.Is(err, context.DeadlineExceeded) {
					return nil, nil, nil, 0, fmt.Errorf("buyer: assess fit: %w", err)
				}
				batch = matching.UnavailableFit(matching.FitRequest{Criteria: criteria, Candidates: fitCandidates[start:end]}, "assessment_timeout")
			}
			fitResult.Matches = append(fitResult.Matches, batch.Matches...)
		}
		confirmed, unconfirmed, contradicted := 0, 0, 0
		for i := range branchRows {
			branchRows[i].fit = fitResult.Matches[i]
			switch branchRows[i].fit.RequiredFit {
			case "contradicted":
				if branchRows[i].result.Eligibility.State != eligibility.Ineligible {
					contradicted++
				}
				continue
			case "unconfirmed":
				branchRows[i].result.QualitativeFit = "unconfirmed"
			default:
				if len(criteria) > 0 && hasRequirement(criteria) {
					branchRows[i].result.QualitativeFit = "exact"
				}
			}
			if !seenRelaxable[branchRows[i].candidate.Listing.URL] {
				seenRelaxable[branchRows[i].candidate.Listing.URL] = true
				relaxable = append(relaxable, branchRows[i].candidate)
			}
			if branchRows[i].fit.RequiredFit == "unconfirmed" {
				unconfirmed++
			} else {
				confirmed++
			}
			if branchRows[i].result.Eligibility.State == eligibility.Ineligible {
				continue
			}
			if !slices.ContainsFunc(rows, func(existing scored) bool { return existing.result.URL == branchRows[i].result.URL }) {
				branchRows[i].branch = branchIndex
				rows = append(rows, branchRows[i])
			}
		}
		reports = append(reports, BranchReport{Neighborhoods: branch.Neighborhoods, CandidateCount: len(branchRows), Matches: confirmed, Unconfirmed: unconfirmed, Contradicted: contradicted})
		if len(criteria) > 0 {
			outcome := "success"
			for _, m := range fitResult.Matches {
				if m.Failure != "" {
					outcome = "partial_fallback"
				}
			}
			if outcome == "success" && len(fitCandidates) == 0 {
				outcome = "empty"
			}
			logger.LogAttrs(ctx, slog.LevelInfo, "buyer_qualitative", slog.String("outcome", outcome),
				slog.Int("candidates", len(fitCandidates)), slog.Int("contradicted", contradicted),
				slog.Int64("duration_ms", time.Since(fitStarted).Milliseconds()))
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "buyer_eligibility", slog.Int("branch", branchIndex),
			slog.Int("count", len(candidates)), slog.Int("hidden", hidden-previousHidden),
			slog.Int64("duration_ms", time.Since(eligibilityStarted).Milliseconds()))
	}
	slices.SortFunc(rows, func(a, b scored) int { n, _ := compareScored(a, b, plan.Sort); return n })
	if len(rows) > maxShown {
		rows = rows[:maxShown]
	}
	results := make([]Result, len(rows))
	for i, r := range rows {
		results[i] = r.result
		results[i].Rank = i + 1
		if i+1 < len(rows) {
			_, results[i].orderReason = compareScored(r, rows[i+1], plan.Sort)
			results[i].orderReason += ":" + rows[i+1].result.URL
		} else {
			results[i].orderReason = "last"
		}
		results[i].fit = r.fit
	}
	return results, eligibility.Relaxations(q, relaxable, catalog), reports, hidden, nil
}

// Deterministic required attributes stay in SQL. Qualities without a
// deterministic gate and every preference become evidence-scored criteria.
func separateQualitative(q search.Query) (search.Query, []matching.FitCriterion) {
	criteria := []matching.FitCriterion{}
	required := q.RequiredAttributes[:0:0]
	for _, f := range q.RequiredAttributes {
		if fitRequirement(f) {
			if !hasAttributeCriterion(criteria, f) {
				criteria = append(criteria, makeFitCriterion(f, "requirement", len(criteria)))
			}
		} else {
			required = append(required, f)
		}
	}
	q.RequiredAttributes = required
	for _, f := range q.PreferredAttributes {
		if !hasAttributeCriterion(criteria, f) {
			criteria = append(criteria, makeFitCriterion(f, "preference", len(criteria)))
		}
	}
	return q, criteria
}

func hasAttributeCriterion(criteria []matching.FitCriterion, filter search.AttributeFilter) bool {
	return slices.ContainsFunc(criteria, func(c matching.FitCriterion) bool {
		return c.AttributeType == filter.Type && c.AttributeValue == filter.Value
	})
}

func fitRequirement(f search.AttributeFilter) bool {
	switch f.Type {
	case "natural_light", "noise_level", "amenity":
		return true
	default:
		return false
	}
}

func makeFitCriterion(f search.AttributeFilter, strength string, index int) matching.FitCriterion {
	text := fitLabel(f)
	return matching.FitCriterion{Criterion: matching.Criterion{ID: fmt.Sprintf("q%d:%s:%s", index, f.Type, f.Value), Text: text, Priority: "primary", AttributeType: f.Type, AttributeValue: f.Value}, Strength: strength, Weight: 1}
}

func fitLabel(f search.AttributeFilter) string {
	switch f.Type {
	case "natural_light":
		if f.Value == "low" {
			return "poca luz natural"
		}
		return "buena luz natural"
	case "noise_level":
		if f.Value == "noisy" {
			return "ruido"
		}
		if f.Value == "moderate" {
			return "ruido moderado"
		}
		return "silencio"
	case "amenity", "outdoor_space":
		return "que tiene " + fitValueLabel(f.Value)
	case "furnished":
		if f.Value == "no" {
			return "que no está amueblado"
		}
		return "que está amueblado"
	case "pets_allowed":
		if f.Value == "no" {
			return "que no acepta mascotas"
		}
		return "que acepta mascotas"
	case "air_conditioning":
		if f.Value == "no" {
			return "que no tiene aire acondicionado"
		}
		return "que tiene aire acondicionado"
	case "transit_access":
		return "acceso a " + fitValueLabel(f.Value)
	default:
		return f.Value
	}
}

func fitValueLabel(value string) string {
	switch value {
	case "balcon":
		return "balcón"
	case "subte_a", "subte_b", "subte_c", "subte_d", "subte_e", "subte_h":
		return "la línea " + strings.TrimPrefix(value, "subte_")
	case "tren":
		return "tren"
	case "colectivo":
		return "colectivo"
	default:
		return value
	}
}

func hasRequirement(criteria []matching.FitCriterion) bool {
	return slices.ContainsFunc(criteria, func(c matching.FitCriterion) bool { return c.Strength == "requirement" })
}

func matchingCandidate(item listing.Listing) matching.Candidate {
	candidate := matching.Candidate{ID: item.URL, URL: item.URL, Evidence: []matching.Evidence{}}
	if item.Description != "" {
		candidate.Evidence = append(candidate.Evidence, matching.Evidence{ID: "description", Text: item.Description, Provenance: "published"})
	}
	for i, a := range item.Attributes {
		if a.Provenance != listing.Stated && a.Provenance != listing.Inferred {
			continue
		}
		text := a.Evidence
		if text == "" {
			if a.Type == "natural_light" || a.Type == "noise_level" {
				continue
			}
			text = fmt.Sprintf("ficha: %s=%s", a.Type, a.Value)
		}
		candidate.Evidence = append(candidate.Evidence, matching.Evidence{ID: fmt.Sprintf("attribute:%d", i), Text: text, Provenance: string(a.Provenance), Type: a.Type, Value: a.Value})
	}
	return candidate
}

func compareScored(a, b scored, userSort string) (int, string) {
	if a.branch != b.branch {
		return cmp.Compare(a.branch, b.branch), "other_branch"
	}
	group := func(r scored) int {
		if r.fit.RequiredFit == "unconfirmed" {
			return 1
		}
		return 0
	}
	if n := cmp.Compare(group(a), group(b)); n != 0 {
		return n, "required_evidence"
	}
	if n := cmp.Compare(a.section, b.section); n != 0 {
		return n, "eligibility"
	}
	var n int
	switch userSort {
	case "price_asc":
		n = compareMissingLast(a.result.Price.Amount, b.result.Price.Amount, 1)
	case "price_desc":
		n = compareMissingLast(a.result.Price.Amount, b.result.Price.Amount, -1)
	case "area_desc":
		n = compareMissingLast(a.result.TotalAreaM2, b.result.TotalAreaM2, -1)
	default:
		n = cmp.Compare(b.fit.Score, a.fit.Score)
		if n != 0 {
			return n, "preference_score"
		}
	}
	if n != 0 {
		return n, userSort
	}
	return cmp.Compare(a.result.URL, b.result.URL), "stable_tie"
}

// matched returns the listing's attributes that answer a quality the branch
// asks for, required or preferred, in the listing's order.
func matched(l listing.Listing, q search.Query) []listing.Attribute {
	var out []listing.Attribute
	for _, a := range l.Attributes {
		asked := func(f search.AttributeFilter) bool { return f.Type == a.Type && f.Value == a.Value }
		if slices.ContainsFunc(q.RequiredAttributes, asked) || slices.ContainsFunc(q.PreferredAttributes, asked) {
			out = append(out, a)
		}
	}
	return out
}

// compareMissingLast orders by value in direction dir; an unpublished value
// sorts last either way.
func compareMissingLast(a, b *float64, dir int) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	}
	return dir * cmp.Compare(*a, *b)
}

// mergeQualification adds what the searcher said in the chat to what they
// declared in the form or their account.
func mergeQualification(declared, chat eligibility.Qualification) eligibility.Qualification {
	out := eligibility.Qualification{}
	for _, q := range []eligibility.Qualification{declared, chat} {
		for fact, values := range q {
			for _, v := range values {
				if !slices.Contains(out[fact], v) {
					out[fact] = append(out[fact], v)
				}
			}
		}
	}
	return out
}

func sortedRequirements(r []Requirement) []Requirement {
	sortRequirements(r)
	return r
}
