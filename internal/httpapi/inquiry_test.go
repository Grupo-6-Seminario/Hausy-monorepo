package httpapi_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/httpapi"
)

// inquiryFacts mirrors the eligibility_facts rows (migrations 0004, 0007, 0008).
var inquiryFacts = eligibility.Catalog{
	"guarantee": {Admissible: true, Label: "Garantía", Multiple: true, Choices: []eligibility.Choice{
		{Value: "propietaria", Label: "Garantía propietaria"},
		{Value: "caucion", Label: "Seguro de caución"},
		{Value: "recibos_garante", Label: "Recibos de sueldo de un garante"},
		{Value: "none", Label: "No tengo"},
	}},
	"income_band": {Admissible: true, Label: "Ingresos mensuales", Choices: []eligibility.Choice{
		{Value: "0-1000000", Label: "Hasta $1.000.000"},
		{Value: "1000000-2000000", Label: "$1.000.000 a $2.000.000"},
		{Value: "2000000-3000000", Label: "$2.000.000 a $3.000.000"},
		{Value: "3000000-", Label: "Más de $3.000.000"},
	}},
	"income_documented": {Admissible: true, Label: "¿Podés comprobar tus ingresos?", Choices: []eligibility.Choice{
		{Value: "yes", Label: "Sí"}, {Value: "no", Label: "No"},
	}},
	"pets": {Admissible: true, Label: "Mascotas", Multiple: true, Choices: []eligibility.Choice{
		{Value: "none", Label: "Ninguna"}, {Value: "dog", Label: "Perro"},
		{Value: "cat", Label: "Gato"}, {Value: "other", Label: "Otra"},
	}},
}

func newInquiryHandler(t *testing.T) (http.Handler, string) {
	t.Helper()
	catalog := agency.NewMemoryCatalog()
	catalog.Facts = inquiryFacts
	handler := httpapi.NewHandler(&recordingAgent{}, auth.NewLocal(auth.NewMemoryStore()), catalog, nil)
	return handler, signUpAndSignIn(t, handler, realtorSignUp)
}

// A rent in pesos that accepts propietaria or caución and asks for three
// times the rent: 600.000 × 3 = 1.800.000.
const gorritiJSON = `{
	"url":"https://example.com/inmobiliaria-demo/gorriti-4800",
	"neighborhood":"palermo",
	"address":"Gorriti 4800",
	"description":"Dos ambientes luminosos.",
	"operation":"alquiler",
	"price":{"amount":600000,"currency":"ARS"},
	"expenses":{"amount":80000,"currency":"ARS"},
	"rooms":2,
	"requirements":[
		{"fact":"guarantee","operator":"one_of","values":["propietaria","caucion"],"hardness":"hard"},
		{"fact":"income_band","operator":"income_multiple","values":["3"],"hardness":"hard"}
	]
}`

func addGorriti(t *testing.T, handler http.Handler, token string) agency.Property {
	t.Helper()
	response := send(t, handler, http.MethodPost, "/api/agency/catalog", token, gorritiJSON)
	if response.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", response.Code, response.Body.String())
	}
	return decode[agency.Property](t, response)
}

// Ana declared her pets too; Gorriti does not ask about them.
const anaInquiry = `{
	"inquiry_id":"6f1c2a1e-3b4d-4e5f-8a9b-0c1d2e3f4a5b",
	"name":"Ana",
	"email":"ana@example.com",
	"message":"¿Se puede visitar el sábado?",
	"searching":["2 ambientes","en Palermo"],
	"qualification":{"guarantee":["caucion"],"income_band":["2000000-3000000"],"pets":["cat"]}
}`

func inbox(t *testing.T, handler http.Handler, token string) []agency.Inquiry {
	t.Helper()
	response := send(t, handler, http.MethodGet, "/api/agency/inquiries", token, "")
	if response.Code != http.StatusOK {
		t.Fatalf("inbox: expected 200, got %d: %s", response.Code, response.Body.String())
	}
	return decode[struct {
		Inquiries []agency.Inquiry `json:"inquiries"`
	}](t, response).Inquiries
}

