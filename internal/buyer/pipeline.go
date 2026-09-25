package buyer

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
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
	AdmissibleFacts(ctx context.Context) (map[string]bool, error)
}

// Writer explains a finished ranking. It cannot search or reorder. reply,
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
	Neighborhoods []string `json:"neighborhoods"`
	Matches       int      `json:"matches"`
}

type pipeline struct {
	planner   Planner
	inventory Inventory
	writer    Writer
	matcher   matching.Classifier
}

// WithPipeline replaces the tool loop: planner → SQL per branch → eligibility
// → order → one writer call. See specs/002-eligibility-first-search.
func WithPipeline(planner Planner, inventory Inventory, writer Writer) Option {
	return func(agent *DefaultAgent) {
		agent.pipeline = &pipeline{planner: planner, inventory: inventory, writer: writer}
	}
}

// WithMatching supplies the grounded qualitative assessor. A missing or
// unavailable assessor leaves a prose-only requirement unconfirmed.
func WithMatching(classifier matching.Classifier) Option {
	return func(agent *DefaultAgent) { agent.pipeline.matcher = classifier }
}

// maxShown matches the old search page size.
const maxShown = 10

var section = map[eligibility.State]int{eligibility.Eligible: 0, eligibility.ConditionallyEligible: 1, eligibility.Unknown: 2}

func (a *DefaultAgent) handlePipeline(ctx context.Context, sessionID, message string, declared eligibility.Qualification, events Events) (*TurnResponse, error) {
	p := a.pipeline
	a.mu.Lock()
	sess, ok := a.sessions[sessionID]
	if !ok {
		sess = &session{id: sessionID}
		a.sessions[sessionID] = sess
	}
	turns := append(slices.Clone(sess.turns), message)
	previous := sess.plan
	a.mu.Unlock()

	plan, err := p.planner.Plan(ctx, turns, previous)
	if err != nil {
		return nil, fmt.Errorf("buyer: plan: %w", err)
	}
	q := mergeQualification(declared, plan.Qualification)

	packet := Packet{Intent: plan.Intent, Question: message, Sort: plan.Sort}
	var results []Result
	var relaxations []eligibility.Relaxation
	if len(plan.Branches) == 0 {
		// Nothing to search (a greeting): keep what is already on screen.
		a.mu.RLock()
		results = sess.results
		a.mu.RUnlock()
	} else {
		admissible, err := p.inventory.AdmissibleFacts(ctx)
		if err != nil {
			return nil, err
		}
		results, relaxations, packet.Branches, packet.Hidden, err = p.rank(ctx, plan, q, admissible)
		if err != nil {
			return nil, err
		}
		for _, b := range plan.Branches {
			packet.Requirements = append(packet.Requirements, requirementsFromQuery(b)...)
		}
		packet.Requirements = slices.CompactFunc(sortedRequirements(packet.Requirements), func(a, b Requirement) bool { return a == b })
	}
	packet.Relaxations = relaxations
	packet.Shown = results
	if plan.Intent != "ask_about_listing" && len(results) > 3 {
		packet.Shown = results[:3]
	}

	if events.Results != nil {
		events.Results(TurnResponse{Requirements: packet.Requirements, Listings: results, Relaxations: relaxations})
	}
	reply, err := p.writer.Write(ctx, packet, events.Reply)
	if err != nil {
		// The ranking is already done; a writer outage must not lose it.
		reply = templateReply(packet)
	}

	a.mu.Lock()
	sess.turns, sess.plan, sess.results = turns, plan, results
	a.mu.Unlock()
	return &TurnResponse{Reply: reply, Requirements: packet.Requirements, Listings: results, Relaxations: relaxations}, nil
}

type scored struct {
	result               Result
	section              int
	fit                  int
	quality              int
	requiredQualitative  []search.AttributeFilter
	preferredQualitative []search.AttributeFilter
}

// rank retrieves every branch, assesses each listing and orders the merged
// list: eligibility section first, then the user's sort (default: how many
// preferred requirements the listing meets), then URL for a stable order.
func (p *pipeline) rank(ctx context.Context, plan intake.Plan, q eligibility.Qualification, admissible map[string]bool) ([]Result, []eligibility.Relaxation, []BranchReport, int, error) {
	var all []eligibility.Candidate
	var rows []scored
	var reports []BranchReport
	seen := map[string]bool{}
	hidden := 0
	for _, branch := range plan.Branches {
		storedQuery, qualitative, preferredQualitative := separateQualitative(branch)
		candidates, err := p.inventory.Candidates(ctx, storedQuery)
		if err != nil {
			return nil, nil, nil, 0, err
		}
		reports = append(reports, BranchReport{Neighborhoods: branch.Neighborhoods, Matches: len(candidates)})
		for _, c := range candidates {
			if seen[c.Listing.URL] {
				continue
			}
			seen[c.Listing.URL] = true
			all = append(all, c)
			verdict := eligibility.Assess(q, c.Listing.Price, c.Rules, admissible)
			if verdict.State == eligibility.Ineligible {
				hidden++
				continue
			}
			rows = append(rows, scored{result: Result{Listing: c.Listing, Eligibility: &verdict}, section: section[verdict.State], fit: fit(c.Listing, branch.PreferredAttributes), requiredQualitative: qualitative, preferredQualitative: preferredQualitative})
		}
	}
	p.assessQualitative(ctx, rows)
	slices.SortFunc(rows, func(a, b scored) int {
		if c := cmp.Compare(a.quality, b.quality); c != 0 {
			return c
		}
		if c := cmp.Compare(a.section, b.section); c != 0 {
			return c
		}
		if c := byUserSort(plan.Sort, a, b); c != 0 {
			return c
		}
		return cmp.Compare(a.result.URL, b.result.URL)
	})
	if len(rows) > maxShown {
		rows = rows[:maxShown]
	}
	results := make([]Result, len(rows))
	for i, r := range rows {
		results[i] = r.result
		results[i].Rank = i + 1
	}
	return results, eligibility.Relaxations(q, all, admissible), reports, hidden, nil
}

