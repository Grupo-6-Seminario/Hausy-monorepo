package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
)

// ReviewQueue reads stored listings that still need the one-time or changed-row audit.
func (s *Store) ReviewQueue(ctx context.Context) ([]listing.Listing, error) {
	rows, err := s.pool.Query(ctx, `SELECT url FROM listings WHERE catalog_status = 'active' AND quality_status IN ('legacy', 'pending') ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var urls []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			rows.Close()
			return nil, err
		}
		urls = append(urls, url)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]listing.Listing, 0, len(urls))
	for _, url := range urls {
		item, err := s.ByURL(ctx, url)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// ExportReviews writes only completed decisions; absent URLs stay pending on
// a fresh load. The snapshot lets load rebuild the same visible inventory.
func (s *Store) ExportReviews(ctx context.Context, output io.Writer, committedURLs map[string]bool) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT url, quality_status, COALESCE(quality_evidence, '[]'::jsonb) FROM listings WHERE quality_status IN ('passed', 'withheld') ORDER BY url`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	count := 0
	for rows.Next() {
		var record quality.Record
		var evidence []byte
		if err := rows.Scan(&record.URL, &record.Review.Status, &evidence); err != nil {
			return count, err
		}
		if !committedURLs[record.URL] {
			continue
		}
		item, err := s.ByURL(ctx, record.URL)
		if err != nil {
			return count, err
		}
		record.ContentSHA256 = quality.Fingerprint(item)
		if err := json.Unmarshal(evidence, &record.Review.Conflicts); err != nil {
			return count, err
		}
		if err := encoder.Encode(record); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

// SaveQualityRecord refuses a review for older listing content. The row stays
// pending even if the snapshot itself was previously marked passed.
func (s *Store) SaveQualityRecord(ctx context.Context, record quality.Record) error {
	item, err := s.ByURL(ctx, record.URL)
	if err != nil {
		return err
	}
	if quality.Fingerprint(item) != record.ContentSHA256 {
		_, err := s.pool.Exec(ctx, `UPDATE listings SET quality_status='pending', quality_evidence=NULL, quality_checked_at=NULL WHERE url=$1`, record.URL)
		if err != nil {
			return err
		}
		return fmt.Errorf("postgres: stale quality review for %s", record.URL)
	}
	return s.SaveReview(ctx, record.URL, record.Review)
}

// SaveReview publishes a clean listing or withholds a quote-backed conflict.
// ponytail: the manual audit assumes no simultaneous catalog edit; add a row
// version precondition if live edits and audits overlap in production.
func (s *Store) SaveReview(ctx context.Context, url string, review quality.Review) error {
	if review.Status != quality.Passed && review.Status != quality.Withheld {
		return fmt.Errorf("postgres: invalid quality status")
	}
	if review.Status == quality.Withheld && len(review.Conflicts) == 0 {
		return fmt.Errorf("postgres: withheld review needs evidence")
	}
	evidence, err := json.Marshal(review.Conflicts)
	if err != nil {
		return err
	}
	result, err := s.pool.Exec(ctx, `UPDATE listings SET quality_status = $2, quality_evidence = $3, quality_checked_at = now() WHERE url = $1 AND catalog_status = 'active'`, url, review.Status, evidence)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("postgres: listing not found for quality review")
	}
	return nil
}

// QuarantineLegacy closes the migration window after every old row has had an
// audit attempt. Unresolved listings remain queued but disappear from search.
func (s *Store) QuarantineLegacy(ctx context.Context) (int64, error) {
	result, err := s.pool.Exec(ctx, `UPDATE listings SET quality_status = 'pending' WHERE quality_status = 'legacy'`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
