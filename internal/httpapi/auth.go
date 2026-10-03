package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
)

type signUpRequest struct {
	Email    string    `json:"email"`
	Password string    `json:"password"`
	Name     string    `json:"name"`
	Role     auth.Role `json:"role"`
}

type signInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	User auth.User `json:"user"`
}

// registerAuth mounts the account routes. Credentials arrive as JSON and
// sessions are presented as "Authorization: Bearer <token>", the same shape a
// Cognito-issued token would take, so swapping the provider keeps this contract.
func registerAuth(mux *http.ServeMux, provider auth.Provider) {
	mux.HandleFunc("POST /api/auth/sign-up", func(w http.ResponseWriter, r *http.Request) {
		var input signUpRequest
		if !decodeJSON(w, r, &input) {
			return
		}
		user, err := provider.SignUp(r.Context(), auth.Registration(input))
		if err != nil {
			writeAuthError(r.Context(), w, err)
			return
		}
		writeJSON(w, http.StatusCreated, userResponse{User: user})
	})

	mux.HandleFunc("POST /api/auth/sign-in", func(w http.ResponseWriter, r *http.Request) {
		var input signInRequest
		if !decodeJSON(w, r, &input) {
			return
		}
		session, err := provider.SignIn(r.Context(), input.Email, input.Password)
		if err != nil {
			writeAuthError(r.Context(), w, err)
			return
		}
		writeJSON(w, http.StatusOK, session)
	})

	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		user, err := provider.Authenticate(r.Context(), bearerToken(r))
		if err != nil {
			writeAuthError(r.Context(), w, err)
			return
		}
		writeJSON(w, http.StatusOK, userResponse{User: user})
	})

	mux.HandleFunc("POST /api/auth/sign-out", func(w http.ResponseWriter, r *http.Request) {
		if err := provider.SignOut(r.Context(), bearerToken(r)); err != nil {
			writeAuthError(r.Context(), w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "La solicitud no es válida.", Code: "invalid_json"})
		return false
	}
	return true
}

func bearerToken(r *http.Request) string {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(token)
}

func writeAuthError(ctx context.Context, w http.ResponseWriter, err error) {
	var registration auth.RegistrationError
	switch {
	case errors.As(err, &registration):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: registration.Reason})
	case errors.Is(err, auth.ErrEmailTaken):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "Ya existe una cuenta con ese email."})
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "El email o la contraseña no son correctos."})
	case errors.Is(err, auth.ErrUnauthenticated):
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "Tenés que iniciar sesión.", Code: "unauthenticated"})
	default:
		logging.FromContext(ctx).LogAttrs(ctx, slog.LevelError, "auth_error",
			slog.String("error_class", logging.ErrorClass(err)))
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "No pudimos procesar tu cuenta. Probá de nuevo."})
	}
}
