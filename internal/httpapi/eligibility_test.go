package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
)

// formCatalog is migration 0007's catalog, trimmed to what these tests use.
var formCatalog = eligibility.Catalog{
	"pets": {Admissible: true, Label: "Mascotas", Priority: "high", Multiple: true, Position: 3,
		Choices: []eligibility.Choice{{Value: "none", Label: "Ninguna"}, {Value: "dog", Label: "Perro"}}},
	"guarantee": {Admissible: true, Label: "Garantía", Priority: "high", Multiple: true, Position: 1,
		Choices: []eligibility.Choice{{Value: "propietaria", Label: "Garantía propietaria"}, {Value: "caucion", Label: "Seguro de caución"}}},
	"income_band": {Admissible: true, Label: "Ingresos mensuales", Priority: "medium", Position: 4,
		Choices: []eligibility.Choice{{Value: "2000000-3000000", Label: "$2.000.000 a $3.000.000"}, {Value: "3000000-", Label: "Más de $3.000.000"}}},
	"income_documented": {Admissible: true, Label: "¿Podés comprobar tus ingresos?", Priority: "high", Position: 2,
		Choices: []eligibility.Choice{{Value: "yes", Label: "Sí"}, {Value: "no", Label: "No"}}},
	"age": {Admissible: false},
}

func (memoryQualifications) Facts(context.Context) (eligibility.Catalog, error) {
	return formCatalog, nil
}

// brokenQualifications fails every read and write, as an unreachable database does.
type brokenQualifications struct{}

func (brokenQualifications) Qualification(context.Context, string) (eligibility.Qualification, error) {
	return nil, errors.New("private database error")
}
func (brokenQualifications) SaveQualification(context.Context, string, eligibility.Qualification) error {
	return errors.New("private database error")
}
func (brokenQualifications) Facts(context.Context) (eligibility.Catalog, error) {
	return formCatalog, nil
}

// unreadableCatalog cannot read the catalog, so nothing can be validated.
type unreadableCatalog struct{ memoryQualifications }

func (unreadableCatalog) Facts(context.Context) (eligibility.Catalog, error) {
	return nil, errors.New("private database error")
}

type failingAgent struct {
	noQuestions
	err error
}

func (a failingAgent) HandleMessage(context.Context, string, string, eligibility.Qualification, buyer.Events) (*buyer.TurnResponse, error) {
	return nil, a.err
}

type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
	Field string `json:"field"`
}

func signedIn(t *testing.T, handler http.Handler) string {
	t.Helper()
	send(t, handler, http.MethodPost, "/api/auth/sign-up", "", `{"email":"ana@ex.test","password":"buscando-depto","name":"Ana","role":"searcher"}`)
	return decode[auth.Session](t, send(t, handler, http.MethodPost, "/api/auth/sign-in", "", `{"email":"ana@ex.test","password":"buscando-depto"}`)).Token
}

