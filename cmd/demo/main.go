// Command demo seeds the fictional agency a deployed demo shows: its realtor
// account, nine properties across the catalog's neighborhoods whose declared
// requirements cover the four eligibility states, and four inquiries so its
// panel does not open empty. It converges: a rerun updates the same
// properties and resends the same inquiries, which are stored once.
//
//	HAUSY_DEMO_PASSWORD=... go run ./cmd/demo -database "$DATABASE_URI"
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
)

// The realtor signs in with this email and HAUSY_DEMO_PASSWORD. The .example
// domain is reserved, so neither the address nor the listings' URLs reach
// anyone real.
const (
	realtorEmail = "inmobiliaria@demo.example"
	realtorName  = "Inmobiliaria Demo"
)

type property struct {
	slug, neighborhood, address, description string
	price, expenses                          float64
	rooms, bedrooms, bathrooms               int
	area                                     float64
	requirements                             []eligibility.Rule
}

func accepts(hardness eligibility.Hardness, guarantees ...string) eligibility.Rule {
	return eligibility.Rule{Fact: "guarantee", Operator: eligibility.OneOf, Values: guarantees, Hardness: hardness}
}

func incomeTimes(multiple string, withExpenses bool, hardness eligibility.Hardness) eligibility.Rule {
	operator := eligibility.IncomeMultiple
	if withExpenses {
		operator = eligibility.IncomeMultipleWithExpenses
	}
	return eligibility.Rule{Fact: "income_band", Operator: operator, Values: []string{multiple}, Hardness: hardness}
}

func documentedIncome(hardness eligibility.Hardness) eligibility.Rule {
	return eligibility.Rule{Fact: "income_documented", Operator: eligibility.OneOf, Values: []string{"yes"}, Hardness: hardness}
}

func pets(hardness eligibility.Hardness, admitted ...string) eligibility.Rule {
	return eligibility.Rule{Fact: "pets", Operator: eligibility.SubsetOf, Values: append([]string{"none"}, admitted...), Hardness: hardness}
}

// Rents follow the catalog's medians per neighborhood, so the properties turn
// up in ordinary searches, not only in a scripted one.
var properties = []property{
	{"palermo-gorriti-4800", "palermo", "Gorriti 4800", "Dos ambientes al frente, luminoso, con balcón y cocina integrada. A dos cuadras de Plaza Serrano.",
		850000, 110000, 2, 1, 1, 48, []eligibility.Rule{accepts(eligibility.Hard, "propietaria", "caucion"), incomeTimes("3", false, eligibility.Hard)}},
	{"palermo-thames-1500", "palermo", "Thames 1500", "Tres ambientes con dos dormitorios, lavadero independiente y vista abierta. Edificio con seguridad.",
		1250000, 160000, 3, 2, 2, 72, []eligibility.Rule{accepts(eligibility.Hard, "propietaria"), documentedIncome(eligibility.Hard), pets(eligibility.Hard)}},
	{"palermo-honduras-5200", "palermo", "Honduras 5200", "Monoambiente amplio y silencioso, contrafrente, ideal para una persona. Apto mascotas pequeñas.",
		560000, 70000, 1, 0, 1, 32, []eligibility.Rule{accepts(eligibility.Discretionary, "caucion", "recibos_garante"), pets(eligibility.Discretionary, "cat")}},
	{"congreso-callao-300", "congreso", "Av. Callao 300", "Dos ambientes reciclado a nuevo, piso alto, muy luminoso. Cerca del subte A y la línea B.",
		600000, 90000, 2, 1, 1, 45, []eligibility.Rule{accepts(eligibility.Hard, "propietaria", "caucion", "recibos_garante"), incomeTimes("2.5", true, eligibility.Hard)}},
	{"congreso-rivadavia-1900", "congreso", "Rivadavia 1900", "Monoambiente con kitchenette, en edificio con ascensor. A media cuadra del Congreso.",
		420000, 60000, 1, 0, 1, 28, nil},
	{"congreso-yrigoyen-1800", "congreso", "Hipólito Yrigoyen 1800", "Tres ambientes de estilo, techos altos y pisos de madera. Admite mascotas.",
		780000, 120000, 3, 2, 1, 80, []eligibility.Rule{accepts(eligibility.Hard, "caucion"), pets(eligibility.Hard, "dog", "cat")}},
	{"monserrat-peru-800", "monserrat", "Perú 800", "Dos ambientes en San Telmo, balcón francés y mucha luz natural. Edificio antiguo reciclado.",
		650000, 85000, 2, 1, 1, 50, []eligibility.Rule{accepts(eligibility.Discretionary, "propietaria", "caucion"), incomeTimes("3", false, eligibility.Discretionary)}},
	{"monserrat-chacabuco-600", "monserrat", "Chacabuco 600", "Tres ambientes al frente con dos dormitorios y balcón corrido. Cerca de Plaza de Mayo.",
		820000, 130000, 3, 2, 1, 75, []eligibility.Rule{documentedIncome(eligibility.Hard), accepts(eligibility.Hard, "recibos_garante", "caucion")}},
	{"monserrat-belgrano-1100", "monserrat", "Av. Belgrano 1100", "Monoambiente divisible, con placard y cocina separada. Se aceptan mascotas.",
		450000, 65000, 1, 0, 1, 34, []eligibility.Rule{pets(eligibility.Hard, "dog", "cat", "other"), accepts(eligibility.Hard, "propietaria", "caucion", "recibos_garante")}},
}

