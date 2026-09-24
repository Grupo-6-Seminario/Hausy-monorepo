package buyer

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
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
}

// WithPipeline replaces the tool loop: planner → SQL per branch → eligibility
// → order → one writer call. See specs/002-eligibility-first-search.
func WithPipeline(planner Planner, inventory Inventory, writer Writer) Option {
	return func(agent *DefaultAgent) {
		agent.pipeline = &pipeline{planner: planner, inventory: inventory, writer: writer}
	}
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
	result  Result
	section int
	fit     int
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
		candidates, err := p.inventory.Candidates(ctx, branch)
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
			rows = append(rows, scored{result: Result{Listing: c.Listing, Eligibility: &verdict}, section: section[verdict.State], fit: fit(c.Listing, branch.PreferredAttributes)})
		}
	}
	slices.SortFunc(rows, func(a, b scored) int {
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
