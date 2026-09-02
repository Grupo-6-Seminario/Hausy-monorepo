// Package httpapi exposes the buyer agent to local user interfaces.
package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
)

type messageRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// NewHandler returns the local HTTP boundary for the buyer agent.
func NewHandler(agent buyer.Agent) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/messages", func(w http.ResponseWriter, r *http.Request) {
		var input messageRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "La solicitud no es válida."})
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
