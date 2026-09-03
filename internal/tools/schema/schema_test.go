package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/tools/schema"
)

// The expected value is a JSON Schema written by hand against the spec, not
// rebuilt from the same helpers under test.
func TestObject_ProducesAValidJSONSchemaDocument(t *testing.T) {
	built := schema.Object("Search the inventory.",
		schema.Fields{
			"neighborhoods":    schema.Array("Neighborhood slugs.", schema.Enum("", "palermo", "congreso")),
			"max_price":        schema.Number("Upper bound, in currency."),
			"bedrooms":         schema.Integer("Least bedrooms."),
			"include_unpriced": schema.Boolean("Whether to keep listings with no published price."),
			"operation":        schema.Enum("What the listing is offered for.", "alquiler", "venta"),
		},
		"neighborhoods")

	const want = `{
		"type": "object",
		"description": "Search the inventory.",
		"properties": {
			"bedrooms": {"type": "integer", "description": "Least bedrooms."},
			"include_unpriced": {"type": "boolean", "description": "Whether to keep listings with no published price."},
			"max_price": {"type": "number", "description": "Upper bound, in currency."},
			"neighborhoods": {
				"type": "array",
				"description": "Neighborhood slugs.",
				"items": {"type": "string", "enum": ["palermo", "congreso"]}
			},
			"operation": {
				"type": "string",
				"description": "What the listing is offered for.",
				"enum": ["alquiler", "venta"]
			}
		},
		"required": ["neighborhoods"]
	}`

	assertJSONEqual(t, want, built)
}

func TestObject_OmitsRequiredWhenEveryFieldIsOptional(t *testing.T) {
	built := schema.Object("Nothing is mandatory.", schema.Fields{
		"url": schema.String("A listing URL."),
	})

	const want = `{
		"type": "object",
		"description": "Nothing is mandatory.",
		"properties": {"url": {"type": "string", "description": "A listing URL."}}
	}`

	assertJSONEqual(t, want, built)
}

func assertJSONEqual(t *testing.T, want string, got any) {
	t.Helper()

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("the schema does not marshal: %v", err)
	}

	var wantValue, gotValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("the expected schema is not valid JSON: %v", err)
	}
	if err := json.Unmarshal(encoded, &gotValue); err != nil {
		t.Fatalf("the built schema is not valid JSON: %v", err)
	}

	wantCanonical, _ := json.Marshal(wantValue)
	gotCanonical, _ := json.Marshal(gotValue)
	if string(wantCanonical) != string(gotCanonical) {
		t.Errorf("schema mismatch\n want: %s\n  got: %s", wantCanonical, gotCanonical)
	}
}
