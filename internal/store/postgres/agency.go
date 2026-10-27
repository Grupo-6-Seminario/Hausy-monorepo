package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Add stores deterministic agency-authored listing facts under one owner.
func (s *Store) Add(ctx context.Context, owner agency.Owner, input agency.PropertyInput) (agency.Property, error) {
	input, err := s.preparePropertyInput(ctx, input)
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
		if err := replaceRules(ctx, tx, id, input.Requirements); err != nil {
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
       count(ci.intent_id),
       (SELECT count(*) FROM listing_inquiries i WHERE i.listing_id = l.id),
       (SELECT json_agg(json_build_object(
                  'fact', r.fact, 'operator', r.operator, 'values', r."values",
                  'hardness', r.hardness, 'visibility', r.visibility,
                  'source', r.source, 'evidence', r.evidence)
                ORDER BY r.id)
        FROM listing_eligibility_rules r WHERE r.listing_id = l.id AND r.source = 'declared')
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
		var requirements []byte
		if err := rows.Scan(
			&id, &property.Agency, &property.Source, &property.URL, &property.Neighborhood,
			&property.Address, &property.Description, &property.Operation,
			&property.Price.Amount, &property.Price.Currency,
			&property.Expenses.Amount, &property.Expenses.Currency,
			&property.TotalAreaM2, &property.CoveredAreaM2, &property.Rooms, &property.Bedrooms,
			&property.Bathrooms, &property.ParkingSpaces, &property.AgeYears, &property.Floor,
			&property.ContactCount, &property.InquiryCount, &requirements,
		); err != nil {
			return nil, fmt.Errorf("postgres: scan agency property: %w", err)
		}
		property.ID = strconv.FormatInt(id, 10)
		property.Requirements = []eligibility.Rule{}
		if requirements != nil {
			if err := json.Unmarshal(requirements, &property.Requirements); err != nil {
				return nil, fmt.Errorf("postgres: requirements of property %d: %w", id, err)
			}
		}
		properties = append(properties, property)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list agency catalog: %w", err)
	}
	return properties, nil
}

// Update replaces editable deterministic facts on one owned active property.
func (s *Store) Update(ctx context.Context, ownerID, propertyID string, input agency.PropertyInput) (agency.Property, error) {
	input, err := s.preparePropertyInput(ctx, input)
	if err != nil {
		return agency.Property{}, err
	}
	parsedOwnerID, parsedPropertyID, err := catalogIDs(ownerID, propertyID)
	if err != nil {
		return agency.Property{}, err
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		// The shared content-change trigger invalidates quality decisions. An
		// agency edit may not bypass a prior withdrawal while doing so.
		var qualityStatus string
		var qualityEvidence []byte
		var qualityCheckedAt *time.Time
		err := tx.QueryRow(ctx, `SELECT quality_status, quality_evidence, quality_checked_at
FROM listings WHERE id = $1 AND owner_user_id = $2 AND catalog_status = 'active'
FOR UPDATE`, parsedPropertyID, parsedOwnerID).Scan(&qualityStatus, &qualityEvidence, &qualityCheckedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return agency.ErrPropertyNotFound
		}
		if err != nil {
			return err
		}
		result, err := tx.Exec(ctx, `
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
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return agency.ErrPropertyNotFound
		}
		if qualityStatus == "withheld" {
			if _, err := tx.Exec(ctx, `UPDATE listings SET quality_status = 'withheld',
quality_evidence = $2, quality_checked_at = $3 WHERE id = $1`, parsedPropertyID, qualityEvidence, qualityCheckedAt); err != nil {
				return err
			}
		}
		return replaceRules(ctx, tx, parsedPropertyID, input.Requirements)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return agency.Property{}, agency.ValidationError{Field: "url", Reason: "ya pertenece a otra propiedad"}
	}
	if errors.Is(err, agency.ErrPropertyNotFound) {
		return agency.Property{}, err
	}
	if err != nil {
		return agency.Property{}, fmt.Errorf("postgres: update agency property: %w", err)
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

func (s *Store) preparePropertyInput(ctx context.Context, input agency.PropertyInput) (agency.PropertyInput, error) {
	input, err := agency.PreparePropertyInput(input)
	if err != nil {
		return agency.PropertyInput{}, err
	}
	facts, err := s.Facts(ctx)
	if err != nil {
		return agency.PropertyInput{}, err
	}
	input.Requirements, err = agency.PrepareRequirements(input.Requirements, facts)
	return input, err
}

// replaceRules makes the declared requirements the listing's whole set: what
// the agency declares replaces what was read from the listing's text.
func replaceRules(ctx context.Context, tx pgx.Tx, listingID int64, rules []eligibility.Rule) error {
	if _, err := tx.Exec(ctx, `DELETE FROM listing_eligibility_rules WHERE listing_id = $1`, listingID); err != nil {
		return err
	}
	for _, r := range rules {
		values, _ := json.Marshal(r.Values)
		if _, err := tx.Exec(ctx,
			`INSERT INTO listing_eligibility_rules (listing_id, fact, operator, "values", hardness, visibility, source, evidence)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			listingID, r.Fact, r.Operator, values, r.Hardness, r.Visibility, r.Source, r.Evidence); err != nil {
			return err
		}
	}
	return nil
}

