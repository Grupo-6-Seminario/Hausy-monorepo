package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

// This file is the read side of the store: the implementation of
// search.Repository. It stays separate from the write side because the two
// change for different reasons -- ingest follows the scraper, search follows
// what the buyer agent needs to ask.

// excerptRunes bounds the prose carried on a search row. Enough to tell two
// listings apart; the rest is what ByURL is for.
const excerptRunes = 220

// builder accumulates a WHERE clause and its arguments together, so a
// condition and the value it tests can never drift apart -- the failure mode
// that turns a filter into a silently wrong answer.
type builder struct {
	clauses []string
	args    []any
}

// param registers a value and returns the placeholder that reads it.
func (b *builder) param(value any) string {
	b.args = append(b.args, value)
	return "$" + strconv.Itoa(len(b.args))
}

func (b *builder) where(clause string) { b.clauses = append(b.clauses, clause) }

func (b *builder) whereClause() string {
	if len(b.clauses) == 0 {
		return "TRUE"
	}
	return strings.Join(b.clauses, "\n  AND ")
}

// applyBaseConditions adds every constraint except price. Price is separate
// because two questions are asked of it -- which listings satisfy the bound,
// and how many were dropped for never publishing one -- and they need the same
// base with different price predicates.
func applyBaseConditions(b *builder, query search.Query) {
	b.where("l.catalog_status = 'active'")
	b.where("l.quality_status IN ('legacy', 'passed')")
	if len(query.Neighborhoods) > 0 {
		b.where("l.neighborhood = ANY(" + b.param(query.Neighborhoods) + ")")
	}
	if query.Operation != "" {
		b.where("l.operation = " + b.param(query.Operation))
	}

	// Every numeric bound is written so a NULL column fails it. A listing that
	// did not publish its area has not shown it meets an area floor.
	for _, bound := range []struct {
		column string
		op     string
		value  any
	}{
		{"l.expenses_amount", "<=", derefFloat(query.MaxExpensesARS)},
		{"l.rooms", ">=", derefInt(query.MinRooms)},
		{"l.rooms", "<=", derefInt(query.MaxRooms)},
		{"l.bedrooms", ">=", derefInt(query.MinBedrooms)},
		{"l.bathrooms", ">=", derefInt(query.MinBathrooms)},
		{"l.parking_spaces", ">=", derefInt(query.MinParkingSpaces)},
		{"l.total_area_m2", ">=", derefFloat(query.MinTotalAreaM2)},
		{"l.age_years", "<=", derefInt(query.MaxAgeYears)},
	} {
		if bound.value == nil {
			continue
		}
		b.where(bound.column + " " + bound.op + " " + b.param(bound.value))
	}
	if query.MaxExpensesARS != nil {
		// Expensas are quoted in pesos. A figure in anything else is not
		// comparable, so it cannot be shown to satisfy the bound.
		b.where("l.expenses_currency = 'ARS'")
	}

	if len(query.RequiredAttributes) > 0 {
		types, values := splitFilters(query.RequiredAttributes)
		b.where(fmt.Sprintf(`(
    SELECT count(*) FROM listing_attributes r
    JOIN unnest(%s::text[], %s::text[]) AS w(type, value) ON w.type = r.type AND w.value = r.value
    WHERE r.listing_id = l.id
  ) = %s`, b.param(types), b.param(values), b.param(len(query.RequiredAttributes))))
	}

	if len(query.ExcludedAttributes) > 0 {
		types, values := splitFilters(query.ExcludedAttributes)
		b.where(fmt.Sprintf(`NOT EXISTS (
    SELECT 1 FROM listing_attributes x
    JOIN unnest(%s::text[], %s::text[]) AS w(type, value) ON w.type = x.type AND w.value = x.value
    WHERE x.listing_id = l.id
  )`, b.param(types), b.param(values)))
	}
}

// applyPriceCondition adds the price predicate, if the query has one.
//
// A price bound is only ever applied within a single currency: the store holds
// ARS and USD side by side with no rate between them, so an unqualified
// comparison would silently answer the wrong question.
func applyPriceCondition(b *builder, query search.Query) {
	if query.Currency == "" {
		return
	}

	priced := []string{"l.price_amount IS NOT NULL", "l.price_currency = " + b.param(query.Currency)}
	if query.MinPrice != nil {
		priced = append(priced, "l.price_amount >= "+b.param(*query.MinPrice))
	}
	if query.MaxPrice != nil {
		priced = append(priced, "l.price_amount <= "+b.param(*query.MaxPrice))
	}

	clause := "(" + strings.Join(priced, " AND ") + ")"
	if query.IncludeUnpriced {
		clause = "(l.price_amount IS NULL OR " + clause + ")"
	}
	b.where(clause)
}

