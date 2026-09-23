package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
)

func TestAgencyCatalog_RealtorAddsAndListsTheirProperty(t *testing.T) {
	provider := auth.NewLocal(auth.NewMemoryStore())
	handler := httpapi.NewHandler(&recordingAgent{}, provider, agency.NewMemoryCatalog(), nil)
	token := signUpAndSignIn(t, handler, realtorSignUp)

	createdResponse := send(t, handler, http.MethodPost, "/api/agency/catalog", token, `{
		"url":"https://www.zonaprop.com.ar/propiedades/palermo-101.html",
		"neighborhood":"palermo",
		"address":"Humboldt al 1900",
		"description":"Semipiso luminoso al contrafrente.",
		"operation":"alquiler",
		"price":{"amount":850,"currency":"USD"},
		"expenses":{"amount":120000,"currency":"ARS"},
		"rooms":3,
		"bedrooms":2
	}`)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", createdResponse.Code, createdResponse.Body.String())
	}
	created := decode[agency.Property](t, createdResponse)
	if created.ID == "" || created.Agency != "Marta" || created.ContactCount != 0 {
		t.Fatalf("create returned %+v", created)
	}

	listResponse := send(t, handler, http.MethodGet, "/api/agency/catalog", token, "")
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", listResponse.Code, listResponse.Body.String())
	}
	listed := decode[struct {
		Properties []agency.Property `json:"properties"`
	}](t, listResponse)
	if len(listed.Properties) != 1 || listed.Properties[0].ID != created.ID {
		t.Fatalf("list returned %+v", listed.Properties)
	}
}

func TestAgencyCatalog_RealtorEditsAndRemovesTheirProperty(t *testing.T) {
	provider := auth.NewLocal(auth.NewMemoryStore())
	handler := httpapi.NewHandler(&recordingAgent{}, provider, agency.NewMemoryCatalog(), nil)
	token := signUpAndSignIn(t, handler, realtorSignUp)
	createdResponse := send(t, handler, http.MethodPost, "/api/agency/catalog", token, validPropertyJSON("Humboldt al 1900"))
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", createdResponse.Code, createdResponse.Body.String())
	}
	created := decode[agency.Property](t, createdResponse)

	updatedResponse := send(t, handler, http.MethodPatch, "/api/agency/catalog/"+created.ID, token, validPropertyJSON("Humboldt al 2000"))
	if updatedResponse.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", updatedResponse.Code, updatedResponse.Body.String())
	}
	if updated := decode[agency.Property](t, updatedResponse); updated.Address != "Humboldt al 2000" {
		t.Fatalf("update returned %+v", updated)
	}

	removedResponse := send(t, handler, http.MethodDelete, "/api/agency/catalog/"+created.ID, token, "")
	if removedResponse.Code != http.StatusNoContent {
		t.Fatalf("remove: expected 204, got %d: %s", removedResponse.Code, removedResponse.Body.String())
	}
	listedResponse := send(t, handler, http.MethodGet, "/api/agency/catalog", token, "")
	listed := decode[struct {
		Properties []agency.Property `json:"properties"`
	}](t, listedResponse)
	if len(listed.Properties) != 0 {
		t.Fatalf("list after remove returned %+v", listed.Properties)
	}
}

func TestAgencyCatalog_RequiresAnAuthenticatedRealtor(t *testing.T) {
	provider := auth.NewLocal(auth.NewMemoryStore())
	handler := httpapi.NewHandler(&recordingAgent{}, provider, agency.NewMemoryCatalog(), nil)

	if response := send(t, handler, http.MethodGet, "/api/agency/catalog", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous list: expected 401, got %d: %s", response.Code, response.Body.String())
	}

	registration := `{"email":"ana@example.com","password":"alquileres-caba","name":"Ana","role":"searcher"}`
	if response := send(t, handler, http.MethodPost, "/api/auth/sign-up", "", registration); response.Code != http.StatusCreated {
		t.Fatalf("searcher sign-up: expected 201, got %d: %s", response.Code, response.Body.String())
	}
	signIn := send(t, handler, http.MethodPost, "/api/auth/sign-in", "", `{"email":"ana@example.com","password":"alquileres-caba"}`)
	if signIn.Code != http.StatusOK {
		t.Fatalf("searcher sign-in: expected 200, got %d: %s", signIn.Code, signIn.Body.String())
	}
	token := decode[auth.Session](t, signIn).Token

	if response := send(t, handler, http.MethodGet, "/api/agency/catalog", token, ""); response.Code != http.StatusForbidden {
		t.Fatalf("searcher list: expected 403, got %d: %s", response.Code, response.Body.String())
	}
}

