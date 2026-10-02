package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

// SaveEligibility replaces a listing's eligibility rules, so loading the same
// file twice leaves the same database. A rule the evaluator cannot read is
// refused before anything changes, so the previous rules stay.
func (s *Store) SaveEligibility(ctx context.Context, url string, rules []eligibility.Rule) error {
	for _, r := range rules {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("postgres: eligibility for %s: %w", url, err)
		}
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		var id int64
		if err := tx.QueryRow(ctx, `SELECT id FROM listings WHERE url = $1`, url).Scan(&id); err != nil {
			return fmt.Errorf("postgres: eligibility for unknown listing %s: %w", url, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM listing_eligibility_rules WHERE listing_id = $1`, id); err != nil {
			return err
		}
		for _, r := range rules {
			values, _ := json.Marshal(r.Values)
			if _, err := tx.Exec(ctx,
				`INSERT INTO listing_eligibility_rules (listing_id, fact, operator, "values", hardness, visibility, source, evidence)
				 VALUES ($1, $2, $3, $4, $5, COALESCE(NULLIF($6, ''), 'public'), COALESCE(NULLIF($7, ''), 'parsed'), $8)`,
				id, r.Fact, r.Operator, values, r.Hardness, r.Visibility, r.Source, r.Evidence); err != nil {
				return fmt.Errorf("postgres: save eligibility rule for %s: %w", url, err)
			}
		}
		return nil
	})
}

// Candidates returns every active listing satisfying the query's hard
// constraints, whole, with its eligibility rules. Unlike Search it has no
// limit: ordering by eligibility needs the whole population.
// ponytail: one ByURL per match; batch it if a branch outgrows a few hundred rows.
func (s *Store) Candidates(ctx context.Context, query search.Query) ([]eligibility.Candidate, error) {
	return s.candidates(ctx, query, 0)
}

// PreviewCandidates reads a bounded, stable sample for deciding whether a
// clarification can change the visible result. Full ranking still uses Candidates.
func (s *Store) PreviewCandidates(ctx context.Context, query search.Query, limit int) ([]eligibility.Candidate, error) {
	if limit < 1 || limit > 50 {
		limit = 50
	}
	return s.candidates(ctx, query, limit)
}

// HasAttributeData checks coverage across the whole matching inventory. A
// capped preview alone could miss attributes that happen to be on later rows.
func (s *Store) HasAttributeData(ctx context.Context, query search.Query, attributeType string) (bool, error) {
	b := &builder{}
	applyBaseConditions(b, query)
	applyPriceCondition(b, query)
	b.where("a.type = " + b.param(attributeType))
	var found bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM listings l JOIN listing_attributes a ON a.listing_id = l.id WHERE `+b.whereClause()+`)`, b.args...).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("postgres: attribute coverage: %w", err)
	}
	return found, nil
}

func (s *Store) candidates(ctx context.Context, query search.Query, limit int) ([]eligibility.Candidate, error) {
	b := &builder{}
	applyBaseConditions(b, query)
	applyPriceCondition(b, query)
	statement := `SELECT l.id, l.url FROM listings l WHERE ` + b.whereClause() + ` ORDER BY l.id`
	if limit > 0 {
		statement += ` LIMIT ` + b.param(limit)
	}
	rows, err := s.pool.Query(ctx, statement, b.args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: candidates: %w", err)
	}
	var ids []int64
	var urls []string
	for rows.Next() {
		var id int64
		var url string
		if err := rows.Scan(&id, &url); err != nil {
			rows.Close()
			return nil, err
		}
		ids, urls = append(ids, id), append(urls, url)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rules, err := s.rulesFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]eligibility.Candidate, 0, len(urls))
	for i, url := range urls {
		item, err := s.ByURL(ctx, url)
		if err != nil {
			return nil, err
		}
		out = append(out, eligibility.Candidate{Listing: item, Rules: rules[ids[i]]})
	}
	return out, nil
}

func (s *Store) rulesFor(ctx context.Context, ids []int64) (map[int64][]eligibility.Rule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT listing_id, fact, operator, "values", hardness, visibility, source, evidence
		 FROM listing_eligibility_rules WHERE listing_id = ANY($1) ORDER BY id`, ids)
	if err != nil {
		return nil, fmt.Errorf("postgres: eligibility rules: %w", err)
	}
	defer rows.Close()
	out := map[int64][]eligibility.Rule{}
	for rows.Next() {
		var id int64
		var r eligibility.Rule
		var values []byte
		if err := rows.Scan(&id, &r.Fact, &r.Operator, &values, &r.Hardness, &r.Visibility, &r.Source, &r.Evidence); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(values, &r.Values); err != nil {
			return nil, err
		}
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("postgres: listing %d: %w", id, err)
		}
		out[id] = append(out[id], r)
	}
	return out, rows.Err()
}

// Facts reads the catalog of facts rules and qualifications may reference.
func (s *Store) Facts(ctx context.Context) (eligibility.Catalog, error) {
	rows, err := s.pool.Query(ctx, `SELECT name, admissible, choices FROM eligibility_facts`)
	if err != nil {
		return nil, fmt.Errorf("postgres: eligibility facts: %w", err)
	}
	defer rows.Close()
	out := eligibility.Catalog{}
	for rows.Next() {
		var name string
		var fact eligibility.Fact
		var choices []byte
		if err := rows.Scan(&name, &fact.Admissible, &choices); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(choices, &fact.Choices); err != nil {
			return nil, fmt.Errorf("postgres: choices of fact %q: %w", name, err)
		}
		out[name] = fact
	}
	return out, rows.Err()
}

// SaveQualification replaces what a user declared. Inadmissible facts are
// refused, not silently dropped.
func (s *Store) SaveQualification(ctx context.Context, userID string, q eligibility.Qualification) error {
	catalog, err := s.Facts(ctx)
	if err != nil {
		return err
	}
	for fact := range q {
		if !catalog.Admissible(fact) {
			return fmt.Errorf("postgres: %q is not an admissible qualification fact", fact)
		}
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM user_qualifications WHERE user_id = $1::bigint`, userID); err != nil {
			return err
		}
		for fact, values := range q {
			for _, v := range values {
				if _, err := tx.Exec(ctx, `INSERT INTO user_qualifications (user_id, fact, value) VALUES ($1::bigint, $2, $3) ON CONFLICT DO NOTHING`, userID, fact, v); err != nil {
					return fmt.Errorf("postgres: save qualification: %w", err)
				}
			}
		}
		return nil
	})
}

func (s *Store) Qualification(ctx context.Context, userID string) (eligibility.Qualification, error) {
	rows, err := s.pool.Query(ctx, `SELECT fact, value FROM user_qualifications WHERE user_id = $1::bigint ORDER BY fact, value`, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: qualification: %w", err)
	}
	defer rows.Close()
	out := eligibility.Qualification{}
	for rows.Next() {
		var fact, value string
		if err := rows.Scan(&fact, &value); err != nil {
			return nil, err
		}
		out[fact] = append(out[fact], value)
	}
	return out, rows.Err()
}