// Search runs one query and returns the page plus the counts that make the
// page honest: how many listings matched in total, and how many were left out
// for never publishing a price.
func (s *Store) Search(ctx context.Context, query search.Query) (search.Results, error) {
	b := &builder{}

	// Preference scoring comes first in the SELECT list, so its placeholders
	// are registered before the WHERE clause's.
	preferenceScore := "0"
	if len(query.PreferredAttributes) > 0 {
		types, values := splitFilters(query.PreferredAttributes)
		// A stated quality is worth more than an inferred one: the listing
		// committed to it in writing, rather than a parser reading it in.
		preferenceScore = fmt.Sprintf(`COALESCE((
      SELECT sum(CASE WHEN p.provenance = 'stated' THEN 2 ELSE 1 END)
      FROM listing_attributes p
      JOIN unnest(%s::text[], %s::text[]) AS w(type, value) ON w.type = p.type AND w.value = p.value
      WHERE p.listing_id = l.id
    ), 0)`, b.param(types), b.param(values))
	}

	applyBaseConditions(b, query)
	applyPriceCondition(b, query)

	rowsSQL := fmt.Sprintf(`
SELECT l.url, l.neighborhood, COALESCE(a.name, ''), COALESCE(l.address, ''), COALESCE(l.operation, ''),
       l.price_amount, COALESCE(l.price_currency, ''),
       l.expenses_amount, COALESCE(l.expenses_currency, ''),
       l.total_area_m2, l.rooms, l.bedrooms, l.bathrooms, l.parking_spaces,
       l.age_years, COALESCE(l.floor, ''), l.description, l.parsed_at IS NULL,
       %s AS preference_score,
       COALESCE((
         SELECT json_agg(json_build_object(
                  'type', at.type, 'value', at.value,
                  'provenance', at.provenance, 'evidence', COALESCE(at.evidence, ''))
                ORDER BY at.type, at.value)
         FROM listing_attributes at WHERE at.listing_id = l.id
       ), '[]'::json) AS attributes,
       count(*) OVER () AS total_matches
FROM listings l
LEFT JOIN agencies a ON a.id = l.agency_id
WHERE %s
ORDER BY preference_score DESC, l.price_amount ASC NULLS LAST, l.id ASC
LIMIT %s`, preferenceScore, b.whereClause(), b.param(query.Limit))

	rows, err := s.pool.Query(ctx, rowsSQL, b.args...)
	if err != nil {
		return search.Results{}, fmt.Errorf("postgres: search: %w", err)
	}
	defer rows.Close()

	var results search.Results
	for rows.Next() {
		var (
			match       search.Match
			description string
			score       int
			attributes  []byte
			total       int
		)
		if err := rows.Scan(
			&match.URL, &match.Neighborhood, &match.Agency, &match.Address, &match.Operation,
			&match.Price.Amount, &match.Price.Currency,
			&match.Expenses.Amount, &match.Expenses.Currency,
			&match.TotalAreaM2, &match.Rooms, &match.Bedrooms, &match.Bathrooms, &match.ParkingSpaces,
			&match.AgeYears, &match.Floor, &description, &match.Unparsed,
			&score, &attributes, &total,
		); err != nil {
			return search.Results{}, fmt.Errorf("postgres: scan search row: %w", err)
		}

		if err := json.Unmarshal(attributes, &match.Attributes); err != nil {
			return search.Results{}, fmt.Errorf("postgres: decode attributes for %s: %w", match.URL, err)
		}
		// Evidence is dropped from a search row on purpose: it is what decides
		// between two candidates, not what finds them, and carrying it on
		// every row spends the agent's context before it gets there.
		for i := range match.Attributes {
			match.Attributes[i].Evidence = ""
		}

		match.Excerpt = excerpt(description)
		match.MatchedPreferences, match.MissedPreferences = splitPreferences(query.PreferredAttributes, match.Attributes)
		match.Rank = len(results.Matches) + 1

		results.TotalMatches = total
		results.Matches = append(results.Matches, match)
	}
	if err := rows.Err(); err != nil {
		return search.Results{}, fmt.Errorf("postgres: search: %w", err)
	}

	excluded, err := s.countUnpriced(ctx, query)
	if err != nil {
		return search.Results{}, err
	}
	results.ExcludedForMissingPrice = excluded

	return results, nil
}

// countUnpriced answers "what did the price filter hide?". It re-runs the
// non-price constraints, so the number describes listings the user would
// otherwise have seen -- not the whole inventory's missing prices.
func (s *Store) countUnpriced(ctx context.Context, query search.Query) (int, error) {
	if query.Currency == "" || query.IncludeUnpriced {
		return 0, nil
	}

	b := &builder{}
	applyBaseConditions(b, query)
	b.where("l.price_amount IS NULL")

	var count int
	err := s.pool.QueryRow(ctx,
		"SELECT count(*) FROM listings l WHERE "+b.whereClause(), b.args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("postgres: count unpriced: %w", err)
	}
	return count, nil
}

const selectNeighborhoods = `
SELECT neighborhood,
       count(*),
       count(*) FILTER (WHERE operation IN ('alquiler', 'alquiler_temporal')),
       count(*) FILTER (WHERE operation = 'venta'),
       min(price_amount) FILTER (WHERE price_currency = 'USD'),
       min(price_amount) FILTER (WHERE price_currency = 'ARS')
FROM listings
WHERE catalog_status = 'active' AND quality_status IN ('legacy', 'passed')
GROUP BY neighborhood
ORDER BY count(*) DESC, neighborhood ASC`

