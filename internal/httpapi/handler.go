// Package httpapi exposes the buyer agent to local user interfaces.
package httpapi

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
)

type messageRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	// Qualification comes from the qualification form, prefilled from the
	// account when signed in. Optional: searching never requires it.
	Qualification eligibility.Qualification  `json:"qualification,omitempty"`
	Answer        *buyer.ClarificationAnswer `json:"answer,omitempty"`
}

// Qualifications stores what signed-in searchers declared and the fact catalog
// they declare it against; postgres.Store fits.
type Qualifications interface {
	Qualification(ctx context.Context, userID string) (eligibility.Qualification, error)
	SaveQualification(ctx context.Context, userID string, q eligibility.Qualification) error
	Facts(ctx context.Context) (eligibility.Catalog, error)
}

// errorResponse is every error body. Error is what the searcher reads; Code is
// stable for clients (specs/004, phase 1 table) and Field names the offending
// input when there is one.
type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
	Field string `json:"field,omitempty"`
}

// NewHandler returns the local HTTP boundary for buyer search, accounts, and
// the realtor catalog. The catalog is a typed application seam, so another
// transport such as A2A can reuse it without entering through HTTP.
// qualifications may be nil when there is no database.
func NewHandler(agent buyer.Agent, provider auth.Provider, catalog agency.Catalog, qualifications Qualifications) http.Handler {
	mux := http.NewServeMux()
	registerAuth(mux, provider)
	registerQualification(mux, provider, qualifications)
	registerFacts(mux, qualifications)
	registerAgency(mux, provider, catalog)
	mux.HandleFunc("GET /api/messages", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"clarification": agent.PendingClarification(r.URL.Query().Get("session_id"))})
	})
	mux.HandleFunc("POST /api/messages", func(w http.ResponseWriter, r *http.Request) {
		var input messageRequest
		if !decodeJSON(w, r, &input) {
			return
		}

		input.Message = strings.TrimSpace(input.Message)
		if input.Answer == nil && input.Message == "" || input.Answer != nil && (input.Message != "" || input.SessionID == "") {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "El mensaje no puede estar vacío."})
			return
		}
		if len(input.Qualification) > 0 && !validQualification(w, r, qualifications, input.Qualification) {
			return
		}
		if strings.Contains(r.Header.Get("Accept"), "application/x-ndjson") {
			streamTurn(w, r, agent, input)
			return
		}
		response, err := runTurn(r.Context(), agent, input, buyer.Events{})
		if err != nil {
			status, body := domainError(err, nil)
			writeJSON(w, status, body)
			return
		}
		writeJSON(w, http.StatusOK, response)
	})
	return logRequests(mux)
}

func runTurn(ctx context.Context, agent buyer.Agent, input messageRequest, events buyer.Events) (*buyer.TurnResponse, error) {
	if input.Answer != nil {
		return agent.HandleClarification(ctx, input.SessionID, *input.Answer, input.Qualification, events)
	}
	return agent.HandleMessage(ctx, input.SessionID, input.Message, input.Qualification, events)
}

// domainError maps an error from the domain or the buyer turn to its HTTP
// status and body (specs/004, phase 1 table). catalog supplies the fact labels
// a refused qualification names; a turn error needs none. Anything unknown is
// the model failing to answer.
func domainError(err error, catalog eligibility.Catalog) (int, errorResponse) {
	var invalid *eligibility.QualificationError
	switch {
	case errors.As(err, &invalid):
		label := catalog[invalid.Fact].Label
		message := map[string]string{
			"unknown_fact":      "Ese dato no está entre las preguntas.",
			"inadmissible_fact": "Hausy no pide ese dato.",
			"invalid_value":     "Elegí una de las opciones de " + label + ".",
			"too_many_values":   label + " admite una sola opción.",
		}[invalid.Code]
		return http.StatusBadRequest, errorResponse{Error: message, Code: invalid.Code, Field: "qualification." + invalid.Fact}
	case errors.Is(err, buyer.ErrStaleClarification):
		return http.StatusConflict, errorResponse{Error: "Esa pregunta ya no está activa. Volvé a la búsqueda.", Code: "stale_clarification"}
	case errors.Is(err, buyer.ErrPendingClarification):
		return http.StatusConflict, errorResponse{Error: "Respondé o editá la pregunta pendiente antes de seguir.", Code: "pending_clarification"}
	}
	return http.StatusBadGateway, errorResponse{Error: agentUnavailable, Code: "agent_unavailable"}
}

const agentUnavailable = "El agente local no pudo responder."

// turnEvent is one line of a streamed turn: "results" (the ranking, before
// the reply), "reply" (a piece of it), then "done" (the whole turn, whose
// reply replaces the streamed one) or "error".
type turnEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta,omitempty"`
	Error string `json:"error,omitempty"`
	Code  string `json:"code,omitempty"`
	*buyer.TurnResponse
}

