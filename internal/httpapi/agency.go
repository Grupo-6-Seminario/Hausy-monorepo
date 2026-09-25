package httpapi

import (
	"errors"
	"net/http"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
)

type catalogResponse struct {
	Properties []agency.Property `json:"properties"`
}

func registerAgency(mux *http.ServeMux, provider auth.Provider, catalog agency.Catalog) {
	mux.HandleFunc("POST /api/listings/{id}/contact-intents", func(w http.ResponseWriter, r *http.Request) {
		var intent agency.ContactIntent
		if !decodeJSON(w, r, &intent) {
			return
		}
		receipt, err := catalog.RecordContactIntent(r.Context(), r.PathValue("id"), intent)
		if err != nil {
			writeAgencyError(w, err)
			return
		}
		status := http.StatusOK
		if receipt.Created {
			status = http.StatusCreated
		}
		writeJSON(w, status, receipt)
	})

	mux.HandleFunc("GET /api/agency/catalog", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedRealtor(w, r, provider)
		if !ok {
			return
		}
		properties, err := catalog.List(r.Context(), user.ID)
		if err != nil {
			writeAgencyError(w, err)
			return
		}
		if properties == nil {
			properties = []agency.Property{}
		}
		writeJSON(w, http.StatusOK, catalogResponse{Properties: properties})
	})

	mux.HandleFunc("POST /api/agency/catalog", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedRealtor(w, r, provider)
		if !ok {
			return
		}
		var input agency.PropertyInput
		if !decodeJSON(w, r, &input) {
			return
		}
		property, err := catalog.Add(r.Context(), agency.Owner{ID: user.ID, Name: user.Name}, input)
		if err != nil {
			writeAgencyError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, property)
	})

	mux.HandleFunc("PATCH /api/agency/catalog/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedRealtor(w, r, provider)
		if !ok {
			return
		}
		var input agency.PropertyInput
		if !decodeJSON(w, r, &input) {
			return
		}
		property, err := catalog.Update(r.Context(), user.ID, r.PathValue("id"), input)
		if err != nil {
			writeAgencyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, property)
	})

	mux.HandleFunc("DELETE /api/agency/catalog/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedRealtor(w, r, provider)
		if !ok {
			return
		}
		if err := catalog.Remove(r.Context(), user.ID, r.PathValue("id")); err != nil {
			writeAgencyError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func authenticatedRealtor(w http.ResponseWriter, r *http.Request, provider auth.Provider) (auth.User, bool) {
	user, err := provider.Authenticate(r.Context(), bearerToken(r))
	if err != nil {
		writeAuthError(r.Context(), w, err)
		return auth.User{}, false
	}
	if user.Role != auth.RoleRealtor {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "Esta cuenta no administra un catálogo inmobiliario."})
		return auth.User{}, false
	}
	return user, true
}

func writeAgencyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agency.ErrInvalidProperty):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "Revisá los datos de la propiedad."})
	case errors.Is(err, agency.ErrInvalidContactIntent):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "El evento de contacto no es válido."})
	case errors.Is(err, agency.ErrContactIntentConflict):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "Ese evento de contacto ya pertenece a otra propiedad."})
	case errors.Is(err, agency.ErrPropertyNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "No encontramos esa propiedad en tu catálogo."})
	default:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "No pudimos actualizar el catálogo. Probá de nuevo."})
	}
}