// managedListing reads a property an agency manages in Hausy, with its
// eligibility requirements, if a searcher can reach it: active and not
// withheld by a review.
type inquiryQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func managedListing(ctx context.Context, reader inquiryQueryer, propertyID string) (listing.Listing, []eligibility.Rule, error) {
	id, err := strconv.ParseInt(propertyID, 10, 64)
	if err != nil {
		return listing.Listing{}, nil, agency.ErrPropertyNotFound
	}
	var item listing.Listing
	var rules []byte
	err = reader.QueryRow(ctx, `
SELECT `+listingColumns+`,
       (SELECT json_agg(json_build_object(
                  'fact', r.fact, 'operator', r.operator, 'values', r."values",
                  'hardness', r.hardness, 'visibility', r.visibility,
                  'source', r.source, 'evidence', r.evidence)
                ORDER BY r.id)
        FROM listing_eligibility_rules r WHERE r.listing_id = l.id)
FROM listings l
LEFT JOIN agencies a ON a.id = l.agency_id
WHERE l.id = $1 AND l.owner_user_id IS NOT NULL
  AND l.catalog_status = 'active' AND l.quality_status <> 'withheld'`, id).Scan(append(listingTargets(&item), &rules)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return listing.Listing{}, nil, agency.ErrPropertyNotFound
	}
	if err != nil {
		return listing.Listing{}, nil, fmt.Errorf("postgres: read managed listing: %w", err)
	}
	var parsed []eligibility.Rule
	if rules != nil {
		if err := json.Unmarshal(rules, &parsed); err != nil {
			return listing.Listing{}, nil, fmt.Errorf("postgres: rules of listing %d: %w", id, err)
		}
	}
	return item, parsed, nil
}

// PreviewInquiry builds the inquiry a searcher would send, storing nothing.
func (s *Store) PreviewInquiry(ctx context.Context, propertyID string, input agency.InquiryInput) (agency.Inquiry, error) {
	facts, err := s.Facts(ctx)
	if err != nil {
		return agency.Inquiry{}, err
	}
	item, rules, err := managedListing(ctx, s.pool, propertyID)
	if err != nil {
		return agency.Inquiry{}, err
	}
	return agency.BuildInquiry(propertyID, item, rules, input, facts)
}

