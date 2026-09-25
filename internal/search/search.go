// Package search is the read side of the listing store: the query a buyer
// agent can ask, the results it gets back, and the rules that keep an
// ambiguous question from becoming a wrong answer.
//
// The division of labour follows docs/DATA_MODEL.md. Hard constraints -- price,
// rooms, neighborhood, a required amenity -- are SQL, because a model is the
// wrong tool for reading a number. Preferences are ranked, and the trade-off
// reasoning on the shortlist belongs to the agent, not here.
package search

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

// Result-set sizing. The default is small because the agent reads every row it
// gets; the cap is what stops a model from asking for the whole inventory.
const (
	DefaultLimit = 10
	MaxLimit     = 50
)

// Currencies and Operations are the closed sets the schema stores. They are
// listed rather than inferred so an invalid value fails at the boundary with a
// message naming the alternatives, instead of quietly matching nothing.
var (
	Currencies = []string{"ARS", "USD"}
	Operations = []string{"alquiler", "alquiler_temporal", "venta"}
)

// AttributeFilter names one parsed quality, in the same (type, value) shape
// the listing store holds and the buyer agent emits.
type AttributeFilter struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func (f AttributeFilter) String() string { return f.Type + "=" + f.Value }

// Query is one search over the inventory.
//
// Every bound is a pointer because "no upper bound on price" and "an upper
// bound of zero" are different questions, and the zero value would collapse
// them into the second.
type Query struct {
	Neighborhoods []string `json:"neighborhoods,omitempty"`
	Operation     string   `json:"operation,omitempty"`

	// Currency is mandatory whenever MinPrice or MaxPrice is set: the store
	// holds ARS and USD side by side with no exchange rate between them.
	Currency string   `json:"currency,omitempty"`
	MinPrice *float64 `json:"min_price,omitempty"`
	MaxPrice *float64 `json:"max_price,omitempty"`

	// IncludeUnpriced keeps listings that never published a price. They are
	// dropped by default: a listing whose price is unknown cannot be shown as
	// satisfying a price ceiling.
	IncludeUnpriced bool `json:"include_unpriced,omitempty"`

	// MaxExpensesARS bounds the expensas. The currency is in the name because
	// expensas are quoted in pesos; a listing that published them in anything
	// else is not comparable and is left out of the bound.
	MaxExpensesARS *float64 `json:"max_expenses_ars,omitempty"`

	MinRooms         *int     `json:"min_rooms,omitempty"`
	MaxRooms         *int     `json:"max_rooms,omitempty"`
	MinBedrooms      *int     `json:"min_bedrooms,omitempty"`
	MinBathrooms     *int     `json:"min_bathrooms,omitempty"`
	MinParkingSpaces *int     `json:"min_parking_spaces,omitempty"`
	MinTotalAreaM2   *float64 `json:"min_total_area_m2,omitempty"`
	MaxAgeYears      *int     `json:"max_age_years,omitempty"`

	// RequiredAttributes must all be present; ExcludedAttributes must all be
	// absent. Use them only for qualities the user will not trade away --
	// every one of them shrinks the result set to nothing quite quickly.
	RequiredAttributes []AttributeFilter `json:"required_attributes,omitempty"`
	ExcludedAttributes []AttributeFilter `json:"excluded_attributes,omitempty"`

	// PreferredAttributes do not filter. They order the results, so a listing
	// that misses one still appears -- lower down, with the miss visible.
	PreferredAttributes []AttributeFilter `json:"preferred_attributes,omitempty"`

	Limit int `json:"limit,omitempty"`
}

// Validate returns the query normalised for execution, or an error written to
// be read by whoever asked -- including a language model, which will only fix
// a call if the rejection says what a valid one looks like.
func (q Query) Validate() (Query, error) {
	q.Neighborhoods = normaliseSlugs(q.Neighborhoods)

	q.Operation = strings.ToLower(strings.TrimSpace(q.Operation))
	if q.Operation != "" && !contains(Operations, q.Operation) {
		return Query{}, fmt.Errorf("operation %q is not stored; use one of %s", q.Operation, strings.Join(Operations, ", "))
	}

	q.Currency = strings.ToUpper(strings.TrimSpace(q.Currency))
	if q.Currency != "" && !contains(Currencies, q.Currency) {
		return Query{}, fmt.Errorf("currency %q is not stored; use one of %s", q.Currency, strings.Join(Currencies, ", "))
	}
	if (q.MinPrice != nil || q.MaxPrice != nil) && q.Currency == "" {
		return Query{}, fmt.Errorf(
			"a price bound needs a currency: prices are stored in %s with no exchange rate between them, "+
				"so an unqualified bound would compare a USD figure against an ARS one",
			strings.Join(Currencies, " and "))
	}
	if q.MinPrice != nil && q.MaxPrice != nil && *q.MinPrice > *q.MaxPrice {
		return Query{}, fmt.Errorf("min_price (%.2f) is above max_price (%.2f)", *q.MinPrice, *q.MaxPrice)
	}
	if q.MinRooms != nil && q.MaxRooms != nil && *q.MinRooms > *q.MaxRooms {
		return Query{}, fmt.Errorf("min_rooms (%d) is above max_rooms (%d)", *q.MinRooms, *q.MaxRooms)
	}

	for label, value := range map[string]*float64{
		"min_price": q.MinPrice, "max_price": q.MaxPrice,
		"max_expenses_ars": q.MaxExpensesARS, "min_total_area_m2": q.MinTotalAreaM2,
	} {
		if value != nil && *value < 0 {
			return Query{}, fmt.Errorf("%s cannot be negative, got %.2f", label, *value)
		}
	}
	for label, value := range map[string]*int{
		"min_rooms": q.MinRooms, "max_rooms": q.MaxRooms, "min_bedrooms": q.MinBedrooms,
		"min_bathrooms": q.MinBathrooms, "min_parking_spaces": q.MinParkingSpaces,
		"max_age_years": q.MaxAgeYears,
	} {
		if value != nil && *value < 0 {
			return Query{}, fmt.Errorf("%s cannot be negative, got %d", label, *value)
		}
	}

	for label, filters := range map[string][]AttributeFilter{
		"required_attributes":  q.RequiredAttributes,
		"excluded_attributes":  q.ExcludedAttributes,
		"preferred_attributes": q.PreferredAttributes,
	} {
		for _, filter := range filters {
			if err := validateAttribute(label, filter); err != nil {
				return Query{}, err
			}
		}
	}

	if q.Limit <= 0 {
		q.Limit = DefaultLimit
	}
	if q.Limit > MaxLimit {
		q.Limit = MaxLimit
	}
	return q, nil
}

