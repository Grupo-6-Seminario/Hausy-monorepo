package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
)

type failingProvider struct{}

func (failingProvider) SignUp(context.Context, auth.Registration) (auth.User, error) {
	return auth.User{}, errors.New("private database error")
}
func (failingProvider) SignIn(context.Context, string, string) (auth.Session, error) {
	return auth.Session{}, errors.New("private database error")
}
func (failingProvider) Authenticate(context.Context, string) (auth.User, error) {
	return auth.User{}, errors.New("private database error")
}
func (failingProvider) SignOut(context.Context, string) error {
	return errors.New("private database error")
}

func TestAuthUnexpectedFailureLogsRequestIDWithoutErrorText(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	handler := httpapi.NewHandler(&recordingAgent{}, failingProvider{}, agency.NewMemoryCatalog(), nil)
	response := send(t, handler, http.MethodGet, "/api/auth/me", "private-token", "")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", response.Code)
	}
	if !strings.Contains(logs.String(), `"msg":"auth_error"`) || !strings.Contains(logs.String(), response.Header().Get("X-Request-ID")) {
		t.Fatalf("missing correlated auth error: %s", logs.String())
	}
	if strings.Contains(logs.String(), "private database error") || strings.Contains(logs.String(), "private-token") {
		t.Fatalf("auth log leaked private input: %s", logs.String())
	}
}

func newAuthHandler() http.Handler {
	return httpapi.NewHandler(&recordingAgent{}, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), nil)
}

func send(t *testing.T, handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decode[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatalf("decode %q: %v", response.Body.String(), err)
	}
	return value
}

const realtorSignUp = `{"email":"marta@inmobiliaria.com","password":"alquileres-caba","name":"Marta","role":"realtor"}`

func TestAuth_SignUpSignInMeAndSignOut(t *testing.T) {
	handler := newAuthHandler()

	signUp := send(t, handler, http.MethodPost, "/api/auth/sign-up", "", realtorSignUp)
	if signUp.Code != http.StatusCreated {
		t.Fatalf("sign-up: expected 201, got %d: %s", signUp.Code, signUp.Body.String())
	}
	created := decode[struct{ User auth.User }](t, signUp).User
	if created.Email != "marta@inmobiliaria.com" || created.Role != auth.RoleRealtor {
		t.Fatalf("sign-up returned %+v", created)
	}

	signIn := send(t, handler, http.MethodPost, "/api/auth/sign-in", "",
		`{"email":"marta@inmobiliaria.com","password":"alquileres-caba"}`)
	if signIn.Code != http.StatusOK {
		t.Fatalf("sign-in: expected 200, got %d: %s", signIn.Code, signIn.Body.String())
	}
	session := decode[auth.Session](t, signIn)
	if session.Token == "" || session.User != created {
		t.Fatalf("sign-in returned %+v", session)
	}

	me := send(t, handler, http.MethodGet, "/api/auth/me", session.Token, "")
	if me.Code != http.StatusOK {
		t.Fatalf("me: expected 200, got %d: %s", me.Code, me.Body.String())
	}
	if got := decode[struct{ User auth.User }](t, me).User; got != created {
		t.Fatalf("me returned %+v", got)
	}

	signOut := send(t, handler, http.MethodPost, "/api/auth/sign-out", session.Token, "")
	if signOut.Code != http.StatusNoContent {
		t.Fatalf("sign-out: expected 204, got %d", signOut.Code)
	}
	if after := send(t, handler, http.MethodGet, "/api/auth/me", session.Token, ""); after.Code != http.StatusUnauthorized {
		t.Fatalf("me after sign-out: expected 401, got %d", after.Code)
	}
}

func TestAuth_ErrorsMapToStatusesWithSpanishMessages(t *testing.T) {
	handler := newAuthHandler()
	if response := send(t, handler, http.MethodPost, "/api/auth/sign-up", "", realtorSignUp); response.Code != http.StatusCreated {
		t.Fatalf("seed sign-up failed: %d", response.Code)
	}

	cases := []struct {
		name, method, path, token, body string
		status                          int
		message                         string
	}{
		{"wrong password", http.MethodPost, "/api/auth/sign-in", "",
			`{"email":"marta@inmobiliaria.com","password":"incorrecta"}`,
			http.StatusUnauthorized, "El email o la contraseña no son correctos."},
		{"duplicate email", http.MethodPost, "/api/auth/sign-up", "", realtorSignUp,
			http.StatusConflict, "Ya existe una cuenta con ese email."},
		{"short password", http.MethodPost, "/api/auth/sign-up", "",
			`{"email":"otra@example.com","password":"corta","name":"Otra","role":"searcher"}`,
			http.StatusBadRequest, "La contraseña debe tener al menos 8 caracteres."},
		{"malformed body", http.MethodPost, "/api/auth/sign-in", "", `{"email":`,
			http.StatusBadRequest, "La solicitud no es válida."},
		{"me without token", http.MethodGet, "/api/auth/me", "", "",
			http.StatusUnauthorized, "Tenés que iniciar sesión."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := send(t, handler, tc.method, tc.path, tc.token, tc.body)
			if response.Code != tc.status {
				t.Fatalf("expected %d, got %d: %s", tc.status, response.Code, response.Body.String())
			}
			if got := decode[struct{ Error string }](t, response).Error; got != tc.message {
				t.Fatalf("expected message %q, got %q", tc.message, got)
			}
		})
	}
}