func (p property) input() agency.PropertyInput {
	price, expenses, area := p.price, p.expenses, p.area
	rooms, bedrooms, bathrooms := p.rooms, p.bedrooms, p.bathrooms
	return agency.PropertyInput{
		URL: "https://example.com/inmobiliaria-demo/" + p.slug, Neighborhood: p.neighborhood,
		Address: p.address, Description: p.description, Operation: "alquiler",
		Price: agency.Money{Amount: &price, Currency: "ARS"}, Expenses: agency.Money{Amount: &expenses, Currency: "ARS"},
		TotalAreaM2: &area, Rooms: &rooms, Bedrooms: &bedrooms, Bathrooms: &bathrooms,
		Requirements: p.requirements,
	}
}

// One inquiry per eligibility a searcher can send with: eligible, depends on
// the agency (twice, for different reasons) and unknown.
var inquiries = []struct {
	slug  string
	input agency.InquiryInput
}{
	{"monserrat-peru-800", agency.InquiryInput{ID: "0b6f3a52-7d1e-4c8a-9f2b-3e4d5c6b7a81", Name: "Valentina", Email: "valentina@correo.example",
		Message: "Hola, ¿el departamento sigue disponible? Trabajo en relación de dependencia.", Searching: []string{"2 ambientes", "en Monserrat"},
		Qualification: eligibility.Qualification{"guarantee": {"propietaria"}, "income_band": {"1000000-2000000"}}}},
	{"congreso-rivadavia-1900", agency.InquiryInput{ID: "1c7a4b63-8e2f-4d9b-a03c-4f5e6d7c8b92", Name: "Diego", Phone: "+54 9 11 5555-0102",
		Message: "¿Se puede visitar el jueves a la tarde?", Searching: []string{"monoambiente", "en Congreso"}}},
	{"palermo-honduras-5200", agency.InquiryInput{ID: "2d8b5c74-9f3a-4e0c-b14d-5a6f7e8d9ca3", Name: "Sofía", Email: "sofia@correo.example",
		Message: "Tengo una gata tranquila. ¿Lo consultan con el propietario?", Searching: []string{"monoambiente", "en Palermo", "apto mascotas"},
		Qualification: eligibility.Qualification{"guarantee": {"recibos_garante"}, "pets": {"cat"}}}},
	{"palermo-gorriti-4800", agency.InquiryInput{ID: "3e9c6d85-a04b-4f1d-825e-6b7a8f9e0db4", Name: "Lucía", Phone: "+54 9 11 5555-0101",
		Searching:     []string{"2 ambientes", "en Palermo", "con balcón"},
		Qualification: eligibility.Qualification{"guarantee": {"caucion"}, "income_band": {"3000000-"}, "income_documented": {"yes"}}}},
}

func main() {
	uri := flag.String("database", os.Getenv("DATABASE_URI"), "Postgres URI of the database to seed")
	flag.Parse()
	password := os.Getenv("HAUSY_DEMO_PASSWORD")
	if *uri == "" || len(password) < 8 {
		log.Fatal("demo: set -database (or DATABASE_URI) and HAUSY_DEMO_PASSWORD (8+ characters)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := seed(ctx, *uri, password); err != nil {
		log.Fatal(err)
	}
}

func seed(ctx context.Context, uri, password string) error {
	store, err := postgres.Open(ctx, uri)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return err
	}

	provider := auth.NewLocal(store)
	user, err := provider.SignUp(ctx, auth.Registration{Email: realtorEmail, Password: password, Name: realtorName, Role: auth.RoleRealtor})
	if errors.Is(err, auth.ErrEmailTaken) {
		session, signInErr := provider.SignIn(ctx, realtorEmail, password)
		if signInErr != nil {
			return fmt.Errorf("demo: %s exists with another password: %w", realtorEmail, signInErr)
		}
		user, err = session.User, nil
	}
	if err != nil {
		return fmt.Errorf("demo: realtor: %w", err)
	}

	existing, err := store.List(ctx, user.ID)
	if err != nil {
		return err
	}
	ids := map[string]string{}
	for _, p := range existing {
		ids[p.URL] = p.ID
	}
	bySlug := map[string]string{}
	for _, p := range properties {
		input := p.input()
		if id, ok := ids[input.URL]; ok {
			if _, err := store.Update(ctx, user.ID, id, input); err != nil {
				return fmt.Errorf("demo: update %s: %w", p.slug, err)
			}
			bySlug[p.slug] = id
			continue
		}
		created, err := store.Add(ctx, agency.Owner{ID: user.ID, Name: user.Name}, input)
		if err != nil {
			return fmt.Errorf("demo: add %s: %w", p.slug, err)
		}
		bySlug[p.slug] = created.ID
	}
	for _, inquiry := range inquiries {
		preview, err := store.PreviewInquiry(ctx, bySlug[inquiry.slug], inquiry.input)
		if err != nil {
			return fmt.Errorf("demo: preview %s: %w", inquiry.slug, err)
		}
		inquiry.input.PreviewRevision = preview.PreviewRevision
		if _, err := store.SendInquiry(ctx, bySlug[inquiry.slug], inquiry.input); err != nil {
			return fmt.Errorf("demo: inquiry about %s: %w", inquiry.slug, err)
		}
	}
	log.Printf("demo: %s owns %d properties with %d example inquiries", realtorEmail, len(properties), len(inquiries))
	return nil
}