func validateAttribute(label string, filter AttributeFilter) error {
	allowed, known := listing.Vocabulary[filter.Type]
	if !known {
		return fmt.Errorf("%s: %q is not an attribute type; the types are %s",
			label, filter.Type, strings.Join(AttributeTypes(), ", "))
	}
	if !listing.AllowsValue(filter.Type, filter.Value) {
		return fmt.Errorf("%s: %q is not a value for %q; the values are %s",
			label, filter.Value, filter.Type, strings.Join(allowed, ", "))
	}
	return nil
}

// AttributeTypes lists the vocabulary's types in a stable order. Map iteration
// is random, and a tool schema that reshuffles between builds is a schema no
// prompt cache can hold on to.
func AttributeTypes() []string {
	types := make([]string, 0, len(listing.Vocabulary))
	for attributeType := range listing.Vocabulary {
		types = append(types, attributeType)
	}
	sort.Strings(types)
	return types
}

// normaliseSlugs maps what a person writes onto what the store holds: the
// ingest keys neighborhoods as lowercase underscore slugs, so "Puerto Madero"
// and "puerto_madero" have to reach SQL as the same string.
func normaliseSlugs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	slugs := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		slug := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), "_")
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		slugs = append(slugs, slug)
	}
	return slugs
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// Match is one listing as a search returns it: enough to compare against the
// others, not enough to decide. Evidence is deliberately absent -- it is what
// Get is for, once the shortlist is short.
type Match struct {
	Rank         int    `json:"rank"`
	URL          string `json:"url"`
	Neighborhood string `json:"neighborhood"`
	Agency       string `json:"agency,omitempty"`
	Address      string `json:"address,omitempty"`
	Operation    string `json:"operation,omitempty"`

	Price    listing.Money `json:"price"`
	Expenses listing.Money `json:"expenses"`

	TotalAreaM2   *float64 `json:"total_area_m2,omitempty"`
	Rooms         *int     `json:"rooms,omitempty"`
	Bedrooms      *int     `json:"bedrooms,omitempty"`
	Bathrooms     *int     `json:"bathrooms,omitempty"`
	ParkingSpaces *int     `json:"parking_spaces,omitempty"`
	AgeYears      *int     `json:"age_years,omitempty"`
	Floor         string   `json:"floor,omitempty"`

	// Attributes are the parsed qualities, each carrying whether the listing
	// stated it or the parser inferred it.
	Attributes []listing.Attribute `json:"attributes,omitempty"`

	// MatchedPreferences and MissedPreferences split the query's preferences
	// over this listing, so a ranking can be explained rather than asserted.
	MatchedPreferences []AttributeFilter `json:"matched_preferences,omitempty"`
	MissedPreferences  []AttributeFilter `json:"missed_preferences,omitempty"`

	// Excerpt is the opening of the seller's own prose, for the qualities no
	// vocabulary covers. The full text is in Get.
	Excerpt string `json:"excerpt,omitempty"`

	// Unparsed marks a listing whose prose was never read, so its empty
	// attribute list means "not looked at", not "does not have any".
	Unparsed bool `json:"unparsed,omitempty"`
}

// Results is what one search produced.
type Results struct {
	// TotalMatches counts every listing satisfying the constraints; Matches
	// holds only the first Limit of them, best preference match first.
	TotalMatches int     `json:"total_matches"`
	Matches      []Match `json:"matches"`

	// ExcludedForMissingPrice counts listings that met every other constraint
	// but never published a price. Surfacing the number is what keeps a price
	// filter from silently hiding a chunk of the market.
	ExcludedForMissingPrice int `json:"excluded_for_missing_price,omitempty"`

	// Notes are plain-language caveats meant to be passed on to the user.
	Notes []string `json:"notes,omitempty"`
}
