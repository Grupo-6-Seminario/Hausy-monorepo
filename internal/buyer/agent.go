// Package buyer implements the Buyer Agent representing the property searcher.
package buyer

import (
	"context"
	"sync"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/clarification"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/intake"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

// EligibilityState defines the four eligibility states outlined in core.md.
type EligibilityState string

const (
	StateEligible              EligibilityState = "eligible"
	StateIneligible            EligibilityState = "ineligible"
	StateConditionallyEligible EligibilityState = "conditionally_eligible"
	StateUnknown               EligibilityState = "unknown"
)

// Requirement represents a typed property search constraint or preference.
type Requirement struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// TurnResponse is the response delivered back to the user after each conversational turn.
type TurnResponse struct {
	Reply        string        `json:"reply"`
	Requirements []Requirement `json:"requirements"`

	// Listings are the properties the agent settled on this turn, whole rather
	// than trimmed: a card shows only each result's Matched, and the rest
	// stays for a detail view.
	Listings []Result `json:"listings,omitempty"`

	// Relaxations is the zero-results line: hidden ineligible listings that
	// another qualification would bring back.
	Relaxations       []eligibility.Relaxation `json:"relaxations,omitempty"`
	Clarification     *clarification.Question  `json:"clarification,omitempty"`
	ClarificationHint string                   `json:"clarification_hint,omitempty"`
}

// Result is a shown listing with its eligibility for this searcher.
type Result struct {
	listing.Listing
	Eligibility *eligibility.Verdict `json:"eligibility,omitempty"`
	// QualitativeFit distinguishes evidenced matches from results that still
	// need a human to confirm a required prose-only quality.
	QualitativeFit string `json:"qualitative_fit,omitempty"`
	// Matched are the attributes that answer what the searcher asked for. The
	// card shows only these; Attributes keeps the rest for a detail view.
	Matched []listing.Attribute `json:"matched,omitempty"`
}

// Agent defines the communication interface between the user (or user-facing client) and the Buyer Agent.
type Agent interface {
	// HandleMessage runs one turn. q is what the searcher declared in the
	// qualification form or their account; it may be nil. events may be zero.
	HandleMessage(ctx context.Context, sessionID string, message string, q eligibility.Qualification, events Events) (*TurnResponse, error)
}

// Events lets a caller watch a turn while it runs, so the searcher sees the
// cards before the reply is done. Both hooks are optional.
type Events struct {
	// Results fires once the ranking is ready, before the reply is written;
	// its Reply is empty.
	Results func(TurnResponse)
	// Reply receives the reply as the writer produces it. The returned
	// TurnResponse.Reply is still the authority: after a writer failure it is
	// a template that replaces whatever was streamed.
	Reply func(delta string)
}

// session is one searcher's conversation: every user message so far, the
// plan they produced, and what is on screen.
type session struct {
	id              string
	turns           []string
	plan            intake.Plan
	results         []Result
	pending         *pendingClarification
	inFlight        bool
	resolvedSources map[string]bool
	declinedFacts   map[string]bool
	confirmed       []clarification.Effect
}

// Option configures an Agent at construction.
type Option func(*DefaultAgent)

// DefaultAgent implements Agent: planner → SQL per branch → eligibility →
// order → one writer call. See specs/002-eligibility-first-search.
type DefaultAgent struct {
	mu       sync.RWMutex
	sessions map[string]*session

	pipeline *pipeline
}

// NewAgent creates a Buyer Agent that searches inventory.
func NewAgent(planner Planner, inventory Inventory, writer Writer, options ...Option) *DefaultAgent {
	agent := &DefaultAgent{
		sessions: make(map[string]*session),
		pipeline: &pipeline{planner: planner, inventory: inventory, writer: writer},
	}
	for _, option := range options {
		option(agent)
	}
	return agent
}

// HandleMessage runs one turn of the searcher's conversation.
func (a *DefaultAgent) HandleMessage(ctx context.Context, sessionID string, message string, q eligibility.Qualification, events Events) (*TurnResponse, error) {
	if sessionID == "" {
		sessionID = "default"
	}
	return a.handlePipeline(ctx, sessionID, message, q, events, nil)
}