// Only qualities with an evidence rubric belong here. The other attributes
// still use the existing published/parsed attribute SQL contract.
func separateQualitative(q search.Query) (search.Query, []search.AttributeFilter, []search.AttributeFilter) {
	var prose, preferredProse []search.AttributeFilter
	required := q.RequiredAttributes[:0:0]
	for _, f := range q.RequiredAttributes {
		if f.Type == "natural_light" {
			prose = append(prose, f)
		} else {
			required = append(required, f)
		}
	}
	q.RequiredAttributes = required
	for _, f := range q.PreferredAttributes {
		if f.Type == "natural_light" {
			preferredProse = append(preferredProse, f)
		}
	}
	return q, prose, preferredProse
}

func (p *pipeline) assessQualitative(ctx context.Context, rows []scored) {
	groups := map[string][]int{}
	for i := range rows {
		r := &rows[i]
		if len(r.requiredQualitative)+len(r.preferredQualitative) == 0 {
			continue
		}
		if len(r.requiredQualitative) > 0 {
			r.result.QualitativeFit, r.quality = "unconfirmed", 1
		}
		r.fit += lightHints(r.result.Listing, append(slices.Clone(r.requiredQualitative), r.preferredQualitative...))
		if p.matcher != nil && r.result.Description != "" {
			key := fmt.Sprintf("%v|%v", r.requiredQualitative, r.preferredQualitative)
			groups[key] = append(groups[key], i)
		}
	}
	for _, ids := range groups {
		first := rows[ids[0]]
		filters := append(slices.Clone(first.requiredQualitative), first.preferredQualitative...)
		criteria := make([]matching.Criterion, len(filters))
		for i, f := range filters {
			criteria[i] = matching.Criterion{ID: fmt.Sprintf("q%d", i), Text: qualitativeQuestion(f), AttributeType: f.Type, AttributeValue: f.Value, Priority: "primary"}
		}
		// ponytail: eight descriptions fit Jev's existing 80-option request limit.
		for start := 0; start < len(ids); start += 8 {
			batch := ids[start:min(start+8, len(ids))]
			candidates := make([]matching.Candidate, len(batch))
			for j, index := range batch {
				item := rows[index].result.Listing
				candidates[j] = matching.Candidate{ID: item.URL, URL: item.URL, Evidence: []matching.Evidence{{ID: "description", Text: item.Description, Provenance: "published"}}}
			}
			candidateCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			result, err := matching.New(p.matcher).Evaluate(candidateCtx, matching.Request{Criteria: criteria, Candidates: candidates})
			cancel()
			if err != nil {
				continue
			}
			byID := map[string]matching.Match{}
			for _, m := range result.Matches {
				byID[m.CandidateID] = m
			}
			for _, index := range batch {
				m, ok := byID[rows[index].result.URL]
				if !ok {
					continue
				}
				allRequired := len(rows[index].requiredQualitative) > 0
				for j, a := range m.Assessments {
					supported := a.Status == "evaluated" && a.Assessment == "supported"
					if j < len(rows[index].requiredQualitative) && !supported {
						allRequired = false
					}
					if supported {
						rows[index].fit += 3
					}
				}
				if allRequired {
					rows[index].result.QualitativeFit, rows[index].quality = "exact", 0
				}
			}
		}
	}
}

func lightHints(item listing.Listing, criteria []search.AttributeFilter) int {
	needsLight := slices.ContainsFunc(criteria, func(f search.AttributeFilter) bool { return f.Type == "natural_light" && f.Value == "high" })
	if !needsLight {
		return 0
	}
	bonus := 0
	for _, a := range item.Attributes {
		if a.Type == "exposure" && a.Value == "frente" {
			bonus++
		}
		if a.Type == "orientation" && (a.Value == "norte" || a.Value == "noreste" || a.Value == "noroeste") {
			bonus++
		}
	}
	return bonus
}

func qualitativeQuestion(f search.AttributeFilter) string {
	if f.Type == "natural_light" && f.Value == "high" {
		return "El aviso afirma explícitamente que la propiedad en conjunto o sus ambientes principales reciben buena luz natural (por ejemplo, luminoso, luz natural, sol directo). La orientación, frente, ventanas o luz de un solo cuarto son indicios, nunca apoyo suficiente. Sólo una afirmación directa de poca luz contradice."
	}
	return f.String()
}

func byUserSort(sort string, a, b scored) int {
	switch sort {
	case "price_asc":
		return compareMissingLast(a.result.Price.Amount, b.result.Price.Amount, 1)
	case "price_desc":
		return compareMissingLast(a.result.Price.Amount, b.result.Price.Amount, -1)
	case "area_desc":
		return compareMissingLast(a.result.TotalAreaM2, b.result.TotalAreaM2, -1)
	}
	return cmp.Compare(b.fit, a.fit)
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

func fit(l listing.Listing, preferred []search.AttributeFilter) int {
	n := 0
	for _, want := range preferred {
		if slices.ContainsFunc(l.Attributes, func(a listing.Attribute) bool { return a.Type == want.Type && a.Value == want.Value }) {
			n++
		}
	}
	return n
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