// SendInquiry stores the inquiry once per inquiry id; a retry returns the
// stored one and reports it was not created.
func (s *Store) SendInquiry(ctx context.Context, propertyID string, input agency.InquiryInput) (agency.Inquiry, error) {
	if err := agency.ValidateInquiryID(input.ID); err != nil {
		return agency.Inquiry{}, err
	}
	facts, err := s.Facts(ctx)
	if err != nil {
		return agency.Inquiry{}, err
	}
	var sent agency.Inquiry
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		stored, err := storedInquiry(ctx, tx, propertyID, input.ID)
		if err == nil {
			sent = stored
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		id, err := strconv.ParseInt(propertyID, 10, 64)
		if err != nil {
			return agency.ErrPropertyNotFound
		}
		// Agency edits lock this same row before replacing its requirements.
		// Hold it until the confirmed brief is stored, closing the preview race.
		var lockedID int64
		if err := tx.QueryRow(ctx, `SELECT id FROM listings WHERE id = $1
AND owner_user_id IS NOT NULL AND catalog_status = 'active' AND quality_status <> 'withheld'
FOR SHARE`, id).Scan(&lockedID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return agency.ErrPropertyNotFound
			}
			return err
		}
		item, rules, err := managedListing(ctx, tx, propertyID)
		if err != nil {
			return err
		}
		inquiry, err := agency.BuildInquiry(propertyID, item, rules, input, facts)
		if err != nil {
			return err
		}
		if err := agency.ValidateInquiryContact(inquiry, input.ID); err != nil {
			return err
		}
		if input.PreviewRevision != inquiry.PreviewRevision {
			return agency.ErrInquiryPreviewChanged
		}
		inquiry.ID = input.ID
		brief, err := json.Marshal(inquiry)
		if err != nil {
			return err
		}
		var createdAt time.Time
		err = tx.QueryRow(ctx, `INSERT INTO listing_inquiries (id, listing_id, brief)
VALUES ($1::uuid, $2, $3) ON CONFLICT (id) DO NOTHING RETURNING created_at`, input.ID, id, brief).Scan(&createdAt)
		if err == nil {
			inquiry.CreatedAt, inquiry.Created = &createdAt, true
			sent = inquiry
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		sent, err = storedInquiry(ctx, tx, propertyID, input.ID)
		return err
	})
	if err != nil {
		return agency.Inquiry{}, fmt.Errorf("postgres: send inquiry: %w", err)
	}
	return sent, nil
}

// storedInquiry returns an immutable receipt even if the listing has changed
// or was archived since the first send.
func storedInquiry(ctx context.Context, reader inquiryQueryer, propertyID, id string) (agency.Inquiry, error) {
	var listingID int64
	var brief []byte
	var createdAt time.Time
	err := reader.QueryRow(ctx, `SELECT listing_id, brief, created_at FROM listing_inquiries WHERE id = $1::uuid`, id).Scan(&listingID, &brief, &createdAt)
	if err != nil {
		return agency.Inquiry{}, err
	}
	if strconv.FormatInt(listingID, 10) != propertyID {
		return agency.Inquiry{}, agency.ErrInquiryConflict
	}
	var stored agency.Inquiry
	if err := json.Unmarshal(brief, &stored); err != nil {
		return agency.Inquiry{}, err
	}
	stored.CreatedAt = &createdAt
	return stored, nil
}

// Inquiries returns the inquiries about one owner's properties, newest first.
func (s *Store) Inquiries(ctx context.Context, ownerID string) ([]agency.Inquiry, error) {
	parsedOwnerID, err := strconv.ParseInt(ownerID, 10, 64)
	if err != nil {
		return nil, agency.ValidationError{Field: "owner", Reason: "no es válido"}
	}
	rows, err := s.pool.Query(ctx, `
SELECT i.brief, i.created_at
FROM listing_inquiries i
JOIN listings l ON l.id = i.listing_id
WHERE l.owner_user_id = $1
ORDER BY i.created_at DESC, i.id`, parsedOwnerID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list inquiries: %w", err)
	}
	defer rows.Close()

	inquiries := make([]agency.Inquiry, 0)
	for rows.Next() {
		var brief []byte
		var inquiry agency.Inquiry
		var createdAt time.Time
		if err := rows.Scan(&brief, &createdAt); err != nil {
			return nil, fmt.Errorf("postgres: scan inquiry: %w", err)
		}
		if err := json.Unmarshal(brief, &inquiry); err != nil {
			return nil, fmt.Errorf("postgres: read inquiry: %w", err)
		}
		inquiry.CreatedAt = &createdAt
		inquiries = append(inquiries, inquiry)
	}
	return inquiries, rows.Err()
}

var _ agency.Catalog = (*Store)(nil)
