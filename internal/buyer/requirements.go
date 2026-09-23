package buyer

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

// requirementsFromQuery renders the search that ran as the requirements the
// interface shows.
//
// Reading them off the real query rather than asking the model to restate them
// costs no extra call and cannot drift: what the user sees listed is what was
// actually asked of the database.
func requirementsFromQuery(query search.Query) []Requirement {
	requirements := make([]Requirement, 0, 8)
	add := func(kind, value string) {
		if value != "" {
			requirements = append(requirements, Requirement{Type: kind, Value: value})
		}
	}

	for _, neighborhood := range query.Neighborhoods {
		add("neighborhood", neighborhood)
	}
	add("operation", query.Operation)
	if query.MinPrice != nil {
		add("min_price", formatAmount(*query.MinPrice)+" "+query.Currency)
	}
	if query.MaxPrice != nil {
		add("max_price", formatAmount(*query.MaxPrice)+" "+query.Currency)
	}
	if query.MaxExpensesARS != nil {
		add("max_expenses", formatAmount(*query.MaxExpensesARS)+" ARS")
	}
	for kind, value := range map[string]*int{
		"min_rooms": query.MinRooms, "max_rooms": query.MaxRooms,
		"min_bedrooms": query.MinBedrooms, "min_bathrooms": query.MinBathrooms,
		"min_parking_spaces": query.MinParkingSpaces, "max_age_years": query.MaxAgeYears,
	} {
		if value != nil {
			add(kind, strconv.Itoa(*value))
		}
	}
	if query.MinTotalAreaM2 != nil {
		add("min_total_area_m2", formatAmount(*query.MinTotalAreaM2))
	}
	for _, filter := range query.RequiredAttributes {
		add(filter.Type, filter.Value)
	}
	for _, filter := range query.PreferredAttributes {
		add("prefiere_"+filter.Type, filter.Value)
	}
	for _, filter := range query.ExcludedAttributes {
		add("excluye_"+filter.Type, filter.Value)
	}

	// Map iteration above is unordered; a list that reshuffles between turns
	// looks to the user like the agent changed its mind.
	sortRequirements(requirements)
	return requirements
}

func formatAmount(value float64) string {
	return strings.TrimSuffix(strings.TrimRight(strconv.FormatFloat(value, 'f', 2, 64), "0"), ".")
}

func sortRequirements(requirements []Requirement) {
	sort.SliceStable(requirements, func(i, j int) bool {
		if requirements[i].Type != requirements[j].Type {
			return requirements[i].Type < requirements[j].Type
		}
		return requirements[i].Value < requirements[j].Value
	})
}
