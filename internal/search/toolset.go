package search

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/tools"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/tools/schema"
)

// Toolset is the inventory as a model can read it.
//
// Four tools, in the order a search actually goes: find out what neighborhoods
// exist, search them, open the ones worth opening, and check what that segment
// of the market costs. The set is deliberately small -- a local model spreads
// itself thin across a long tool list, and every capability here has to earn
// the attention it takes from the others.
//
// Nothing in it writes. A buyer agent reading the seller side has no business
// changing it, and a read-only surface is one fewer thing to reason about when
// this moves behind a Bedrock action group.
func Toolset(repository Repository) []tools.Tool {
	return []tools.Tool{
		listNeighborhoodsTool(repository),
		searchListingsTool(repository),
		getListingTool(repository),
		priceStatsTool(repository),
	}
}

func listNeighborhoodsTool(repository Repository) tools.Tool {
	return tools.New(tools.Spec{
		Name: "list_neighborhoods",
		Description: "List the neighborhoods that have listings, with how many and how cheap they start. " +
			"Call this first when the user names a place you have not searched yet: neighborhoods are stored " +
			"as fixed slugs, and searching one that does not exist returns nothing rather than an error.",
		InputSchema: schema.Object("No arguments.", schema.Fields{}),
	}, func(ctx context.Context, _ struct{}) (any, error) {
		neighborhoods, err := repository.Neighborhoods(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"neighborhoods": neighborhoods}, nil
	})
}

func searchListingsTool(repository Repository) tools.Tool {
	return tools.New(tools.Spec{
		Name: "search_listings",
		Description: "Search the inventory. Put the user's hard constraints here -- price, neighborhood, rooms, " +
			"anything they will not go without -- and their softer wishes in preferred_attributes, which order " +
			"the results instead of cutting them. Returns the best matches plus a total, so a total far larger " +
			"than the rows you got back means it is worth narrowing before you answer. Judge the trade-offs " +
			"yourself from what comes back; do not ask this tool to rank taste.",
		InputSchema: searchSchema(),
	}, func(ctx context.Context, query Query) (any, error) {
		validated, err := query.Validate()
		if err != nil {
			return nil, err
		}

		results, err := repository.Search(ctx, validated)
		if err != nil {
			return nil, err
		}

		results.Notes = append(results.Notes, resultNotes(validated, results)...)
		return results, nil
	})
}

type getListingInput struct {
	URL string `json:"url"`
}

func getListingTool(repository Repository) tools.Tool {
	return tools.New(tools.Spec{
		Name: "get_listing",
		Description: "Open one listing in full: the seller's whole description, and every parsed attribute with " +
			"the phrase it came from. Call it on the two or three candidates you are actually weighing. " +
			"Attributes marked 'stated' are the listing's own words; 'inferred' is a reading of them -- quote " +
			"the evidence to the user rather than repeating the label as fact.",
		InputSchema: schema.Object("One listing, by URL.", schema.Fields{
			"url": schema.String("The listing URL, exactly as a search returned it."),
		}, "url"),
	}, func(ctx context.Context, input getListingInput) (any, error) {
		url := strings.TrimSpace(input.URL)
		if url == "" {
			return nil, fmt.Errorf("url is required; pass one exactly as search_listings returned it")
		}
		return repository.ByURL(ctx, url)
	})
}

func priceStatsTool(repository Repository) tools.Tool {
	return tools.New(tools.Spec{
		Name: "neighborhood_price_stats",
		Description: "What one segment of the market costs: the price spread for a neighborhood, operation and " +
			"currency, optionally at a given number of bedrooms. Use it to tell the user whether a listing is " +
			"cheap or dear for where it is, instead of quoting a price with nothing to compare it to. " +
			"Read the sample size before leaning on the median -- a median over five listings is not a rate.",
		InputSchema: schema.Object("One market segment.", schema.Fields{
			"neighborhood": schema.String("Neighborhood slug, from list_neighborhoods."),
			"operation":    schema.Enum("What the listings are offered for.", Operations...),
			"currency":     schema.Enum("Which prices to measure. The two are not converted.", Currencies...),
			"bedrooms":     schema.Integer("Optional. Restrict to listings with exactly this many bedrooms."),
		}, "neighborhood", "operation", "currency"),
	}, func(ctx context.Context, query StatsQuery) (any, error) {
		validated, err := query.Validate()
		if err != nil {
			return nil, err
		}
		return repository.PriceStats(ctx, validated)
	})
}

