package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Add stores deterministic agency-authored listing facts under one owner.
func (s *Store) Add(ctx context.Context, owner agency.Owner, input agency.PropertyInput) (agency.Property, error) {
	var err error
	input, err = agency.PreparePropertyInput(input)
	if err != nil {
		return agency.Property{}, err
	}
	ownerID, err := strconv.ParseInt(owner.ID, 10, 64)
	if err != nil {
		return agency.Property{}, agency.ValidationError{Field: "owner", Reason: "no es válido"}
	}

	var property agency.Property
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		agencyID, err := upsertAgency(ctx, tx, owner.Name)
		if err != nil {
			return err
		}
		var id int64
		err = tx.QueryRow(ctx, `
INSERT INTO listings (
    source, url, neighborhood, agency_id, address, description,
    operation, price_amount, price_currency, expenses_amount, expenses_currency,
    total_area_m2, covered_area_m2, rooms, bedrooms, bathrooms, parking_spaces,
    age_years, floor, scraped_at, owner_user_id, catalog_status, quality_status
) VALUES (
    'agency', $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16,
    $17, $18, now(), $19, 'active', 'pending'
) RETURNING id`,
			input.URL, input.Neighborhood, agencyID, nullable(input.Address), input.Description,
			input.Operation, input.Price.Amount, nullable(input.Price.Currency),
			input.Expenses.Amount, nullable(input.Expenses.Currency),
			input.TotalAreaM2, input.CoveredAreaM2, input.Rooms, input.Bedrooms,
			input.Bathrooms, input.ParkingSpaces, input.AgeYears, nullable(input.Floor), ownerID,
		).Scan(&id)
		if err != nil {
			return err
		}
		property = agency.Property{
			ID:            strconv.FormatInt(id, 10),
			Agency:        owner.Name,
			Source:        "agency",
			PropertyInput: input,
		}
		return nil
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return agency.Property{}, agency.ValidationError{Field: "url", Reason: "ya pertenece a otra propiedad"}
	}
	if err != nil {
		return agency.Property{}, fmt.Errorf("postgres: add agency property: %w", err)
	}
	return property, nil
}

// List returns active properties owned by one realtor, including contact
// totals derived from immutable events.
func (s *Store) List(ctx context.Context, ownerID string) ([]agency.Property, error) {
	parsedOwnerID, err := strconv.ParseInt(ownerID, 10, 64)
	if err != nil {
		return nil, agency.ValidationError{Field: "owner", Reason: "no es válido"}
	}
	rows, err := s.pool.Query(ctx, `
SELECT l.id, COALESCE(a.name, ''), l.source, l.url, l.neighborhood,
       COALESCE(l.address, ''), l.description, COALESCE(l.operation, ''),
       l.price_amount, COALESCE(l.price_currency, ''),
       l.expenses_amount, COALESCE(l.expenses_currency, ''),
       l.total_area_m2, l.covered_area_m2, l.rooms, l.bedrooms, l.bathrooms,
       l.parking_spaces, l.age_years, COALESCE(l.floor, ''),
       count(ci.intent_id)
FROM listings l
LEFT JOIN agencies a ON a.id = l.agency_id
LEFT JOIN listing_contact_intents ci ON ci.listing_id = l.id
WHERE l.owner_user_id = $1 AND l.catalog_status = 'active'
GROUP BY l.id, a.name
ORDER BY l.id DESC`, parsedOwnerID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list agency catalog: %w", err)
	}
	defer rows.Close()

	properties := make([]agency.Property, 0)
	for rows.Next() {
		var property agency.Property
		var id int64
		if err := rows.Scan(
			&id, &property.Agency, &property.Source, &property.URL, &property.Neighborhood,
			&property.Address, &property.Description, &property.Operation,
			&property.Price.Amount, &property.Price.Currency,
			&property.Expenses.Amount, &property.Expenses.Currency,
			&property.TotalAreaM2, &property.CoveredAreaM2, &property.Rooms, &property.Bedrooms,
			&property.Bathrooms, &property.ParkingSpaces, &property.AgeYears, &property.Floor,
			&property.ContactCount,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan agency property: %w", err)
		}
		property.ID = strconv.FormatInt(id, 10)
		properties = append(properties, property)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list agency catalog: %w", err)
	}
	return properties, nil
}

