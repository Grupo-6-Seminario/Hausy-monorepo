// Package httpapi exposes the buyer agent to local user interfaces.
package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
)

type messageRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// NewHandler returns the local HTTP boundary for buyer search, accounts, and
// the realtor catalog. The catalog is a typed application seam, so another
// transport such as A2A can reuse it without entering through HTTP.
func NewHandler(agent buyer.Agent, provider auth.Provider, catalog agency.Catalog) http.Handler {
	mux := http.NewServeMux()
	registerAuth(mux, provider)
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

		response, err := agent.HandleMessage(r.Context(), input.SessionID, input.Message)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "El agente local no pudo responder."})
			return
		}
		writeJSON(w, http.StatusOK, response)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
