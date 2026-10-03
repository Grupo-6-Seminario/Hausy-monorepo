// Package postgres persists scraped and parsed listings.
package postgres

import (
	"context"
	"embed"
	"fmt"
	"sort"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Store is the listing repository backed by Postgres.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects to the database and verifies the connection is usable, so a
// misconfigured URI fails here rather than on the first query.
func Open(ctx context.Context, uri string) (*Store, error) {
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() { s.pool.Close() }

// Migrate applies every migration the database has not seen yet, in filename
// order, each in its own transaction. Applied versions are recorded so calling
// it on an up-to-date database does nothing.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("postgres: create schema_migrations: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("postgres: read migrations: %w", err)
	}
	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		versions = append(versions, entry.Name())
	}
	sort.Strings(versions)

	for _, version := range versions {
		var applied bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version,
		).Scan(&applied); err != nil {
			return fmt.Errorf("postgres: check migration %s: %w", version, err)
		}
		if applied {
			continue
		}

		statements, err := migrationFiles.ReadFile("migrations/" + version)
		if err != nil {
			return fmt.Errorf("postgres: read migration %s: %w", version, err)
		}

		// The schema change and its bookkeeping share a transaction, so a
		// migration can never be recorded as applied when it did not run.
		err = s.inTx(ctx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(statements)); err != nil {
				return fmt.Errorf("postgres: apply migration %s: %w", version, err)
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

const upsertListing = `
INSERT INTO listings (
	source, url, neighborhood, agency_id, address, description,
	operation, price_amount, price_currency, expenses_amount, expenses_currency,
	total_area_m2, covered_area_m2, rooms, bedrooms, bathrooms, parking_spaces,
	age_years, floor, scraped_at, parsed_at, parser_model, quality_status
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
	$12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, 'pending'
)
ON CONFLICT (url) DO UPDATE SET
	source = EXCLUDED.source,
	neighborhood = EXCLUDED.neighborhood,
	agency_id = EXCLUDED.agency_id,
	address = EXCLUDED.address,
	description = EXCLUDED.description,
	operation = EXCLUDED.operation,
	price_amount = EXCLUDED.price_amount,
	price_currency = EXCLUDED.price_currency,
	expenses_amount = EXCLUDED.expenses_amount,
	expenses_currency = EXCLUDED.expenses_currency,
	total_area_m2 = EXCLUDED.total_area_m2,
	covered_area_m2 = EXCLUDED.covered_area_m2,
	rooms = EXCLUDED.rooms,
	bedrooms = EXCLUDED.bedrooms,
	bathrooms = EXCLUDED.bathrooms,
	parking_spaces = EXCLUDED.parking_spaces,
	age_years = EXCLUDED.age_years,
	floor = EXCLUDED.floor,
	scraped_at = EXCLUDED.scraped_at,
	parsed_at = EXCLUDED.parsed_at,
	parser_model = EXCLUDED.parser_model,
	ingested_at = now()
RETURNING id`

// Save writes one listing and its attributes, keyed on the URL. Re-running an
// ingest over the same seed file therefore produces the same database rather
// than a second copy of it.
func (s *Store) Save(ctx context.Context, item listing.Listing) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		agencyID, err := upsertAgency(ctx, tx, item.Agency)
		if err != nil {
			return err
		}

		source := item.Source
		if source == "" {
			source = "zonaprop"
		}

		var listingID int64
		err = tx.QueryRow(ctx, upsertListing,
			source, item.URL, item.Neighborhood, agencyID, nullable(item.Address), item.Description,
			nullable(item.Operation), item.Price.Amount, nullable(item.Price.Currency),
			item.Expenses.Amount, nullable(item.Expenses.Currency),
			item.TotalAreaM2, item.CoveredAreaM2, item.Rooms, item.Bedrooms, item.Bathrooms,
			item.ParkingSpaces, item.AgeYears, nullable(item.Floor),
			item.ScrapedAt, item.ParsedAt, nullable(item.ParserModel),
		).Scan(&listingID)
		if err != nil {
			return fmt.Errorf("postgres: save listing %s: %w", item.URL, err)
		}

		// Attributes are replaced wholesale so a re-parse cannot leave a
		// listing carrying two contradictory readings of the same quality.
		if _, err := tx.Exec(ctx, `DELETE FROM listing_attributes WHERE listing_id = $1`, listingID); err != nil {
			return fmt.Errorf("postgres: clear attributes for %s: %w", item.URL, err)
		}
		for _, attribute := range item.Attributes {
			if _, err := tx.Exec(ctx,
				`INSERT INTO listing_attributes (listing_id, type, value, provenance, evidence)
				 VALUES ($1, $2, $3, $4, $5)
				 ON CONFLICT (listing_id, type, value) DO NOTHING`,
				listingID, attribute.Type, attribute.Value, string(attribute.Provenance), nullable(attribute.Evidence),
			); err != nil {
				return fmt.Errorf("postgres: save attribute %s=%s: %w", attribute.Type, attribute.Value, err)
			}
		}
		return nil
	})
}

