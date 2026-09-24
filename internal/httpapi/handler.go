// Package httpapi exposes the buyer agent to local user interfaces.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
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
	Qualification eligibility.Qualification `json:"qualification,omitempty"`
}

// Qualifications stores what signed-in searchers declared; postgres.Store fits.
type Qualifications interface {
	Qualification(ctx context.Context, userID string) (eligibility.Qualification, error)
	SaveQualification(ctx context.Context, userID string, q eligibility.Qualification) error
}

type errorResponse struct {
	Error string `json:"error"`
}

// NewHandler returns the local HTTP boundary for buyer search, accounts, and
// the realtor catalog. The catalog is a typed application seam, so another
// transport such as A2A can reuse it without entering through HTTP.
// qualifications may be nil when there is no database.
func NewHandler(agent buyer.Agent, provider auth.Provider, catalog agency.Catalog, qualifications Qualifications) http.Handler {
	mux := http.NewServeMux()
	registerAuth(mux, provider)
	registerQualification(mux, provider, qualifications)
	registerAgency(mux, provider, catalog)
	mux.HandleFunc("POST /api/messages", func(w http.ResponseWriter, r *http.Request) {
		var input messageRequest
		if !decodeJSON(w, r, &input) {
			return
		}

		input.Message = strings.TrimSpace(input.Message)
		if input.Message == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "El mensaje no puede estar vacío."})
			return
		}

		if strings.Contains(r.Header.Get("Accept"), "application/x-ndjson") {
			streamTurn(w, r, agent, input)
			return
		}
		response, err := agent.HandleMessage(r.Context(), input.SessionID, input.Message, input.Qualification, buyer.Events{})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: agentUnavailable})
			return
		}
		writeJSON(w, http.StatusOK, response)
	})
	return mux
}

const agentUnavailable = "El agente local no pudo responder."

// turnEvent is one line of a streamed turn: "results" (the ranking, before
// the reply), "reply" (a piece of it), then "done" (the whole turn, whose
// reply replaces the streamed one) or "error".
type turnEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta,omitempty"`
	Error string `json:"error,omitempty"`
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
	response, err := agent.HandleMessage(r.Context(), input.SessionID, input.Message, input.Qualification, buyer.Events{
		Results: func(turn buyer.TurnResponse) { send(turnEvent{Type: "results", TurnResponse: &turn}) },
		Reply:   func(delta string) { send(turnEvent{Type: "reply", Delta: delta}) },
	})
	switch {
	case err == nil:
		send(turnEvent{Type: "done", TurnResponse: response})
	case started:
		send(turnEvent{Type: "error", Error: agentUnavailable})
	default:
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: agentUnavailable})
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
			writeAuthError(w, err)
			return
		}
		if store == nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "No hay base de datos para guardar tu perfil."})
			return
		}
		q, err := store.Qualification(r.Context(), user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "No pudimos leer tu perfil."})
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
			writeAuthError(w, err)
			return
		}
		if store == nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "No hay base de datos para guardar tu perfil."})
			return
		}
		var q eligibility.Qualification
		if !decodeJSON(w, r, &q) {
			return
		}
		if err := store.SaveQualification(r.Context(), user.ID, q); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "Ese dato no se puede guardar en tu perfil."})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
