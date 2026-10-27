package httpapi

import (
	"errors"
	"net/http"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
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

	// An inquiry needs no account: searching never does. The preview is the
	// exact payload the agency would receive, so the searcher consents to it.
	mux.HandleFunc("POST /api/listings/{id}/inquiries/preview", func(w http.ResponseWriter, r *http.Request) {
		var input agency.InquiryInput
		if !decodeJSON(w, r, &input) {
			return
		}
		inquiry, err := catalog.PreviewInquiry(r.Context(), r.PathValue("id"), input)
		if err != nil {
			writeInquiryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, inquiry)
	})

	mux.HandleFunc("POST /api/listings/{id}/inquiries", func(w http.ResponseWriter, r *http.Request) {
		var input agency.InquiryInput
		if !decodeJSON(w, r, &input) {
			return
		}
		inquiry, err := catalog.SendInquiry(r.Context(), r.PathValue("id"), input)
		if err != nil {
			writeInquiryError(w, err)
			return
		}
		status := http.StatusOK
		if inquiry.Created {
			status = http.StatusCreated
		}
		writeJSON(w, status, inquiry)
	})

	mux.HandleFunc("GET /api/agency/inquiries", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedRealtor(w, r, provider)
		if !ok {
			return
		}
		inquiries, err := catalog.Inquiries(r.Context(), user.ID)
		if err != nil {
			writeAgencyError(w, err)
			return
		}
		if inquiries == nil {
			inquiries = []agency.Inquiry{}
		}
		writeJSON(w, http.StatusOK, map[string][]agency.Inquiry{"inquiries": inquiries})
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

// writeInquiryError answers a searcher, who manages no catalog: a property
// they cannot reach is one that takes no inquiries.
func writeInquiryError(w http.ResponseWriter, err error) {
	var invalid agency.InquiryError
	var qualification *eligibility.QualificationError
	switch {
	case errors.As(err, &invalid):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: invalid.Reason, Code: "invalid_inquiry", Field: invalid.Field})
	case errors.As(err, &qualification):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "Revisá lo que declaraste sobre vos.", Code: qualification.Code, Field: "qualification." + qualification.Fact})
	case errors.Is(err, agency.ErrIneligibleInquiry):
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: "No cumplís un requisito obligatorio de esta propiedad.", Code: "ineligible"})
	case errors.Is(err, agency.ErrInquiryConflict):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "Esa consulta ya se envió por otra propiedad."})
	case errors.Is(err, agency.ErrInquiryPreviewChanged):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "Los requisitos cambiaron. Revisá la vista previa actualizada antes de enviar.", Code: "preview_changed"})
	case errors.Is(err, agency.ErrPropertyNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "Esta propiedad no recibe consultas por Hausy."})
	default:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "No pudimos enviar la consulta. Probá de nuevo."})
	}
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