func upsertAgency(ctx context.Context, tx pgx.Tx, name string) (*int64, error) {
	if name == "" {
		return nil, nil
	}
	var id int64
	// DO UPDATE rather than DO NOTHING so the row is returned on conflict;
	// DO NOTHING returns no row and the insert would look like a failure.
	err := tx.QueryRow(ctx,
		`INSERT INTO agencies (name) VALUES ($1)
		 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`, name).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("postgres: save agency %q: %w", name, err)
	}
	return &id, nil
}

// listingColumns are a listing's own columns, read with listingTargets.
const listingColumns = `l.source, l.url, l.neighborhood, COALESCE(a.name, ''), COALESCE(l.address, ''),
       l.description, COALESCE(l.operation, ''),
       l.price_amount, COALESCE(l.price_currency, ''),
       l.expenses_amount, COALESCE(l.expenses_currency, ''),
       l.total_area_m2, l.covered_area_m2, l.rooms, l.bedrooms, l.bathrooms,
       l.parking_spaces, l.age_years, COALESCE(l.floor, ''),
       l.scraped_at, l.parsed_at, COALESCE(l.parser_model, ''), l.id`

const selectListing = `
SELECT ` + listingColumns + `
FROM listings l
LEFT JOIN agencies a ON a.id = l.agency_id
WHERE l.url = $1 AND l.catalog_status = 'active'`

// listingTargets are the scan destinations of listingColumns, in order.
func listingTargets(item *listing.Listing, id *int64) []any {
	return []any{
		&item.Source, &item.URL, &item.Neighborhood, &item.Agency, &item.Address,
		&item.Description, &item.Operation,
		&item.Price.Amount, &item.Price.Currency,
		&item.Expenses.Amount, &item.Expenses.Currency,
		&item.TotalAreaM2, &item.CoveredAreaM2, &item.Rooms, &item.Bedrooms, &item.Bathrooms,
		&item.ParkingSpaces, &item.AgeYears, &item.Floor,
		&item.ScrapedAt, &item.ParsedAt, &item.ParserModel, id,
	}
}

// ByURL reads back a single listing with its attributes.
func (s *Store) ByURL(ctx context.Context, url string) (listing.Listing, error) {
	var item listing.Listing
	var id int64

	err := s.pool.QueryRow(ctx, selectListing, url).Scan(listingTargets(&item, &id)...)
	if err != nil {
		return listing.Listing{}, fmt.Errorf("postgres: read listing %s: %w", url, err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT type, value, provenance, COALESCE(evidence, '')
		 FROM listing_attributes WHERE listing_id = $1 ORDER BY type, value`, id)
	if err != nil {
		return listing.Listing{}, fmt.Errorf("postgres: read attributes for %s: %w", url, err)
	}
	defer rows.Close()

	for rows.Next() {
		var attribute listing.Attribute
		var provenance string
		if err := rows.Scan(&attribute.Type, &attribute.Value, &provenance, &attribute.Evidence); err != nil {
			return listing.Listing{}, fmt.Errorf("postgres: scan attribute: %w", err)
		}
		attribute.Provenance = listing.Provenance(provenance)
		item.Attributes = append(item.Attributes, attribute)
	}
	return item, rows.Err()
}

// Count reports how many listings are stored.
func (s *Store) Count(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM listings`).Scan(&count)
	return count, err
}

// CountAgencies reports how many distinct agencies are stored.
func (s *Store) CountAgencies(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM agencies`).Scan(&count)
	return count, err
}

// DeleteAll empties the listing tables. It exists for tests and for a clean
// re-ingest; it deliberately leaves the schema in place.
func (s *Store) DeleteAll(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `TRUNCATE listing_attributes, listings, agencies RESTART IDENTITY CASCADE`)
	return err
}

func (s *Store) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// nullable maps the empty string to SQL NULL, keeping "not published" distinct
// from "published as blank".
func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