func confirmedInquiry(t *testing.T, handler http.Handler, path, body string) string {
	t.Helper()
	response := send(t, handler, http.MethodPost, path+"/preview", "", body)
	if response.Code != http.StatusOK {
		t.Fatalf("preview: %d: %s", response.Code, response.Body.String())
	}
	preview := decode[map[string]any](t, response)
	if preview["preview_revision"] == nil {
		t.Fatal("preview does not identify the content being confirmed")
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(body), &input); err != nil {
		t.Fatal(err)
	}
	input["preview_revision"] = preview["preview_revision"]
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestInquiry_ChangedPreviewNeedsConfirmationAndRetryKeepsTheOriginalReceipt(t *testing.T) {
	handler, token := newInquiryHandler(t)
	gorriti := addGorriti(t, handler, token)
	path := "/api/listings/" + gorriti.ID + "/inquiries"
	if response := send(t, handler, http.MethodPost, path, "", anaInquiry); response.Code != http.StatusConflict {
		t.Fatalf("send without preview: %d %s", response.Code, response.Body.String())
	}
	body := confirmedInquiry(t, handler, path, anaInquiry)
	updated := strings.Replace(gorritiJSON, `["propietaria","caucion"]`, `["caucion"]`, 1)
	if response := send(t, handler, http.MethodPatch, "/api/agency/catalog/"+gorriti.ID, token, updated); response.Code != http.StatusOK {
		t.Fatalf("edit: %s", response.Body.String())
	}
	response := send(t, handler, http.MethodPost, path, "", body)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"preview_changed"`) {
		t.Fatalf("stale consent: %d %s", response.Code, response.Body.String())
	}
	if got := inbox(t, handler, token); len(got) != 0 {
		t.Fatalf("unconfirmed inquiry was sent: %+v", got)
	}
	body = confirmedInquiry(t, handler, path, anaInquiry)
	response = send(t, handler, http.MethodPost, path, "", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("confirmed send: %d %s", response.Code, response.Body.String())
	}
	sent := decode[agency.Inquiry](t, response)
	if response := send(t, handler, http.MethodDelete, "/api/agency/catalog/"+gorriti.ID, token, ""); response.Code != http.StatusNoContent {
		t.Fatal("archive failed")
	}
	response = send(t, handler, http.MethodPost, path, "", body)
	if response.Code != http.StatusOK {
		t.Fatalf("retry after archive: %d %s", response.Code, response.Body.String())
	}
	if got := decode[agency.Inquiry](t, response); !reflect.DeepEqual(got, sent) {
		t.Fatalf("retry changed receipt: %+v, want %+v", got, sent)
	}
}

func TestAgencyCatalog_DeclaredRequirementsReadAsTheAgencysWords(t *testing.T) {
	handler, token := newInquiryHandler(t)
	created := addGorriti(t, handler, token)

	want := []eligibility.Rule{
		{Fact: "guarantee", Operator: eligibility.OneOf, Values: []string{"propietaria", "caucion"}, Hardness: eligibility.Hard,
			Visibility: "public", Source: "declared", Evidence: "Acepta garantía propietaria o seguro de caución"},
		{Fact: "income_band", Operator: eligibility.IncomeMultiple, Values: []string{"3"}, Hardness: eligibility.Hard,
			Visibility: "public", Source: "declared", Evidence: "Ingresos de 3 veces el alquiler"},
	}
	if !reflect.DeepEqual(created.Requirements, want) {
		t.Fatalf("requirements = %+v\nwant %+v", created.Requirements, want)
	}
	listed := decode[struct {
		Properties []agency.Property `json:"properties"`
	}](t, send(t, handler, http.MethodGet, "/api/agency/catalog", token, "")).Properties
	if len(listed) != 1 || !reflect.DeepEqual(listed[0].Requirements, want) {
		t.Fatalf("listed = %+v", listed)
	}
}

func TestAgencyCatalog_RefusesARequirementTheEvaluatorCannotRead(t *testing.T) {
	handler, token := newInquiryHandler(t)
	for name, requirement := range map[string]string{
		"refusal instead of acceptance": `{"fact":"guarantee","operator":"none_of","values":["caucion"],"hardness":"hard"}`,
		"protected characteristic":      `{"fact":"age","operator":"one_of","values":["30"],"hardness":"hard"}`,
		"multiple that is not a number": `{"fact":"income_band","operator":"income_multiple","values":["tres"],"hardness":"hard"}`,
		"guarantee nobody offers":       `{"fact":"guarantee","operator":"one_of","values":["fiador"],"hardness":"hard"}`,
	} {
		t.Run(name, func(t *testing.T) {
			body := strings.Replace(validPropertyJSON("Humboldt al 1900"), `"bedrooms":2`, `"bedrooms":2,"requirements":[`+requirement+`]`, 1)
			if response := send(t, handler, http.MethodPost, "/api/agency/catalog", token, body); response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestInquiry_PreviewCarriesOnlyWhatThePropertyAsksAndStoresNothing(t *testing.T) {
	handler, token := newInquiryHandler(t)
	gorriti := addGorriti(t, handler, token)

	response := send(t, handler, http.MethodPost, "/api/listings/"+gorriti.ID+"/inquiries/preview", "", anaInquiry)
	if response.Code != http.StatusOK {
		t.Fatalf("preview: expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "Gato") {
		t.Fatalf("preview carries pets Gorriti never asked about: %s", response.Body.String())
	}
	preview := decode[agency.Inquiry](t, response)
	want := []agency.InquiryRequirement{
		{Fact: "guarantee", Label: "Garantía", Requirement: "Acepta garantía propietaria o seguro de caución",
			Hardness: eligibility.Hard, Declared: []string{"Seguro de caución"}, Outcome: eligibility.Met},
		{Fact: "income_band", Label: "Ingresos mensuales", Requirement: "Ingresos de 3 veces el alquiler",
			Hardness: eligibility.Hard, Declared: []string{"$2.000.000 a $3.000.000"}, Outcome: eligibility.Met},
	}
	if preview.Eligibility != eligibility.Eligible || !reflect.DeepEqual(preview.Requirements, want) {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.Property != "Gorriti 4800" || preview.Agency != "Marta" || preview.Name != "Ana" ||
		!reflect.DeepEqual(preview.Searching, []string{"2 ambientes", "en Palermo"}) {
		t.Fatalf("preview = %+v", preview)
	}
	if got := inbox(t, handler, token); len(got) != 0 {
		t.Fatalf("a preview delivered %+v", got)
	}
}

func TestInquiry_SendReachesOnlyTheOwningAgencyOnce(t *testing.T) {
	handler, token := newInquiryHandler(t)
	gorriti := addGorriti(t, handler, token)
	path := "/api/listings/" + gorriti.ID + "/inquiries"
	body := confirmedInquiry(t, handler, path, anaInquiry)

	if response := send(t, handler, http.MethodPost, path, "", body); response.Code != http.StatusCreated {
		t.Fatalf("send: expected 201, got %d: %s", response.Code, response.Body.String())
	}
	if response := send(t, handler, http.MethodPost, path, "", body); response.Code != http.StatusOK {
		t.Fatalf("retry: expected 200, got %d: %s", response.Code, response.Body.String())
	}

	got := inbox(t, handler, token)
	if len(got) != 1 || got[0].Name != "Ana" || got[0].Email != "ana@example.com" ||
		got[0].ListingID != gorriti.ID || got[0].Message != "¿Se puede visitar el sábado?" || got[0].CreatedAt == nil {
		t.Fatalf("inbox = %+v", got)
	}
	listed := decode[struct {
		Properties []agency.Property `json:"properties"`
	}](t, send(t, handler, http.MethodGet, "/api/agency/catalog", token, "")).Properties
	if listed[0].InquiryCount != 1 {
		t.Fatalf("inquiry_count = %d, want 1", listed[0].InquiryCount)
	}

	pablo := `{"email":"pablo@inmobiliaria.com","password":"alquileres-caba","name":"Pablo","role":"realtor"}`
	if response := send(t, handler, http.MethodPost, "/api/auth/sign-up", "", pablo); response.Code != http.StatusCreated {
		t.Fatalf("sign-up: %d: %s", response.Code, response.Body.String())
	}
	signIn := send(t, handler, http.MethodPost, "/api/auth/sign-in", "", `{"email":"pablo@inmobiliaria.com","password":"alquileres-caba"}`)
	if other := inbox(t, handler, decode[auth.Session](t, signIn).Token); len(other) != 0 {
		t.Fatalf("another agency sees %+v", other)
	}
}

func TestInquiry_ASearcherAStrictRequirementRulesOutCannotSend(t *testing.T) {
	handler, token := newInquiryHandler(t)
	gorriti := addGorriti(t, handler, token)
	recibos := strings.Replace(anaInquiry, `"guarantee":["caucion"]`, `"guarantee":["recibos_garante"]`, 1)

	response := send(t, handler, http.MethodPost, "/api/listings/"+gorriti.ID+"/inquiries", "", recibos)
	if response.Code != http.StatusUnprocessableEntity || decode[struct{ Code string }](t, response).Code != "ineligible" {
		t.Fatalf("expected 422 ineligible, got %d", response.Code)
	}
	if got := inbox(t, handler, token); len(got) != 0 {
		t.Fatalf("inbox = %+v", got)
	}
}

func TestInquiry_NeedsAWayToReachTheSearcher(t *testing.T) {
	handler, token := newInquiryHandler(t)
	gorriti := addGorriti(t, handler, token)
	anonymous := strings.Replace(anaInquiry, `"email":"ana@example.com",`, "", 1)

	response := send(t, handler, http.MethodPost, "/api/listings/"+gorriti.ID+"/inquiries", "", anonymous)
	if response.Code != http.StatusBadRequest || decode[struct{ Field string }](t, response).Field != "contact" {
		t.Fatalf("expected 400 on contact, got %d", response.Code)
	}
}

func TestInquiry_WhatsAppRequiresAnExplicitCountryCode(t *testing.T) {
	for _, tc := range []struct {
		phone  string
		status int
	}{
		{"11 5555-1234", http.StatusBadRequest},
		{"++54 9 11 5555-1234", http.StatusBadRequest},
		{"+54 9 11 5555-1234", http.StatusCreated},
	} {
		t.Run(tc.phone, func(t *testing.T) {
			handler, token := newInquiryHandler(t)
			gorriti := addGorriti(t, handler, token)
			body := strings.Replace(anaInquiry, `"email":"ana@example.com",`, `"phone":"`+tc.phone+`",`, 1)
			body = confirmedInquiry(t, handler, "/api/listings/"+gorriti.ID+"/inquiries", body)
			response := send(t, handler, http.MethodPost, "/api/listings/"+gorriti.ID+"/inquiries", "", body)
			if response.Code != tc.status {
				t.Fatalf("got %d: %s", response.Code, response.Body.String())
			}
			if tc.status == http.StatusCreated {
				if got := inbox(t, handler, token); len(got) != 1 || got[0].Phone != "+5491155551234" {
					t.Fatalf("inbox = %+v", got)
				}
			} else if !strings.Contains(response.Body.String(), "código de país") {
				t.Fatalf("missing actionable error: %s", response.Body.String())
			}
		})
	}
}

func TestInquiry_ADetailedSearchCanSendWhatItPreviewed(t *testing.T) {
	handler, token := newInquiryHandler(t)
	gorriti := addGorriti(t, handler, token)
	path := "/api/listings/" + gorriti.ID + "/inquiries"
	var input map[string]any
	if err := json.Unmarshal([]byte(anaInquiry), &input); err != nil {
		t.Fatal(err)
	}
	input["searching"] = []string{"Palermo", "Congreso", "Monserrat", "2 ambientes", "1 dormitorio", "1 baño", "con cochera", "luminoso", "balcón", "pileta", "lavadero", "ascensor", "apto mascotas", "a estrenar", "silencioso"}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	body := confirmedInquiry(t, handler, path, string(encoded))
	response := send(t, handler, http.MethodPost, path, "", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("previewed search rejected at send: %d %s", response.Code, response.Body.String())
	}
	if got := inbox(t, handler, token); len(got) != 1 || len(got[0].Searching) != 15 {
		t.Fatalf("detailed search = %+v", got)
	}
}

func TestInquiry_AListingNoAgencyManagesInHausyTakesNoInquiries(t *testing.T) {
	handler, _ := newInquiryHandler(t)
	if response := send(t, handler, http.MethodPost, "/api/listings/999/inquiries/preview", "", anaInquiry); response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", response.Code, response.Body.String())
	}
}

func TestAuth_ClosedToRealtorsKeepsSearcherSignUpOpen(t *testing.T) {
	provider := auth.ClosedToRealtors{Provider: auth.NewLocal(auth.NewMemoryStore())}
	handler := httpapi.NewHandler(&recordingAgent{}, provider, agency.NewMemoryCatalog(), nil)

	realtor := send(t, handler, http.MethodPost, "/api/auth/sign-up", "", realtorSignUp)
	if realtor.Code != http.StatusBadRequest || !strings.Contains(realtor.Body.String(), "cerrada") {
		t.Fatalf("realtor sign-up: expected 400 closed, got %d: %s", realtor.Code, realtor.Body.String())
	}
	searcher := `{"email":"ana@example.com","password":"buscando-depto","name":"Ana","role":"searcher"}`
	if response := send(t, handler, http.MethodPost, "/api/auth/sign-up", "", searcher); response.Code != http.StatusCreated {
		t.Fatalf("searcher sign-up: expected 201, got %d: %s", response.Code, response.Body.String())
	}
}
