// Command seed recreates the journeys database from a small committed
// fixture: it drops and creates the database, migrates it, loads the fixture
// listings through the pipeline, publishes them, and creates the realtor who
// owns two of them. It refuses any database whose name does not end in
// "_test", so it can never reach the dev database.
//
//	go run ./e2e/journeys/seed -database postgresql://hausy:hausy@localhost:5432/hausy_e2e_test
package main

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/pipeline"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
	"github.com/jackc/pgx/v5"
)

//go:embed fixtures
var fixtures embed.FS

// The journeys sign in with these and contact listing A. Keep them in sync
// with frontend/journeys/fixture.ts.
const (
	realtorEmail    = "inmobiliaria@journeys.test"
	realtorPassword = "journeys-realtor"
	realtorName     = "Inmobiliaria Journeys"
	listingA        = "http://127.0.0.1:8710/publicacion/60023997"
)

func main() {
	uri := flag.String("database", "postgresql://hausy:hausy@localhost:5432/hausy_e2e_test", "Postgres URI of the database to recreate; its name must end in _test")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := seed(ctx, *uri); err != nil {
		log.Fatal(err)
	}
}

// adminURI returns the URI of the server's "postgres" database and the name
// of the database to recreate, refusing a name that does not end in "_test".
func adminURI(uri string) (string, string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", "", err
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	if !strings.HasSuffix(name, "_test") {
		return "", "", fmt.Errorf("seed: refusing to recreate %q: its name does not end in _test", name)
	}
	parsed.Path = "/postgres"
	return parsed.String(), name, nil
}

func seed(ctx context.Context, uri string) error {
	admin, name, err := adminURI(uri)
	if err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		return err
	}
	quoted := pgx.Identifier{name}.Sanitize()
	_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+quoted+" WITH (FORCE)")
	if err == nil {
		_, err = conn.Exec(ctx, "CREATE DATABASE "+quoted)
	}
	conn.Close(ctx)
	if err != nil {
		return err
	}

	store, err := postgres.Open(ctx, uri)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return err
	}

	parsed, err := fixtures.ReadFile("fixtures/listings.parsed.jsonl")
	if err != nil {
		return err
	}
	if err := load(pipeline.Load(ctx, bytes.NewReader(parsed), store)); err != nil {
		return err
	}
	rules, err := fixtures.ReadFile("fixtures/listings.eligibility.jsonl")
	if err != nil {
		return err
	}
	if err := load(pipeline.LoadEligibility(ctx, bytes.NewReader(rules), store)); err != nil {
		return err
	}
	// The fixture is curated, so every listing is published as passed.
	lines := bufio.NewScanner(bytes.NewReader(parsed))
	lines.Buffer(make([]byte, 1<<20), 1<<20)
	for lines.Scan() {
		var item struct{ URL string }
		if err := json.Unmarshal(lines.Bytes(), &item); err != nil {
			return err
		}
		if err := store.SaveReview(ctx, item.URL, quality.Review{Status: quality.Passed}); err != nil {
			return err
		}
	}

	realtor, err := auth.NewLocal(store).SignUp(ctx, auth.Registration{Email: realtorEmail, Password: realtorPassword, Name: realtorName, Role: auth.RoleRealtor})
	if err != nil {
		return err
	}
	// Listing B is the realtor's own entry, made the way the dashboard makes it.
	rooms := 2
	price := 700000.0
	if _, err := store.Add(ctx, agency.Owner{ID: realtor.ID, Name: realtor.Name}, agency.PropertyInput{
		URL:          "http://127.0.0.1:8710/publicacion/inmobiliaria-congreso",
		Neighborhood: "congreso",
		Address:      "Rivadavia al 1800",
		Description:  "Departamento de dos ambientes en Congreso, cargado por la inmobiliaria.",
		Operation:    "alquiler",
		Price:        agency.Money{Amount: &price, Currency: "ARS"},
		Rooms:        &rooms,
	}); err != nil {
		return err
	}
	// Listing A came through the pipeline, which is how it carries the pileta
	// the search asks for; the agency API cannot store prose-derived
	// attributes. No API hands a scraped listing to a realtor, so this one
	// update is SQL.
	conn, err = pgx.Connect(ctx, uri)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	tag, err := conn.Exec(ctx, "UPDATE listings SET owner_user_id = $1 WHERE url = $2", realtor.ID, listingA)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("seed: listing A %s is not in the fixture", listingA)
	}
	log.Printf("seed: recreated %s with the journeys fixture and realtor %s", name, realtorEmail)
	return nil
}

// load fails the seed on any listing the pipeline skipped or rejected: a
// journey would otherwise fail later, far from the cause.
func load(report pipeline.Report, err error) error {
	if err != nil {
		return err
	}
	if report.Failed > 0 || report.Skipped > 0 {
		return fmt.Errorf("seed: loaded %d, skipped %d, failed %d: %v", report.Loaded, report.Skipped, report.Failed, report.Errors)
	}
	return nil
}