// Update replaces editable deterministic facts on one owned active property.
func (s *Store) Update(ctx context.Context, ownerID, propertyID string, input agency.PropertyInput) (agency.Property, error) {
	var err error
	input, err = agency.PreparePropertyInput(input)
	if err != nil {
		return agency.Property{}, err
	}
	parsedOwnerID, parsedPropertyID, err := catalogIDs(ownerID, propertyID)
	if err != nil {
		return agency.Property{}, err
	}
	result, err := s.pool.Exec(ctx, `
UPDATE listings SET
    url = $1, neighborhood = $2, address = $3, description = $4,
    operation = $5, price_amount = $6, price_currency = $7,
    expenses_amount = $8, expenses_currency = $9,
    total_area_m2 = $10, covered_area_m2 = $11, rooms = $12,
    bedrooms = $13, bathrooms = $14, parking_spaces = $15,
    age_years = $16, floor = $17, ingested_at = now()
WHERE id = $18 AND owner_user_id = $19 AND catalog_status = 'active'`,
		input.URL, input.Neighborhood, nullable(input.Address), input.Description,
		input.Operation, input.Price.Amount, nullable(input.Price.Currency),
		input.Expenses.Amount, nullable(input.Expenses.Currency),
		input.TotalAreaM2, input.CoveredAreaM2, input.Rooms, input.Bedrooms,
		input.Bathrooms, input.ParkingSpaces, input.AgeYears, nullable(input.Floor),
		parsedPropertyID, parsedOwnerID,
	)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return agency.Property{}, agency.ValidationError{Field: "url", Reason: "ya pertenece a otra propiedad"}
	}
	if err != nil {
		return agency.Property{}, fmt.Errorf("postgres: update agency property: %w", err)
	}
	if result.RowsAffected() == 0 {
		return agency.Property{}, agency.ErrPropertyNotFound
	}
	properties, err := s.List(ctx, ownerID)
	if err != nil {
		return agency.Property{}, err
	}
	for _, property := range properties {
		if property.ID == propertyID {
			return property, nil
		}
	}
	return agency.Property{}, agency.ErrPropertyNotFound
}

// Remove archives an owned property. Canonical facts and contact history stay
// available for audit, while buyer search and the active catalog omit it.
func (s *Store) Remove(ctx context.Context, ownerID, propertyID string) error {
	parsedOwnerID, parsedPropertyID, err := catalogIDs(ownerID, propertyID)
	if err != nil {
		return err
	}
	result, err := s.pool.Exec(ctx, `
UPDATE listings SET catalog_status = 'archived', ingested_at = now()
WHERE id = $1 AND owner_user_id = $2 AND catalog_status = 'active'`, parsedPropertyID, parsedOwnerID)
	if err != nil {
		return fmt.Errorf("postgres: archive agency property: %w", err)
	}
	if result.RowsAffected() == 0 {
		return agency.ErrPropertyNotFound
	}
	return nil
}

func catalogIDs(ownerID, propertyID string) (int64, int64, error) {
	parsedOwnerID, err := strconv.ParseInt(ownerID, 10, 64)
	if err != nil {
		return 0, 0, agency.ValidationError{Field: "owner", Reason: "no es válido"}
	}
	parsedPropertyID, err := strconv.ParseInt(propertyID, 10, 64)
	if err != nil {
		return 0, 0, agency.ErrPropertyNotFound
	}
	return parsedOwnerID, parsedPropertyID, nil
}

// RecordContactIntent persists an immutable event and reports whether this
// call created it. The UUID primary key makes retries idempotent.
func (s *Store) RecordContactIntent(ctx context.Context, propertyID string, intent agency.ContactIntent) (agency.ContactReceipt, error) {
	if err := agency.ValidateContactIntent(intent); err != nil {
		return agency.ContactReceipt{}, err
	}
	parsedPropertyID, err := strconv.ParseInt(propertyID, 10, 64)
	if err != nil {
		return agency.ContactReceipt{}, agency.ErrPropertyNotFound
	}

	receipt := agency.ContactReceipt{
		IntentID:  intent.ID,
		ListingID: propertyID,
		Recorded:  true,
	}
	var insertedListingID int64
	err = s.pool.QueryRow(ctx, `
INSERT INTO listing_contact_intents (intent_id, listing_id, source)
SELECT $1::uuid, l.id, $3
FROM listings l
WHERE l.id = $2 AND l.catalog_status = 'active'
ON CONFLICT (intent_id) DO NOTHING
RETURNING listing_id`, intent.ID, parsedPropertyID, string(intent.Source)).Scan(&insertedListingID)
	if err == nil {
		receipt.Created = true
		return receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return agency.ContactReceipt{}, fmt.Errorf("postgres: record contact intent: %w", err)
	}

	var recordedListingID int64
	err = s.pool.QueryRow(ctx,
		`SELECT listing_id FROM listing_contact_intents WHERE intent_id = $1::uuid`, intent.ID,
	).Scan(&recordedListingID)
	switch {
	case err == nil && recordedListingID == parsedPropertyID:
		return receipt, nil
	case err == nil:
		return agency.ContactReceipt{}, agency.ErrContactIntentConflict
	case errors.Is(err, pgx.ErrNoRows):
		return agency.ContactReceipt{}, agency.ErrPropertyNotFound
	default:
		return agency.ContactReceipt{}, fmt.Errorf("postgres: read contact intent: %w", err)
	}
}
