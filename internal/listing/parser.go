package listing

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/llm"
)

// Parser turns listing prose into vocabulary attributes.
//
// It is used only for the qualities that exist nowhere but the description --
// light, noise, exposure, condition. Prices, areas and room counts are read
// off the page during the scrape, because a model is the wrong instrument for
// reading a number that is already printed in a field.
type Parser struct {
	client llm.Client
	model  string
}

// NewParser returns a Parser backed by the given completion client. The model
// name is recorded alongside the attributes it produces, so a later re-parse
// with a different model is visible rather than silently blended in.
func NewParser(client llm.Client, model string) *Parser {
	return &Parser{client: client, model: model}
}

// Model reports the model name this parser records against its output.
func (p *Parser) Model() string { return p.model }

// Parse extracts the fuzzy qualities of a property from its description.
//
// The reply is line-delimited rather than JSON. A 9B model asked for nested
// JSON collapses "type"/"value" into one key and truncates mid-object, which
// costs the whole reply; one attribute per line degrades to losing a single
// line instead. Anything outside the vocabulary is discarded, because a small
// model reliably invents plausible-looking terms and a vocabulary that admits
// them gives the buyer side nothing dependable to match against.
func (p *Parser) Parse(ctx context.Context, description string) ([]Attribute, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil, fmt.Errorf("listing: description is empty")
	}

	resp, err := p.client.Chat(ctx, llm.ChatRequest{
		Model: p.model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt()},
			{Role: "user", Content: description},
		},
		Temperature: 0,
		// Amenity-rich listings produce twenty-odd rows with quoted evidence;
		// 1024 truncated them mid-answer.
		MaxTokens: 4096,
	})
	if err != nil {
		return nil, fmt.Errorf("listing: attribute extraction failed: %w", err)
	}

	reply := stripFences(resp.Content)
	attributes := make([]Attribute, 0, 16)
	seen := make(map[[2]string]bool, 16)
	recognised := 0

	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(line)
		// The model narrates around the rows often enough that skipping
		// non-rows is cheaper than fighting it in the prompt.
		if line == "" || !strings.Contains(line, fieldSeparator) {
			continue
		}

		// Evidence is a quote and will itself contain the separator, so the
		// split is bounded to the three leading fields and the remainder is
		// kept whole.
		fields := strings.SplitN(line, fieldSeparator, 4)
		if len(fields) < 3 {
			continue
		}
		recognised++

		attributeType := strings.ToLower(strings.TrimSpace(fields[0]))
		value := strings.ToLower(strings.TrimSpace(fields[1]))
		if !AllowsValue(attributeType, value) {
			continue
		}

		evidence := ""
		if len(fields) == 4 {
			evidence = strings.TrimSpace(fields[3])
		}

		// The model narrates its omissions instead of omitting, emitting a row
		// whose evidence reads "no se menciona". Keeping those would turn the
		// absence of a claim into a claim: "outdoor_space=none" drawn from
		// silence excludes every listing that simply never mentioned a balcony.
		if reportsAbsence(evidence) {
			continue
		}

		key := [2]string{attributeType, value}
		if seen[key] {
			continue
		}
		seen[key] = true

		// Anything other than an exact "stated" is treated as the weaker
		// claim. Reading a malformed field as "stated" would overstate how
		// well established the claim is, which is the one error the buyer
		// side cannot detect on its own.
		provenance := Inferred
		// "stated" is the strong claim the buyer side leans on, so it has to
		// mean the listing actually says it. An unquotable claim is demoted
		// rather than dropped: the reading may well be right, but it stops
		// being presented as the seller's own words.
		if strings.ToLower(strings.TrimSpace(fields[2])) == string(Stated) && quotes(description, evidence) {
			provenance = Stated
		}

		attributes = append(attributes, Attribute{
			Type:       attributeType,
			Value:      value,
			Provenance: provenance,
			Evidence:   evidence,
		})
	}

	// An empty reply is the model finding nothing worth recording. A reply
	// full of text with no recognisable row is a failure, and must not be
	// stored as "this property has no notable qualities" -- that is a claim
	// about the property, not the absence of one.
	if recognised == 0 && strings.TrimSpace(reply) != "" {
		return nil, fmt.Errorf("listing: no attribute rows in model reply: %s", truncate(resp.Content, 200))
	}
	return dropContradictions(attributes), nil
}

