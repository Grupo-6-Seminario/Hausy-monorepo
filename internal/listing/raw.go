package listing

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Raw is one listing exactly as the scraper read it off the page: text, not
// values. The scrape deliberately stops at "what does the page say", so that
// turning "$ 850.000" into an amount is a pure function that can be tested,
// corrected and re-run without visiting the site again.
type Raw struct {
	Source       string            `json:"source,omitempty"`
	URL          string            `json:"url"`
	Neighborhood string            `json:"neighborhood"`
	Agency       string            `json:"agency,omitempty"`
	Address      string            `json:"address,omitempty"`
	Title        string            `json:"title,omitempty"`
	Description  string            `json:"description"`
	Operation    string            `json:"operation,omitempty"`
	PriceText    string            `json:"price_text,omitempty"`
	ExpensesText string            `json:"expenses_text,omitempty"`
	Features     map[string]string `json:"features,omitempty"`
	ScrapedAt    time.Time         `json:"scraped_at"`
}

// Validate reports whether the row can be ingested at all. The URL is the
// identity the store keys on and the description is the parser's only input,
// so a row missing either is not a listing we can do anything with.
func (r Raw) Validate() error {
	if strings.TrimSpace(r.URL) == "" {
		return fmt.Errorf("listing: row has no url")
	}
	if strings.TrimSpace(r.Description) == "" {
		return fmt.Errorf("listing: %s has no description", r.URL)
	}
	return nil
}

// readMoreAffordance is ZonaProp's "expand the text" control, which lands in
// the scraped text as though the seller had written it.
const readMoreAffordance = "Leer descripción completa"

// Normalize turns scraped text into typed values. It never guesses: a figure
// the listing does not publish stays nil, because a buyer agent filtering on
// "expenses under X" must not be handed a zero that the seller never claimed.
func Normalize(raw Raw) Listing {
	source := raw.Source
	if source == "" {
		source = "zonaprop"
	}

	description := strings.TrimSpace(raw.Description)
	description = strings.TrimSpace(strings.TrimSuffix(description, readMoreAffordance))

	item := Listing{
		Source:       source,
		URL:          strings.TrimSpace(raw.URL),
		Neighborhood: strings.TrimSpace(raw.Neighborhood),
		Agency:       strings.TrimSpace(raw.Agency),
		Address:      strings.TrimSpace(raw.Address),
		Description:  description,
		Operation:    strings.ToLower(strings.TrimSpace(raw.Operation)),
		Price:        parseMoney(raw.PriceText),
		Expenses:     parseMoney(raw.ExpensesText),
		ScrapedAt:    raw.ScrapedAt,
	}

	features := normalizeFeatureKeys(raw.Features)
	item.TotalAreaM2 = parseDecimal(features["superficie total"])
	item.CoveredAreaM2 = parseDecimal(features["superficie cubierta"])
	item.Rooms = parseCount(features["ambientes"])
	item.Bedrooms = parseCount(features["dormitorios"])
	item.Bathrooms = parseCount(features["banos"])
	item.ParkingSpaces = parseCount(features["cocheras"])
	item.AgeYears = parseAge(features["antiguedad"])
	item.Floor = strings.TrimSpace(features["piso"])

	if item.Operation == "" {
		item.Operation = detectOperation(raw.PriceText)
	}

	// ZonaProp publishes the disposition as its own field, so exposure is read
	// rather than inferred. Deriving it here keeps one of the qualities a
	// buyer trades off on from depending on a model at all.
	if exposure := detectExposure(features["disposicion"]); exposure != "" {
		item.Attributes = append(item.Attributes, Attribute{
			Type:       "exposure",
			Value:      exposure,
			Provenance: Stated,
			Evidence:   strings.TrimSpace(features["disposicion"]),
		})
	}

	return item
}

// detectOperation reads the operation out of the price line, which ZonaProp
// renders as "Alquiler $ 450.000".
func detectOperation(priceText string) string {
	folded := foldLabel(priceText)
	switch {
	case strings.Contains(folded, "alquiler temporal"), strings.Contains(folded, "temporario"):
		return "alquiler_temporal"
	case strings.Contains(folded, "alquiler"):
		return "alquiler"
	case strings.Contains(folded, "venta"):
		return "venta"
	default:
		return ""
	}
}

