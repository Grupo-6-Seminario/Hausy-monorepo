// Package buyer implements the Buyer Agent representing the property searcher.
package buyer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/tools"
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
	// than trimmed: the interface renders them as cards, so it wants the
	// seller's full prose and the evidence behind every parsed attribute.
	Listings []listing.Listing `json:"listings,omitempty"`
}

// Agent defines the communication interface between the user (or user-facing client) and the Buyer Agent.
type Agent interface {
	HandleMessage(ctx context.Context, sessionID string, message string) (*TurnResponse, error)
}

// session tracks conversational state and accumulated requirements for a searcher.
type session struct {
	id           string
	requirements []Requirement

	// history is the conversation as the model saw it, tool traffic included.
	// Used only in inventory mode, where a follow-up question ("¿y más
	// barato?") is unanswerable without what came before it.
	history []llm.Message
}

// DefaultAgent implements Agent backed by an llm.Client.
//
// It runs in one of two modes. Given an inventory it searches the store through
// the tools in internal/search; without one it falls back to extracting
// requirements from what the user says, which is all it can honestly do when
// there is nothing to search.
type DefaultAgent struct {
	llmClient llm.Client
	mu        sync.RWMutex
	sessions  map[string]*session

	inventory search.Repository
	runner    *tools.Runner
}

// NewAgent creates a new Buyer Agent backed by the provided LLM client.
func NewAgent(client llm.Client, options ...Option) *DefaultAgent {
	agent := &DefaultAgent{
		llmClient: client,
		sessions:  make(map[string]*session),
	}
	for _, option := range options {
		option(agent)
	}
	return agent
}

const extractionSystemPrompt = `Extract property-search requirements from the user message. Return only valid JSON in this shape: {"requirements":[{"type":"...","value":"..."}]}. Do not use Markdown fences. Do not invent requirements.`

type extractionResult struct {
	Requirements []Requirement `json:"requirements"`
}

// HandleMessage receives a user's input, extracts typed requirements, updates state, and returns the agent's turn response.
func (a *DefaultAgent) HandleMessage(ctx context.Context, sessionID string, message string) (*TurnResponse, error) {
	if sessionID == "" {
		sessionID = "default"
	}

	if a.inventory != nil {
		return a.handleWithInventory(ctx, sessionID, message)
	}

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: extractionSystemPrompt},
			{Role: "user", Content: message},
		},
		Temperature: 0,
		MaxTokens:   512,
	}

	resp, err := a.llmClient.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm extraction failed: %w", err)
	}

	extracted, err := parseExtraction(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse extracted requirements: %w", err)
	}

	a.mu.Lock()
	sess, ok := a.sessions[sessionID]
	if !ok {
		sess = &session{id: sessionID}
		a.sessions[sessionID] = sess
	}

	sess.requirements = mergeRequirements(sess.requirements, extracted.Requirements)
	accumulated := make([]Requirement, len(sess.requirements))
	copy(accumulated, sess.requirements)
	a.mu.Unlock()

	reply := generateReply(accumulated)

	return &TurnResponse{
		Reply:        reply,
		Requirements: accumulated,
	}, nil
}

func parseExtraction(content string) (*extractionResult, error) {
	content = strings.TrimSpace(content)
	// Strip markdown code fences if present
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) >= 2 {
			if strings.HasPrefix(lines[0], "```") {
				lines = lines[1:]
			}
			if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "```") {
				lines = lines[:len(lines)-1]
			}
			content = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}

	var result extractionResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("%w: raw content: %s", err, content)
	}
	return &result, nil
}

func mergeRequirements(existing, incoming []Requirement) []Requirement {
	res := append([]Requirement{}, existing...)
	for _, inc := range incoming {
		found := false
		for _, ex := range res {
			if strings.EqualFold(ex.Type, inc.Type) && strings.EqualFold(ex.Value, inc.Value) {
				found = true
				break
			}
		}
		if !found {
			res = append(res, inc)
		}
	}
	return res
}

func generateReply(reqs []Requirement) string {
	if len(reqs) == 0 {
		return "Entendido. ¿Podrías darme más detalles sobre lo que estás buscando?"
	}

	var parts []string
	for _, r := range reqs {
		parts = append(parts, fmt.Sprintf("%s: %s", r.Type, r.Value))
	}
	return fmt.Sprintf("Entendido. Requisitos registrados: %s.", strings.Join(parts, ", "))
}