// Each row of the phase 1 error table (specs/004-eligibility-filter/prompts.md):
// clients tell errors apart by code, and the message stays what the searcher reads.
func TestErrorResponsesCarryAStableCode(t *testing.T) {
	newHandler := func(agent buyer.Agent, q httpapi.Qualifications) http.Handler {
		return httpapi.NewHandler(agent, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), q)
	}
	for _, tc := range []struct {
		name         string
		handler      http.Handler
		method, path string
		signIn       bool
		body         string
		status       int
		want         errorBody
	}{
		{"not JSON", newHandler(&recordingAgent{}, memoryQualifications{}), http.MethodPost, "/api/messages", false, `{"message":`,
			http.StatusBadRequest, errorBody{"La solicitud no es válida.", "invalid_json", ""}},
		{"unknown fact on save", newHandler(&recordingAgent{}, memoryQualifications{}), http.MethodPut, "/api/me/qualification", true, `{"zodiac":["leo"]}`,
			http.StatusBadRequest, errorBody{"Ese dato no está entre las preguntas.", "unknown_fact", "qualification.zodiac"}},
		{"inadmissible fact on save", newHandler(&recordingAgent{}, memoryQualifications{}), http.MethodPut, "/api/me/qualification", true, `{"age":["30"]}`,
			http.StatusBadRequest, errorBody{"Hausy no pide ese dato.", "inadmissible_fact", "qualification.age"}},
		{"value outside the choices in a search", newHandler(&recordingAgent{}, memoryQualifications{}), http.MethodPost, "/api/messages", false,
			`{"session_id":"s","message":"Palermo","qualification":{"pets":["iguana"]}}`,
			http.StatusBadRequest, errorBody{"Elegí una de las opciones de Mascotas.", "invalid_value", "qualification.pets"}},
		{"two values for a single choice", newHandler(&recordingAgent{}, memoryQualifications{}), http.MethodPut, "/api/me/qualification", true, `{"income_documented":["yes","no"]}`,
			http.StatusBadRequest, errorBody{"¿Podés comprobar tus ingresos? admite una sola opción.", "too_many_values", "qualification.income_documented"}},
		{"no session", newHandler(&recordingAgent{}, memoryQualifications{}), http.MethodPut, "/api/me/qualification", false, `{}`,
			http.StatusUnauthorized, errorBody{"Tenés que iniciar sesión.", "unauthenticated", ""}},
		{"pending question", newHandler(failingAgent{err: buyer.ErrPendingClarification}, nil), http.MethodPost, "/api/messages", false, `{"session_id":"s","message":"Palermo"}`,
			http.StatusConflict, errorBody{"Respondé o editá la pregunta pendiente antes de seguir.", "pending_clarification", ""}},
		{"stale answer", newHandler(&recordingAgent{}, nil), http.MethodPost, "/api/messages", false, `{"session_id":"s","answer":{"question_id":"q","selected":["a"]}}`,
			http.StatusConflict, errorBody{"Esa pregunta ya no está activa. Volvé a la búsqueda.", "stale_clarification", ""}},
		{"database failure", newHandler(&recordingAgent{}, brokenQualifications{}), http.MethodGet, "/api/me/qualification", true, ``,
			http.StatusInternalServerError, errorBody{"No pudimos leer tus datos.", "internal", ""}},
		{"save failure", newHandler(&recordingAgent{}, brokenQualifications{}), http.MethodPut, "/api/me/qualification", true, `{"guarantee":["caucion"]}`,
			http.StatusInternalServerError, errorBody{"No pudimos guardar tus datos.", "internal", ""}},
		{"questions unreadable", newHandler(&recordingAgent{}, unreadableCatalog{}), http.MethodGet, "/api/eligibility/facts", false, ``,
			http.StatusInternalServerError, errorBody{"No pudimos cargar las preguntas de requisitos.", "internal", ""}},
		{"questions unreadable in a search", newHandler(&recordingAgent{}, unreadableCatalog{}), http.MethodPost, "/api/messages", false,
			`{"session_id":"s","message":"Palermo","qualification":{"guarantee":["caucion"]}}`,
			http.StatusInternalServerError, errorBody{"No pudimos validar tus datos.", "internal", ""}},
		{"model down", newHandler(failingAgent{err: errors.New("dial tcp: connection refused")}, nil), http.MethodPost, "/api/messages", false, `{"session_id":"s","message":"Palermo"}`,
			http.StatusBadGateway, errorBody{"El agente local no pudo responder.", "agent_unavailable", ""}},
		{"no database", newHandler(&recordingAgent{}, nil), http.MethodGet, "/api/me/qualification", true, ``,
			http.StatusServiceUnavailable, errorBody{"No hay base de datos para guardar tus datos.", "no_database", ""}},
		{"no database for the questions", newHandler(&recordingAgent{}, nil), http.MethodGet, "/api/eligibility/facts", false, ``,
			http.StatusServiceUnavailable, errorBody{"El servicio no tiene base de datos disponible.", "no_database", ""}},
		{"no database in a search", newHandler(&recordingAgent{}, nil), http.MethodPost, "/api/messages", false,
			`{"session_id":"s","message":"Palermo","qualification":{"guarantee":["caucion"]}}`,
			http.StatusServiceUnavailable, errorBody{"El servicio no tiene base de datos disponible.", "no_database", ""}},
	} {
		token := ""
		if tc.signIn {
			token = signedIn(t, tc.handler)
		}
		resp := send(t, tc.handler, tc.method, tc.path, token, tc.body)
		if resp.Code != tc.status {
			t.Errorf("%s: status %d, want %d: %s", tc.name, resp.Code, tc.status, resp.Body.String())
			continue
		}
		if got := decode[errorBody](t, resp); got != tc.want {
			t.Errorf("%s:\n got %+v\nwant %+v", tc.name, got, tc.want)
		}
	}
}

// A refused qualification never reaches the search or the account.
func TestARefusedQualificationIsNeitherSearchedNorStored(t *testing.T) {
	agent := &qualifiedAgent{}
	store := memoryQualifications{}
	handler := httpapi.NewHandler(agent, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), store)
	if resp := send(t, handler, http.MethodPost, "/api/messages", "", `{"session_id":"s","message":"Palermo","qualification":{"age":["30"]}}`); resp.Code != http.StatusBadRequest || agent.got != nil {
		t.Fatalf("search: got %d, the agent saw %v", resp.Code, agent.got)
	}
	token := signedIn(t, handler)
	if resp := send(t, handler, http.MethodPut, "/api/me/qualification", token, `{"guarantee":["caucion"],"age":["30"]}`); resp.Code != http.StatusBadRequest || len(store) != 0 {
		t.Fatalf("save: got %d, stored %v", resp.Code, store)
	}
}

// The form reads its questions from here, without an account: admissible
// facts only, in the catalog's order.
func TestFactsServeTheAdmissibleCatalogInOrder(t *testing.T) {
	handler := httpapi.NewHandler(&recordingAgent{}, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), memoryQualifications{})
	resp := send(t, handler, http.MethodGet, "/api/eligibility/facts", "", "")
	if resp.Code != http.StatusOK || resp.Header().Get("Cache-Control") != "max-age=300" {
		t.Fatalf("got %d, Cache-Control %q", resp.Code, resp.Header().Get("Cache-Control"))
	}
	type fact struct {
		Name     string               `json:"name"`
		Label    string               `json:"label"`
		Priority string               `json:"priority"`
		Multiple bool                 `json:"multiple"`
		Choices  []eligibility.Choice `json:"choices"`
	}
	got := decode[struct {
		Facts []fact `json:"facts"`
	}](t, resp).Facts
	want := []fact{
		{"guarantee", "Garantía", "high", true, formCatalog["guarantee"].Choices},
		{"income_documented", "¿Podés comprobar tus ingresos?", "high", false, formCatalog["income_documented"].Choices},
		{"pets", "Mascotas", "high", true, formCatalog["pets"].Choices},
		{"income_band", "Ingresos mensuales", "medium", false, formCatalog["income_band"].Choices},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