// streamTurn answers one turn as NDJSON, so the interface shows the cards and
// the reply while it is written. Before the first event a failure is the
// plain 502; after it, only an error event can report it.
func streamTurn(w http.ResponseWriter, r *http.Request, agent buyer.Agent, input messageRequest) {
	started := false
	send := func(event turnEvent) {
		if !started {
			w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			started = true
		}
		_ = json.NewEncoder(w).Encode(event)
		_ = http.NewResponseController(w).Flush()
	}
	response, err := runTurn(r.Context(), agent, input, buyer.Events{
		Results: func(turn buyer.TurnResponse) { send(turnEvent{Type: "results", TurnResponse: &turn}) },
		Reply:   func(delta string) { send(turnEvent{Type: "reply", Delta: delta}) },
	})
	switch {
	case err == nil:
		send(turnEvent{Type: "done", TurnResponse: response})
	case started:
		setOutcome(r.Context(), "error")
		send(turnEvent{Type: "error", Error: agentUnavailable, Code: "agent_unavailable"})
	default:
		setOutcome(r.Context(), "error")
		status, body := domainError(err, nil)
		writeJSON(w, status, body)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// registerQualification lets a signed-in searcher keep their qualification,
// so the form comes prefilled on their next visit.
func registerQualification(mux *http.ServeMux, provider auth.Provider, store Qualifications) {
	mux.HandleFunc("GET /api/me/qualification", func(w http.ResponseWriter, r *http.Request) {
		user, err := provider.Authenticate(r.Context(), bearerToken(r))
		if err != nil {
			writeAuthError(r.Context(), w, err)
			return
		}
		if store == nil {
			writeJSON(w, http.StatusServiceUnavailable, noDatabase)
			return
		}
		q, err := store.Qualification(r.Context(), user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "No pudimos leer tus datos.", Code: "internal"})
			return
		}
		if q == nil {
			q = eligibility.Qualification{}
		}
		writeJSON(w, http.StatusOK, q)
	})
	mux.HandleFunc("PUT /api/me/qualification", func(w http.ResponseWriter, r *http.Request) {
		user, err := provider.Authenticate(r.Context(), bearerToken(r))
		if err != nil {
			writeAuthError(r.Context(), w, err)
			return
		}
		if store == nil {
			writeJSON(w, http.StatusServiceUnavailable, noDatabase)
			return
		}
		var q eligibility.Qualification
		if !decodeJSON(w, r, &q) {
			return
		}
		if !validQualification(w, r, store, q) {
			return
		}
		if err := store.SaveQualification(r.Context(), user.ID, q); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "No pudimos guardar tus datos.", Code: "internal"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

var (
	noDatabase = errorResponse{Error: "No hay base de datos para guardar tus datos.", Code: "no_database"}
	// The catalog paths (the form's questions, a search's validation) store nothing.
	noCatalog = errorResponse{Error: "El servicio no tiene base de datos disponible.", Code: "no_database"}
)

// factResponse is one question of the qualification form.
type factResponse struct {
	Name     string               `json:"name"`
	Label    string               `json:"label"`
	Priority string               `json:"priority"`
	Multiple bool                 `json:"multiple"`
	Choices  []eligibility.Choice `json:"choices"`
}

// registerFacts serves the qualification form's questions. It is public:
// searching never needs an account. Inadmissible facts are never offered.
func registerFacts(mux *http.ServeMux, store Qualifications) {
	mux.HandleFunc("GET /api/eligibility/facts", func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeJSON(w, http.StatusServiceUnavailable, noCatalog)
			return
		}
		catalog, err := store.Facts(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "No pudimos cargar las preguntas de requisitos.", Code: "internal"})
			return
		}
		names := slices.SortedFunc(maps.Keys(catalog), func(a, b string) int {
			return cmp.Or(cmp.Compare(catalog[a].Position, catalog[b].Position), strings.Compare(a, b))
		})
		facts := []factResponse{}
		for _, name := range names {
			if fact := catalog[name]; fact.Admissible {
				facts = append(facts, factResponse{Name: name, Label: fact.Label, Priority: fact.Priority, Multiple: fact.Multiple, Choices: fact.Choices})
			}
		}
		w.Header().Set("Cache-Control", "max-age=300")
		writeJSON(w, http.StatusOK, map[string][]factResponse{"facts": facts})
	})
}

// validQualification checks a declared qualification against the catalog and
// answers the first problem, so nothing invalid is searched or stored.
func validQualification(w http.ResponseWriter, r *http.Request, store Qualifications, q eligibility.Qualification) bool {
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, noCatalog)
		return false
	}
	catalog, err := store.Facts(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "No pudimos validar tus datos.", Code: "internal"})
		return false
	}
	if err := eligibility.ValidateQualification(catalog, q); err != nil {
		status, body := domainError(err, catalog)
		writeJSON(w, status, body)
		return false
	}
	return true
}
