package listing_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

// fakeClient returns a canned completion so the parser's own behaviour is what
// the test observes, rather than a model's.
type fakeClient struct {
	reply     string
	err       error
	lastReq   llm.ChatRequest
	callCount int
}

func (c *fakeClient) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.callCount++
	c.lastReq = req
	if c.err != nil {
		return nil, c.err
	}
	return &llm.ChatResponse{Content: c.reply}, nil
}

func TestParse_ReturnsAttributesWithProvenanceAndEvidence(t *testing.T) {
	client := &fakeClient{reply: `natural_light|high|stated|muy luminoso
exposure|contrafrente|stated|al contrafrente
noise_level|quiet|inferred|al contrafrente`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "Depto muy luminoso al contrafrente.")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := []listing.Attribute{
		{Type: "natural_light", Value: "high", Provenance: listing.Stated, Evidence: "muy luminoso"},
		{Type: "exposure", Value: "contrafrente", Provenance: listing.Stated, Evidence: "al contrafrente"},
		{Type: "noise_level", Value: "quiet", Provenance: listing.Inferred, Evidence: "al contrafrente"},
	}
	assertAttributes(t, got, want)
}

func TestParse_StripsMarkdownFences(t *testing.T) {
	client := &fakeClient{reply: "```\nfurnished|yes|stated|Amoblado\n```"}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "Amoblado.")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	assertAttributes(t, got, []listing.Attribute{
		{Type: "furnished", Value: "yes", Provenance: listing.Stated, Evidence: "Amoblado"},
	})
}

// A 9B model will invent terms outside the vocabulary. Those must be discarded
// rather than stored, or the closed vocabulary stops being closed and the
// buyer side has nothing dependable to match against.
func TestParse_DropsValuesOutsideTheVocabulary(t *testing.T) {
	client := &fakeClient{reply: `natural_light|altisima|stated|
vista_al_rio|si|stated|
noise_level|quiet|stated|`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "irrelevante")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	assertAttributes(t, got, []listing.Attribute{
		{Type: "noise_level", Value: "quiet", Provenance: listing.Inferred},
	})
}

// "unknown" is what the prompt asks the model to say when the description does
// not establish something. Storing it would misrepresent silence as a finding.
func TestParse_DropsUnknownValues(t *testing.T) {
	client := &fakeClient{reply: `natural_light|unknown|inferred|
noise_level||stated|
pets_allowed|no|stated|`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "irrelevante")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	// Demoted to inferred: the row claims "stated" but carries no quote to
	// back it, and an unverifiable claim does not get the strong provenance.
	assertAttributes(t, got, []listing.Attribute{
		{Type: "pets_allowed", Value: "no", Provenance: listing.Inferred},
	})
}

// An unrecognised provenance is treated as the weaker of the two. Trusting a
// malformed field as "stated" would overstate how well established the claim is.
func TestParse_DowngradesUnrecognisedProvenanceToInferred(t *testing.T) {
	client := &fakeClient{reply: `amenity|pileta|obvious|pileta climatizada`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "Con pileta climatizada.")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	assertAttributes(t, got, []listing.Attribute{
		{Type: "amenity", Value: "pileta", Provenance: listing.Inferred, Evidence: "pileta climatizada"},
	})
}

func TestParse_DeduplicatesRepeatedAttributes(t *testing.T) {
	client := &fakeClient{reply: `amenity|pileta|stated|
amenity|pileta|inferred|
amenity|gimnasio|stated|`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "irrelevante")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	assertAttributes(t, got, []listing.Attribute{
		{Type: "amenity", Value: "pileta", Provenance: listing.Inferred},
		{Type: "amenity", Value: "gimnasio", Provenance: listing.Inferred},
	})
}

// A reply in no recognisable format must be reported rather than recorded as
// "this listing has no notable qualities", which is a claim about the property.
func TestParse_ReturnsErrorWhenNothingInTheReplyParses(t *testing.T) {
	client := &fakeClient{reply: "No puedo ayudarte con eso."}

	if _, err := listing.NewParser(client, "test-model").Parse(context.Background(), "irrelevante"); err == nil {
		t.Fatal("expected an error for an unparseable reply, got nil")
	}
}