// singleValued lists the types a property can only have one of. A flat has one
// orientation and one exposure; it can have many amenities.
var singleValued = map[string]bool{
	"natural_light":    true,
	"noise_level":      true,
	"orientation":      true,
	"exposure":         true,
	"outdoor_space":    true,
	"condition":        true,
	"furnished":        true,
	"pets_allowed":     true,
	"air_conditioning": true,
	"heating":          true,
}

// dropContradictions resolves single-valued types the model gave more than one
// answer for.
//
// A quoted claim outranks an unquotable one, so "balcon" backed by the
// listing's own words survives "none" inferred from silence. When the readings
// are equally well supported they stay irreconcilable: keeping one arbitrarily
// would invent a fact, so the listing is left honestly silent instead.
func dropContradictions(attributes []Attribute) []Attribute {
	total := make(map[string]int, len(attributes))
	stated := make(map[string]int, len(attributes))
	for _, attribute := range attributes {
		total[attribute.Type]++
		if attribute.Provenance == Stated {
			stated[attribute.Type]++
		}
	}

	kept := attributes[:0]
	for _, attribute := range attributes {
		if !singleValued[attribute.Type] || total[attribute.Type] == 1 {
			kept = append(kept, attribute)
			continue
		}
		if stated[attribute.Type] == 1 && attribute.Provenance == Stated {
			kept = append(kept, attribute)
		}
	}
	return kept
}

// absenceMarkers are the phrases the model uses when it is explaining that the
// description does not establish something.
var absenceMarkers = []string{
	"no se menciona", "no menciona", "no mencionada", "no mencionado",
	"no hay mencion", "sin mencion", "no especifica", "no figura",
	"no indica", "no se aclara", "no se especifica", "omitir", "no aplica",
}

func reportsAbsence(evidence string) bool {
	folded := foldLabel(evidence)
	for _, marker := range absenceMarkers {
		if strings.Contains(folded, marker) {
			return true
		}
	}
	return false
}

// quotes reports whether the evidence really appears in the description,
// ignoring case, accents and spacing so a genuine quote is not rejected over
// the model failing to reproduce an accent.
func quotes(description, evidence string) bool {
	if strings.TrimSpace(evidence) == "" {
		return false
	}
	return strings.Contains(foldLabel(description), foldLabel(evidence))
}

func truncate(text string, limit int) string {
	text = strings.ReplaceAll(strings.TrimSpace(text), "\n", " ")
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}

// stripFences removes a Markdown code fence the model wrapped its answer in.
func stripFences(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "```") {
		return content
	}
	lines := strings.Split(content, "\n")
	lines = lines[1:]
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

const fieldSeparator = "|"

func systemPrompt() string {
	var b strings.Builder
	b.WriteString(`Sos un analista inmobiliario. Extraés atributos de la descripción de una propiedad en Argentina.

Respondé una línea por atributo, con este formato exacto y nada más:
type|value|provenance|evidencia

Sin markdown, sin numeración, sin texto antes ni después.

Reglas:
1. Usá solamente los type y value del vocabulario de abajo, escritos igual. Cualquier otro término se descarta.
2. Si la descripción no establece un atributo, omitilo. No adivines. Está bien devolver pocas líneas, o ninguna.
3. provenance es "stated" si la descripción lo dice explícitamente, o "inferred" si lo deducís del contexto.
4. evidencia es la frase textual de la descripción que respalda el atributo.
5. No extraigas precios, superficies ni cantidad de ambientes: esos datos no se leen acá.

Ejemplo:
natural_light|high|stated|muy luminoso, con gran ventanal
amenity|pileta|stated|pileta climatizada en el último piso
noise_level|quiet|inferred|departamento al contrafrente

Vocabulario (type: valores permitidos):
`)

	types := make([]string, 0, len(Vocabulary))
	for attributeType := range Vocabulary {
		types = append(types, attributeType)
	}
	sort.Strings(types)
	for _, attributeType := range types {
		fmt.Fprintf(&b, "- %s: %s\n", attributeType, strings.Join(Vocabulary[attributeType], ", "))
	}
	return b.String()
}
