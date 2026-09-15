package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
)

func newAuthHandler() http.Handler {
	return httpapi.NewHandler(&recordingAgent{}, auth.NewLocal(auth.NewMemoryStore()))
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