// An empty reply is the model finding nothing worth recording, which is a
// legitimate outcome for a terse listing.
func TestParse_AcceptsAnEmptyReplyAsNoAttributes(t *testing.T) {
	client := &fakeClient{reply: "   "}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "Depto.")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d attributes, want 0", len(got))
	}
}

// Truncation is the failure mode that broke the JSON format: the model runs out
// of tokens mid-answer. A line format degrades to losing the last line instead
// of losing the whole reply.
func TestParse_KeepsCompleteLinesWhenTheReplyIsTruncated(t *testing.T) {
	client := &fakeClient{reply: `amenity|pileta|stated|pileta climatizada
amenity|gimnasio|stated|gimnasio
natural_li`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "irrelevante")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d attributes, want 2: %+v", len(got), got)
	}
}

// The model sometimes narrates before or after the rows; the rows must still
// be found rather than the whole reply rejected.
func TestParse_IgnoresProseAroundTheRows(t *testing.T) {
	client := &fakeClient{reply: `Claro, aquí están los atributos:
natural_light|high|stated|muy luminoso
Espero que te sirva.`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "irrelevante")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(got) != 1 || got[0].Type != "natural_light" {
		t.Errorf("got %+v, want one natural_light attribute", got)
	}
}

// Evidence is free text and will contain the delimiter; splitting must not
// truncate the quote at the first pipe inside it.
func TestParse_KeepsDelimitersInsideTheEvidence(t *testing.T) {
	client := &fakeClient{reply: `natural_light|high|stated|luminoso | al frente | sin obstrucciones`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "irrelevante")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d attributes, want 1", len(got))
	}
	if got[0].Evidence != "luminoso | al frente | sin obstrucciones" {
		t.Errorf("Evidence = %q", got[0].Evidence)
	}
}

func TestParse_ReturnsErrorWhenClientFails(t *testing.T) {
	client := &fakeClient{err: errors.New("connection refused")}

	if _, err := listing.NewParser(client, "test-model").Parse(context.Background(), "irrelevante"); err == nil {
		t.Fatal("expected an error when the client fails, got nil")
	}
}

func TestParse_RejectsBlankDescriptionWithoutCallingTheModel(t *testing.T) {
	client := &fakeClient{reply: ""}

	if _, err := listing.NewParser(client, "test-model").Parse(context.Background(), "   "); err == nil {
		t.Fatal("expected an error for a blank description, got nil")
	}
	if client.callCount != 0 {
		t.Errorf("expected no model call for a blank description, got %d", client.callCount)
	}
}

// The prompt must carry the vocabulary; without it the model has no way to
// know which terms are admissible and everything it returns gets discarded.
func TestParse_SendsVocabularyAndDescriptionToTheModel(t *testing.T) {
	client := &fakeClient{reply: ""}

	if _, err := listing.NewParser(client, "test-model").Parse(context.Background(), "Depto al frente"); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if client.lastReq.Model != "test-model" {
		t.Errorf("Model = %q, want %q", client.lastReq.Model, "test-model")
	}
	if client.lastReq.Temperature != 0 {
		t.Errorf("Temperature = %v, want 0", client.lastReq.Temperature)
	}

	var prompt strings.Builder
	for _, m := range client.lastReq.Messages {
		prompt.WriteString(m.Content)
	}
	for _, needed := range []string{"natural_light", "contrafrente", "Depto al frente"} {
		if !strings.Contains(prompt.String(), needed) {
			t.Errorf("prompt does not mention %q", needed)
		}
	}
}