func detectExposure(disposition string) string {
	folded := foldLabel(disposition)
	// Contrafrente contains "frente", so it has to be tested first or every
	// rear-facing flat is recorded as front-facing.
	for _, value := range []string{"contrafrente", "frente", "interno", "lateral"} {
		if strings.Contains(folded, value) {
			return value
		}
	}
	return ""
}

// featureAliases maps the accent- and case-folded labels ZonaProp uses to the
// single key Normalize looks up, so a page variant spelling a label
// differently still lands on the same field.
var featureAliases = map[string]string{
	"superficie total":    "superficie total",
	"sup total":           "superficie total",
	"superficie":          "superficie total",
	"superficie cubierta": "superficie cubierta",
	"sup cubierta":        "superficie cubierta",
	"ambientes":           "ambientes",
	"ambiente":            "ambientes",
	"dormitorios":         "dormitorios",
	"dormitorio":          "dormitorios",
	"banos":               "banos",
	"bano":                "banos",
	"cocheras":            "cocheras",
	"cochera":             "cocheras",
	"antiguedad":          "antiguedad",
	"piso":                "piso",
	"pisos":               "piso",
	"disposicion":         "disposicion",
	"orientacion":         "disposicion",
}

func normalizeFeatureKeys(features map[string]string) map[string]string {
	normalized := make(map[string]string, len(features))
	for label, value := range features {
		key, known := featureAliases[foldLabel(label)]
		if !known {
			continue
		}
		// A page can repeat a label; the first non-empty value wins rather
		// than an empty later one overwriting a real reading.
		if existing := normalized[key]; existing != "" {
			continue
		}
		normalized[key] = value
	}
	return normalized
}

var accentFolder = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ü", "u", "Ñ", "n",
)

var nonLabelChars = regexp.MustCompile(`[^a-z0-9 ]+`)
var extraSpace = regexp.MustCompile(`\s+`)

func foldLabel(label string) string {
	folded := accentFolder.Replace(strings.ToLower(strings.TrimSpace(label)))
	folded = nonLabelChars.ReplaceAllString(folded, " ")
	return strings.TrimSpace(extraSpace.ReplaceAllString(folded, " "))
}

// numberPattern matches an Argentine-formatted figure: "." groups thousands
// and "," introduces decimals.
var numberPattern = regexp.MustCompile(`[0-9][0-9.,]*`)

func parseMoney(text string) Money {
	amount := parseDecimal(text)
	if amount == nil {
		return Money{}
	}
	return Money{Amount: amount, Currency: detectCurrency(text)}
}

func detectCurrency(text string) string {
	lowered := strings.ToLower(text)
	// Dollars first: "U$S" and "US$" both contain "$", so testing for pesos
	// first would label every dollar listing as ARS.
	for _, marker := range []string{"u$s", "us$", "usd", "dolar", "dólar"} {
		if strings.Contains(lowered, marker) {
			return "USD"
		}
	}
	if strings.Contains(lowered, "$") || strings.Contains(lowered, "ars") || strings.Contains(lowered, "peso") {
		return "ARS"
	}
	return ""
}

func parseDecimal(text string) *float64 {
	match := numberPattern.FindString(text)
	if match == "" {
		return nil
	}
	match = strings.Trim(match, ".,")

	// A lone comma before exactly three digits is US grouping, not an
	// Argentine decimal. Reading "1,200" as 1.2 would understate a price by a
	// factor of a thousand, and nothing downstream would notice.
	if !strings.Contains(match, ".") {
		if comma := strings.LastIndex(match, ","); comma >= 0 && len(match)-comma-1 == 3 {
			match = strings.ReplaceAll(match, ",", "")
		}
	}
	match = strings.ReplaceAll(match, ".", "")
	match = strings.ReplaceAll(match, ",", ".")

	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return nil
	}
	return &value
}

func parseCount(text string) *int {
	value := parseDecimal(text)
	if value == nil {
		return nil
	}
	count := int(*value)
	return &count
}

// parseAge reads a building's age. "A estrenar" is an age of zero, which is a
// published fact, not a missing one.
func parseAge(text string) *int {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	folded := foldLabel(text)
	if strings.Contains(folded, "estrenar") {
		zero := 0
		return &zero
	}
	if strings.Contains(folded, "construccion") || strings.Contains(folded, "pozo") {
		return nil
	}
	return parseCount(text)
}
