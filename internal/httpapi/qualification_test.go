package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
)

type qualifiedAgent struct{ got eligibility.Qualification }

func (a *qualifiedAgent) HandleMessage(_ context.Context, _, _ string, q eligibility.Qualification) (*buyer.TurnResponse, error) {
	a.got = q
	return &buyer.TurnResponse{Reply: "ok"}, nil
}

type memoryQualifications map[string]eligibility.Qualification

func (m memoryQualifications) Qualification(_ context.Context, userID string) (eligibility.Qualification, error) {
	return m[userID], nil
}

func (m memoryQualifications) SaveQualification(_ context.Context, userID string, q eligibility.Qualification) error {
	m[userID] = q
	return nil
}

func TestMessagesCarryTheSearchersQualification(t *testing.T) {
	agent := &qualifiedAgent{}
	handler := httpapi.NewHandler(agent, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), memoryQualifications{})
	resp := send(t, handler, http.MethodPost, "/api/messages", "", `{"session_id":"s","message":"Palermo","qualification":{"guarantee":["caucion"],"income_band":["2000000-3000000"]}}`)
	if resp.Code != http.StatusOK || !slices.Equal(agent.got["guarantee"], []string{"caucion"}) {
		t.Fatalf("got %d, qualification %v", resp.Code, agent.got)
	}
}

func TestSignedInSearchersSaveAndReadTheirQualification(t *testing.T) {
	handler := httpapi.NewHandler(&recordingAgent{}, auth.NewLocal(auth.NewMemoryStore()), agency.NewMemoryCatalog(), memoryQualifications{})
	if resp := send(t, handler, http.MethodPut, "/api/me/qualification", "", `{"guarantee":["propietaria"]}`); resp.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous save must be refused, got %d", resp.Code)
	}
	send(t, handler, http.MethodPost, "/api/auth/sign-up", "", `{"email":"ana@ex.test","password":"buscando-depto","name":"Ana","role":"searcher"}`)
	token := decode[auth.Session](t, send(t, handler, http.MethodPost, "/api/auth/sign-in", "", `{"email":"ana@ex.test","password":"buscando-depto"}`)).Token

	if resp := send(t, handler, http.MethodPut, "/api/me/qualification", token, `{"guarantee":["propietaria"],"income_band":["3000000-"]}`); resp.Code != http.StatusNoContent {
		t.Fatalf("save: got %d %s", resp.Code, resp.Body.String())
	}
	got := decode[eligibility.Qualification](t, send(t, handler, http.MethodGet, "/api/me/qualification", token, ""))
	if !slices.Equal(got["guarantee"], []string{"propietaria"}) || !slices.Equal(got["income_band"], []string{"3000000-"}) {
		t.Fatalf("got %v", got)
	}
}