// resultNotes turns the shape of a result into something the agent can say out
// loud. A filter that quietly removed a third of the market is the failure
// mode this exists to prevent.
func resultNotes(query Query, results Results) []string {
	var notes []string

	if results.ExcludedForMissingPrice > 0 {
		notes = append(notes, fmt.Sprintf(
			"%d listings met every other constraint but never published a price, so they are not in this count. "+
				"Set include_unpriced to true to see them.", results.ExcludedForMissingPrice))
	}
	if results.TotalMatches == 0 && len(query.RequiredAttributes) > 0 {
		notes = append(notes, fmt.Sprintf(
			"Nothing matched. %s were required; try moving the ones the user would trade away into "+
				"preferred_attributes.", joinFilters(query.RequiredAttributes)))
	}
	if results.TotalMatches > len(results.Matches) {
		notes = append(notes, fmt.Sprintf(
			"Showing %d of %d matches, best preference match first.", len(results.Matches), results.TotalMatches))
	}
	if query.MinPrice != nil || query.MaxPrice != nil {
		notes = append(notes, fmt.Sprintf(
			"The price bound applied to %s listings only; prices in the other currency were not converted.", query.Currency))
	}
	return notes
}

func joinFilters(filters []AttributeFilter) string {
	parts := make([]string, 0, len(filters))
	for _, filter := range filters {
		parts = append(parts, filter.String())
	}
	return strings.Join(parts, ", ")
}

// searchSchema describes the query to the model.
//
// The attribute filters are closed over the vocabulary here rather than only in
// Validate. A schema enum costs nothing; a rejected call costs a round trip on
// a local model, and often produces the same invented value again.
func searchSchema() map[string]any {
	attribute := schema.Object("One parsed quality of a property.", schema.Fields{
		"type":  schema.Enum("Which quality.", AttributeTypes()...),
		"value": schema.Enum("Its value. Must be one this type allows -- see the tool description.", AttributeValues()...),
	}, "type", "value")

	return schema.Object("Hard constraints, plus preferences that only affect ordering.", schema.Fields{
		"neighborhoods": schema.Array("Neighborhood slugs. Several are OR-ed. Empty searches everywhere.",
			schema.String("A slug from list_neighborhoods.")),
		"operation": schema.Enum("What the listing is offered for. Almost always worth setting.", Operations...),
		"currency": schema.Enum(
			"Required whenever min_price or max_price is set. ARS and USD are stored side by side and never "+
				"converted, so a bound without this would compare pesos against dollars.", Currencies...),
		"min_price": schema.Number("Lower price bound, in currency."),
		"max_price": schema.Number("Upper price bound, in currency."),
		"include_unpriced": schema.Boolean(
			"Keep listings that never published a price. False by default: an unknown price cannot be shown " +
				"as fitting a budget."),
		"max_expenses_ars":   schema.Number("Upper bound on monthly expensas, in pesos."),
		"min_rooms":          schema.Integer("Least ambientes. Argentine convention: a monoambiente is 1."),
		"max_rooms":          schema.Integer("Most ambientes."),
		"min_bedrooms":       schema.Integer("Least dormitorios."),
		"min_bathrooms":      schema.Integer("Least bathrooms."),
		"min_parking_spaces": schema.Integer("Least cocheras."),
		"min_total_area_m2":  schema.Number("Least total surface, in square metres."),
		"max_age_years":      schema.Integer("Most years old. 0 means a estrenar."),
		"required_attributes": schema.Array(
			"Qualities every result must have. Use sparingly -- each one can empty the result set.", attribute),
		"excluded_attributes": schema.Array("Qualities that disqualify a listing.", attribute),
		"preferred_attributes": schema.Array(
			"Qualities that rank results without excluding any. This is where a wish belongs when the user "+
				"would still consider a listing that misses it.", attribute),
		"limit": schema.Integer(fmt.Sprintf("How many rows to return. Default %d, most %d.", DefaultLimit, MaxLimit)),
	})
}

// AttributeValues is every value the vocabulary admits, across all types, in a
// stable order. It is the union rather than a per-type set because JSON Schema
// can only express the dependency between the two fields with a oneOf branch
// per type -- a document large enough to crowd out the rest of the prompt on a
// small model. The union rules out invented words; Validate rules out the
// mismatched pairs, with an error naming the values the type does allow.
func AttributeValues() []string {
	unique := make(map[string]bool)
	for _, values := range listing.Vocabulary {
		for _, value := range values {
			unique[value] = true
		}
	}
	values := make([]string, 0, len(unique))
	for value := range unique {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