// Neighborhoods lists what the inventory actually covers. An agent that guesses
// a slug gets an empty result rather than an error, so it needs somewhere to
// look the real ones up.
func (s *Store) Neighborhoods(ctx context.Context) ([]search.Neighborhood, error) {
	rows, err := s.pool.Query(ctx, selectNeighborhoods)
	if err != nil {
		return nil, fmt.Errorf("postgres: neighborhoods: %w", err)
	}
	defer rows.Close()

	var neighborhoods []search.Neighborhood
	for rows.Next() {
		var item search.Neighborhood
		if err := rows.Scan(&item.Slug, &item.Listings, &item.ForRent, &item.ForSale,
			&item.MinPriceUSD, &item.MinPriceARS); err != nil {
			return nil, fmt.Errorf("postgres: scan neighborhood: %w", err)
		}
		neighborhoods = append(neighborhoods, item)
	}
	return neighborhoods, rows.Err()
}

// PriceStats describes one segment of the market, so a price can be reported
// as cheap or dear for where it is instead of quoted into a vacuum.
func (s *Store) PriceStats(ctx context.Context, query search.StatsQuery) (search.Stats, error) {
	b := &builder{}
	b.where("l.neighborhood = " + b.param(query.Neighborhood))
	b.where("l.operation = " + b.param(query.Operation))
	b.where("l.price_currency = " + b.param(query.Currency))
	b.where("l.price_amount IS NOT NULL")
	b.where("l.catalog_status = 'active'")
	b.where("l.quality_status IN ('legacy', 'passed')")
	if query.Bedrooms != nil {
		b.where("l.bedrooms = " + b.param(*query.Bedrooms))
	}

	statsSQL := `
SELECT count(*),
       min(l.price_amount)::double precision,
       percentile_cont(0.25) WITHIN GROUP (ORDER BY l.price_amount::double precision),
       percentile_cont(0.50) WITHIN GROUP (ORDER BY l.price_amount::double precision),
       percentile_cont(0.75) WITHIN GROUP (ORDER BY l.price_amount::double precision),
       max(l.price_amount)::double precision,
       percentile_cont(0.50) WITHIN GROUP (ORDER BY (l.price_amount / l.total_area_m2)::double precision)
         FILTER (WHERE l.total_area_m2 > 0),
       percentile_cont(0.50) WITHIN GROUP (ORDER BY l.expenses_amount::double precision)
         FILTER (WHERE l.expenses_currency = 'ARS' AND l.expenses_amount IS NOT NULL)
FROM listings l
WHERE ` + b.whereClause()

	stats := search.Stats{
		Neighborhood: query.Neighborhood,
		Operation:    query.Operation,
		Currency:     query.Currency,
		Bedrooms:     query.Bedrooms,
	}
	if err := s.pool.QueryRow(ctx, statsSQL, b.args...).Scan(
		&stats.SampleSize, &stats.Min, &stats.P25, &stats.Median, &stats.P75, &stats.Max,
		&stats.MedianPricePerM2, &stats.MedianExpensesARS,
	); err != nil {
		return search.Stats{}, fmt.Errorf("postgres: price stats: %w", err)
	}

	stats.Notes = statsNotes(stats)
	return stats, nil
}

// statsNotes says out loud how much weight the numbers will bear. A median over
// a handful of listings is an anecdote, and reporting it as a market rate is
// the way this tool would mislead if it could.
func statsNotes(stats search.Stats) []string {
	switch {
	case stats.SampleSize == 0:
		return []string{"No listings are stored for this neighborhood, operation and currency, so there is nothing to compare against."}
	case stats.SampleSize < 8:
		return []string{fmt.Sprintf(
			"Only %d listings back these figures. Treat them as a rough indication, not a market rate.", stats.SampleSize)}
	default:
		return nil
	}
}

// splitPreferences reports which of the query's preferences a listing meets and
// which it misses, so an ordering can be explained instead of asserted.
func splitPreferences(preferred []search.AttributeFilter, attributes []listing.Attribute) (matched, missed []search.AttributeFilter) {
	present := make(map[search.AttributeFilter]bool, len(attributes))
	for _, attribute := range attributes {
		present[search.AttributeFilter{Type: attribute.Type, Value: attribute.Value}] = true
	}
	for _, preference := range preferred {
		if present[preference] {
			matched = append(matched, preference)
		} else {
			missed = append(missed, preference)
		}
	}
	return matched, missed
}

func splitFilters(filters []search.AttributeFilter) (types, values []string) {
	types = make([]string, 0, len(filters))
	values = make([]string, 0, len(filters))
	for _, filter := range filters {
		types = append(types, filter.Type)
		values = append(values, filter.Value)
	}
	return types, values
}

func excerpt(description string) string {
	runes := []rune(strings.Join(strings.Fields(description), " "))
	if len(runes) <= excerptRunes {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:excerptRunes])) + "…"
}

// derefFloat and derefInt turn an unset bound into an untyped nil, which the
// caller reads as "no condition" rather than as a zero to compare against.
func derefFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func derefInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
