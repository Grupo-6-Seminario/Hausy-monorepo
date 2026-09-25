// Package listing holds the seller-side inventory model: the facts read
// directly from a listing page and the fuzzy qualities parsed out of its prose.
package listing

import (
	"regexp"
	"slices"
	"time"
)

// Provenance records how firmly an attribute is established. Seller agents are
// incentivised to describe a property favourably, so the buyer side needs to
// tell a claim the listing actually makes from one the parser concluded.
type Provenance string

const (
	// Stated means the description says it in so many words.
	Stated Provenance = "stated"
	// Inferred means the parser concluded it from surrounding prose.
	Inferred Provenance = "inferred"
)

// Attribute is one fuzzy quality of a property, in the same (type, value)
// shape the buyer agent extracts from a natural-language query, so a buyer
// requirement and a listing attribute compare without translation.
type Attribute struct {
	Type       string     `json:"type"`
	Value      string     `json:"value"`
	Provenance Provenance `json:"provenance"`
	Evidence   string     `json:"evidence,omitempty"`
}

// Money is an amount in a currency. Amount is nil when the listing does not
// publish the figure, which is different from publishing zero.
type Money struct {
	Amount   *float64 `json:"amount,omitempty"`
	Currency string   `json:"currency,omitempty"`
}

// Listing is one property as scraped, before and after the parse step. The
// numeric fields are read deterministically from the page rather than inferred
// by a model: a language model is the wrong tool for reading a price.
type Listing struct {
	Rank         int    `json:"rank,omitempty"`
	Source       string `json:"source"`
	URL          string `json:"url"`
	Neighborhood string `json:"neighborhood"`
	Agency       string `json:"agency,omitempty"`
	Address      string `json:"address,omitempty"`
	Description  string `json:"description"`

	Operation     string   `json:"operation,omitempty"`
	Price         Money    `json:"price"`
	Expenses      Money    `json:"expenses"`
	TotalAreaM2   *float64 `json:"total_area_m2,omitempty"`
	CoveredAreaM2 *float64 `json:"covered_area_m2,omitempty"`
	Rooms         *int     `json:"rooms,omitempty"`
	Bedrooms      *int     `json:"bedrooms,omitempty"`
	Bathrooms     *int     `json:"bathrooms,omitempty"`
	ParkingSpaces *int     `json:"parking_spaces,omitempty"`
	AgeYears      *int     `json:"age_years,omitempty"`
	Floor         string   `json:"floor,omitempty"`

	ScrapedAt time.Time `json:"scraped_at"`

	// Filled in by the parse step, not the scrape.
	Attributes  []Attribute `json:"attributes,omitempty"`
	ParsedAt    *time.Time  `json:"parsed_at,omitempty"`
	ParserModel string      `json:"parser_model,omitempty"`
}

// Vocabulary is the closed set of attribute types and the values each accepts.
//
// It is closed on purpose. Both sides of the marketplace must speak the same
// terms or nothing matches -- a buyer asking for "luminoso" against a listing
// tagged "mucha luz" joins on nothing. Keys are English to match the codebase;
// values stay Spanish wherever the Argentine term carries meaning that a
// translation would lose ("contrafrente" is not "rear-facing", and a "parrilla"
// is not a barbecue).
var Vocabulary = map[string][]string{
	"natural_light":    {"high", "medium", "low"},
	"noise_level":      {"quiet", "moderate", "noisy"},
	"orientation":      {"norte", "sur", "este", "oeste", "noreste", "noroeste", "sudeste", "sudoeste"},
	"exposure":         {"frente", "contrafrente", "interno", "lateral"},
	"outdoor_space":    {"balcon", "balcon_terraza", "terraza", "patio", "jardin", "none"},
	"condition":        {"a_estrenar", "excelente", "muy_bueno", "bueno", "a_refaccionar"},
	"furnished":        {"yes", "no"},
	"pets_allowed":     {"yes", "no"},
	"air_conditioning": {"yes", "no"},
	"heating":          {"central", "individual", "losa_radiante", "none"},
	"suitable_for":     {"vivienda", "profesional", "oficina", "comercial", "estudiantes"},
	"amenity":          {"pileta", "gimnasio", "laundry", "coworking", "sum", "seguridad", "parrilla", "ascensor", "cochera", "solarium", "terraza_comun"},
	"transit_access":   {"subte_a", "subte_b", "subte_c", "subte_d", "subte_e", "subte_h", "tren", "colectivo"},
}

// AllowsValue reports whether the vocabulary admits this type and value.
func AllowsValue(attributeType, value string) bool {
	for _, allowed := range Vocabulary[attributeType] {
		if allowed == value {
			return true
		}
	}
	return false
}

// amenityTerms are the words that name each amenity, synonyms included, over
// lowercase accent-folded text. A lavadero is the unit's own unless the ad
// says it is shared, so only a shared one names the building's laundry.
var amenityTerms = map[string]*regexp.Regexp{
	"pileta": regexp.MustCompile(`pileta|piscina|natatorio`), "gimnasio": regexp.MustCompile(`gimnasio|\bgym\b`),
	"laundry": regexp.MustCompile(`laundry|lavanderia|lavadero (comun|compartido)`), "coworking": regexp.MustCompile(`coworking|co working`),
	"sum": regexp.MustCompile(`\bsum\b|salon de (usos|fiestas|eventos)`), "seguridad": regexp.MustCompile(`seguridad|vigilancia|porteria`),
	"parrilla": regexp.MustCompile(`parrilla`), "ascensor": regexp.MustCompile(`ascensor`),
	"cochera": regexp.MustCompile(`cochera|garage|garaje|estacionamiento`), "solarium": regexp.MustCompile(`solarium`),
	"terraza_comun": regexp.MustCompile(`terraza (comun|compartida)`),
}

// NamesAmenity reports whether text names this amenity ("gym" names gimnasio).
func NamesAmenity(value, text string) bool {
	term, ok := amenityTerms[value]
	return ok && term.MatchString(foldLabel(text))
}

// StatedAmenities keeps an amenity only when the listing states it in words
// that name it. A model's inference ("zona segura por contexto") or a quote
// about something else ("seguro Respaldar" is not seguridad) is not the
// listing saying so. Other attribute types pass through.
func StatedAmenities(attributes []Attribute) []Attribute {
	return slices.DeleteFunc(slices.Clone(attributes), func(a Attribute) bool {
		return a.Type == "amenity" && (a.Provenance != Stated || !NamesAmenity(a.Value, a.Evidence))
	})
}