func TestContactIntent_RecordsOneCountAcrossAnIdempotentReplay(t *testing.T) {
	catalog := agency.NewMemoryCatalog()
	created, err := catalog.Add(context.Background(), agency.Owner{ID: "realtor-7", Name: "Marta"}, validAgencyPropertyInput())
	if err != nil {
		t.Fatalf("seed property: %v", err)
	}
	handler := httpapi.NewHandler(&recordingAgent{}, auth.NewLocal(auth.NewMemoryStore()), catalog, nil)
	body := `{"intent_id":"4c065799-5ad0-4df4-9b09-8dcc541507d2","source":"search_result_card"}`

	first := send(t, handler, http.MethodPost, "/api/listings/"+created.ID+"/contact-intents", "", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("first intent: expected 201, got %d: %s", first.Code, first.Body.String())
	}
	firstReceipt := decode[agency.ContactReceipt](t, first)
	if !firstReceipt.Recorded || firstReceipt.IntentID == "" || firstReceipt.ListingID != created.ID {
		t.Fatalf("first intent returned %+v", firstReceipt)
	}

	replay := send(t, handler, http.MethodPost, "/api/listings/"+created.ID+"/contact-intents", "", body)
	if replay.Code != http.StatusOK {
		t.Fatalf("replayed intent: expected 200, got %d: %s", replay.Code, replay.Body.String())
	}
	properties, err := catalog.List(context.Background(), "realtor-7")
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	if got := properties[0].ContactCount; got != 1 {
		t.Fatalf("contact count = %d, want 1", got)
	}
}

func validPropertyJSON(address string) string {
	return `{
		"url":"https://www.zonaprop.com.ar/propiedades/palermo-101.html",
		"neighborhood":"palermo",
		"address":"` + address + `",
		"description":"Semipiso luminoso al contrafrente.",
		"operation":"alquiler",
		"price":{"amount":850,"currency":"USD"},
		"expenses":{"amount":120000,"currency":"ARS"},
		"rooms":3,
		"bedrooms":2
	}`
}

func validAgencyPropertyInput() agency.PropertyInput {
	return agency.PropertyInput{
		URL:          "https://www.zonaprop.com.ar/propiedades/palermo-101.html",
		Neighborhood: "palermo",
		Address:      "Humboldt al 1900",
		Description:  "Semipiso luminoso al contrafrente.",
		Operation:    "alquiler",
		Price:        agency.Money{Amount: agencyFloat64Pointer(850), Currency: "USD"},
	}
}

func agencyFloat64Pointer(value float64) *float64 { return &value }

func signUpAndSignIn(t *testing.T, handler http.Handler, registration string) string {
	t.Helper()
	if response := send(t, handler, http.MethodPost, "/api/auth/sign-up", "", registration); response.Code != http.StatusCreated {
		t.Fatalf("sign-up failed: %d: %s", response.Code, response.Body.String())
	}
	response := send(t, handler, http.MethodPost, "/api/auth/sign-in", "", `{
		"email":"marta@inmobiliaria.com",
		"password":"alquileres-caba"
	}`)
	if response.Code != http.StatusOK {
		t.Fatalf("sign-in failed: %d: %s", response.Code, response.Body.String())
	}
	return decode[auth.Session](t, response).Token
}