func assertAttributes(t *testing.T, got, want []listing.Attribute) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d attributes, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("attribute %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The model narrates its own omissions instead of omitting: it emits a row
// whose evidence reads "no se menciona". Recording that turns the absence of a
// claim into a claim, and "outdoor_space=none" from silence would exclude
// listings that simply never mentioned a balcony.
func TestParse_DropsRowsWhoseEvidenceReportsAnAbsence(t *testing.T) {
	client := &fakeClient{reply: `amenity|parrilla|inferred|No hay mención de parrilla. Omitir.
outdoor_space|none|inferred|No se menciona balcón ni patio
heating|none|inferred|no se menciona calefacción
orientation|norte|inferred|No mencionada.
natural_light|high|stated|muy luminoso`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "Depto muy luminoso.")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	assertAttributes(t, got, []listing.Attribute{
		{Type: "natural_light", Value: "high", Provenance: listing.Stated, Evidence: "muy luminoso"},
	})
}

// A property has one orientation. When the model offers several, they cannot
// all be true, and picking one arbitrarily invents a fact -- so the type is
// dropped and the listing is left honestly silent about it.
func TestParse_DropsSingleValuedTypesTheModelContradictsItselfOn(t *testing.T) {
	client := &fakeClient{reply: `orientation|norte|inferred|zona
orientation|sur|inferred|zona
orientation|este|inferred|zona
amenity|pileta|stated|pileta climatizada
amenity|gimnasio|stated|gimnasio`}

	description := "zona pileta climatizada gimnasio"
	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), description)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	for _, attribute := range got {
		if attribute.Type == "orientation" {
			t.Errorf("kept a contradicted orientation: %+v", got)
		}
	}
	// Multi-valued types are unaffected: a building really can have both.
	if len(got) != 2 {
		t.Errorf("got %d attributes, want the 2 amenities: %+v", len(got), got)
	}
}

func TestParse_KeepsASingleValuedTypeWhenThereIsOnlyOneReading(t *testing.T) {
	client := &fakeClient{reply: `orientation|norte|stated|orientación norte`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(), "Con orientación norte.")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	assertAttributes(t, got, []listing.Attribute{
		{Type: "orientation", Value: "norte", Provenance: listing.Stated, Evidence: "orientación norte"},
	})
}

// "stated" is the strong claim the buyer side leans on, so it has to mean the
// listing actually says it. An unquotable claim is demoted rather than dropped:
// the reading may still be right, but it is no longer presented as the
// seller's own words.
func TestParse_DemotesStatedClaimsThatAreNotInTheDescription(t *testing.T) {
	client := &fakeClient{reply: `natural_light|high|stated|luminoso y soleado
amenity|pileta|stated|pileta climatizada`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(),
		"Departamento luminoso y soleado en Palermo.")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	assertAttributes(t, got, []listing.Attribute{
		{Type: "natural_light", Value: "high", Provenance: listing.Stated, Evidence: "luminoso y soleado"},
		{Type: "amenity", Value: "pileta", Provenance: listing.Inferred, Evidence: "pileta climatizada"},
	})
}

// Quote checking must not hinge on the model reproducing accents, casing and
// spacing exactly, or every genuine quote is demoted too.
func TestParse_VerifiesQuotesIgnoringCaseAccentsAndSpacing(t *testing.T) {
	client := &fakeClient{reply: `natural_light|high|stated|MUY   LUMINOSO Y AIREADO`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(),
		"Depto muy luminoso y aireado al frente.")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(got) != 1 || got[0].Provenance != listing.Stated {
		t.Errorf("genuine quote was demoted: %+v", got)
	}
}

// When a single-valued type is contradicted, a quoted claim beats an
// unquotable one rather than both being thrown away. The model offering both
// "balcon" (quoted) and "none" (inferred from silence) should leave the
// balcony standing, not erase a feature the listing plainly advertises.
func TestParse_ResolvesAContradictionInFavourOfTheQuotedClaim(t *testing.T) {
	client := &fakeClient{reply: `outdoor_space|balcon|stated|amplio balcón al frente
outdoor_space|none|inferred|no parece haber patio`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(),
		"Departamento con amplio balcón al frente.")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	assertAttributes(t, got, []listing.Attribute{
		{Type: "outdoor_space", Value: "balcon", Provenance: listing.Stated, Evidence: "amplio balcón al frente"},
	})
}

// Two equally well quoted answers stay irreconcilable, so the type is dropped.
func TestParse_DropsAContradictionBetweenTwoQuotedClaims(t *testing.T) {
	client := &fakeClient{reply: `condition|excelente|stated|excelente estado
condition|a_refaccionar|stated|a refaccionar`}

	got, err := listing.NewParser(client, "test-model").Parse(context.Background(),
		"En excelente estado. Cocina a refaccionar.")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want no condition attribute", got)
	}
}
