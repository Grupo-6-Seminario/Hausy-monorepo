// Package comparison runs an offline matching experiment, never the application.
package comparison

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

var catalog = eligibility.Catalog{"guarantee": {Admissible: true}, "income_band": {Admissible: true}, "caucion_quoted": {Admissible: true}, "pets": {Admissible: true, Multiple: true, Choices: []eligibility.Choice{{Value: "none", Label: "No tengo"}, {Value: "dog", Label: "Perro"}, {Value: "cat", Label: "Gato"}}}}

// Load binds selected real rows to existing quality and eligibility snapshots.
// The in-memory adapter covers these fixtures, not SQL or complete CABA stock.
func Load(root, fixture string) ([]Case, map[string]string, error) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		return nil, nil, err
	}
	var cases []Case
	if err = json.Unmarshal(data, &cases); err != nil {
		return nil, nil, err
	}
	hashes := map[string]string{"fixtures/cases.json": fmt.Sprintf("%x", sha256.Sum256(data))}
	stock := map[string]eligibility.Candidate{}
	reviews := map[string]quality.Record{}
	read := func(name string, fn func([]byte) error) error {
		data, err := os.ReadFile(filepath.Join(root, "data", name))
		if err != nil {
			return err
		}
		hashes["data/"+name] = fmt.Sprintf("%x", sha256.Sum256(data))
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(nil, 1<<20)
		for scanner.Scan() {
			if err := fn(scanner.Bytes()); err != nil {
				return err
			}
		}
		return scanner.Err()
	}
	if err := read("listings.parsed.jsonl", func(line []byte) error {
		var item listing.Listing
		if err := json.Unmarshal(line, &item); err != nil {
			return err
		}
		item.Attributes = listing.StatedAmenities(item.Attributes)
		slices.SortFunc(item.Attributes, func(a, b listing.Attribute) int {
			if n := cmp.Compare(a.Type, b.Type); n != 0 {
				return n
			}
			return cmp.Compare(a.Value, b.Value)
		})
		stock[item.URL] = eligibility.Candidate{Listing: item}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	if err := read("listings.eligibility.jsonl", func(line []byte) error {
		var record struct {
			URL   string             `json:"url"`
			Rules []eligibility.Rule `json:"rules"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			return err
		}
		item := stock[record.URL]
		item.Rules = append(record.Rules, eligibility.PublishedRules(item.Listing.Attributes, catalog)...)
		stock[record.URL] = item
		return nil
	}); err != nil {
		return nil, nil, err
	}
	if err := read("listings.quality.jsonl", func(line []byte) error {
		var record quality.Record
		if err := json.Unmarshal(line, &record); err != nil {
			return err
		}
		reviews[record.URL] = record
		return nil
	}); err != nil {
		return nil, nil, err
	}
	for i := range cases {
		for _, url := range cases[i].SnapshotURLs {
			c, ok := stock[url]
			if !ok {
				return nil, nil, fmt.Errorf("missing snapshot row %s", url)
			}
			review, ok := reviews[url]
			if !ok || review.Review.Status != "passed" || quality.Fingerprint(c.Listing) != review.ContentSHA256 {
				return nil, nil, fmt.Errorf("snapshot row not currently published under content fingerprint: %s", url)
			}
			cases[i].Candidates = append(cases[i].Candidates, c)
		}
	}
	return cases, hashes, nil
}

type inventory struct{ rows []eligibility.Candidate }

func (s inventory) Facts(context.Context) (eligibility.Catalog, error) { return catalog, nil }
func (s inventory) Candidates(ctx context.Context, q search.Query) ([]eligibility.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	q, err := q.Validate()
	if err != nil {
		return nil, err
	}
	out := []eligibility.Candidate{}
	for _, c := range s.rows {
		if hardMatches(c.Listing, q) {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b eligibility.Candidate) int { return cmp.Compare(a.Listing.URL, b.Listing.URL) })
	return out, nil
}
func hardMatches(l listing.Listing, q search.Query) bool {
	if len(q.Neighborhoods) > 0 && !slices.Contains(q.Neighborhoods, l.Neighborhood) {
		return false
	}
	if q.Operation != "" && q.Operation != l.Operation {
		return false
	}
	if q.Currency != "" {
		if l.Price.Amount == nil {
			if !q.IncludeUnpriced {
				return false
			}
		} else if l.Price.Currency != q.Currency || !bounds(l.Price.Amount, q.MinPrice, q.MaxPrice) {
			return false
		}
	}
	if q.MaxExpensesARS != nil && (l.Expenses.Currency != "ARS" || !bounds(l.Expenses.Amount, nil, q.MaxExpensesARS)) {
		return false
	}
	if !bounds(l.Rooms, q.MinRooms, q.MaxRooms) || !bounds(l.Bedrooms, q.MinBedrooms, nil) || !bounds(l.Bathrooms, q.MinBathrooms, nil) || !bounds(l.ParkingSpaces, q.MinParkingSpaces, nil) || !bounds(l.TotalAreaM2, q.MinTotalAreaM2, nil) || !bounds(l.AgeYears, nil, q.MaxAgeYears) {
		return false
	}
	for _, f := range q.RequiredAttributes {
		if !slices.ContainsFunc(l.Attributes, func(a listing.Attribute) bool { return a.Type == f.Type && a.Value == f.Value }) {
			return false
		}
	}
	for _, f := range q.ExcludedAttributes {
		if slices.ContainsFunc(l.Attributes, func(a listing.Attribute) bool { return a.Type == f.Type && a.Value == f.Value }) {
			return false
		}
	}
	return true
}
func bounds[T ~int | ~float64](value, min, max *T) bool {
	return (min == nil || value != nil && *value >= *min) && (max == nil || value != nil && *value <= *max)
}
