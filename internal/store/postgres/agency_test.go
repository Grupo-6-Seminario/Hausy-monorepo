package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/store/postgres"
)

func TestStore_PersistsAnAgencyCatalogEntry(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	provider := auth.NewLocal(store)
	owner, err := provider.SignUp(ctx, auth.Registration{
		Email: uniqueEmail(), Password: "alquileres-caba", Name: "Lounge Propiedades", Role: auth.RoleRealtor,
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	created, err := store.Add(ctx, agency.Owner{ID: owner.ID, Name: owner.Name}, postgresAgencyPropertyInput())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if created.ID == "" || created.ContactCount != 0 {
		t.Fatalf("Add returned %+v", created)
	}

	properties, err := store.List(ctx, owner.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(properties) != 1 || properties[0].ID != created.ID || properties[0].Agency != owner.Name {
		t.Fatalf("List returned %+v", properties)
	}
}

func TestStore_UpdatesAndArchivesOnlyOwnedCatalogEntries(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	provider := auth.NewLocal(store)
	owner, err := provider.SignUp(ctx, auth.Registration{
		Email: uniqueEmail(), Password: "alquileres-caba", Name: "Lounge Propiedades", Role: auth.RoleRealtor,
	})
	if err != nil {
		t.Fatalf("SignUp owner: %v", err)
	}
	other, err := provider.SignUp(ctx, auth.Registration{
		Email: uniqueEmail(), Password: "alquileres-caba", Name: "Otra inmobiliaria", Role: auth.RoleRealtor,
	})
	if err != nil {
		t.Fatalf("SignUp other: %v", err)
	}
	created, err := store.Add(ctx, agency.Owner{ID: owner.ID, Name: owner.Name}, postgresAgencyPropertyInput())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	input := postgresAgencyPropertyInput()
	input.Address = "Humboldt al 2000"
	updated, err := store.Update(ctx, owner.ID, created.ID, input)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Address != input.Address {
		t.Fatalf("Update returned %+v", updated)
	}
	if _, err := store.Update(ctx, other.ID, created.ID, input); !errors.Is(err, agency.ErrPropertyNotFound) {
		t.Fatalf("other owner Update error = %v, want ErrPropertyNotFound", err)
	}

	if err := store.Remove(ctx, owner.ID, created.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	properties, err := store.List(ctx, owner.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(properties) != 0 {
		t.Fatalf("List after Remove returned %+v", properties)
	}
}

func TestStore_PersistsContactIntentsIdempotently(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	provider := auth.NewLocal(store)
	owner, err := provider.SignUp(ctx, auth.Registration{
		Email: uniqueEmail(), Password: "alquileres-caba", Name: "Lounge Propiedades", Role: auth.RoleRealtor,
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	created, err := store.Add(ctx, agency.Owner{ID: owner.ID, Name: owner.Name}, postgresAgencyPropertyInput())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	intent := agency.ContactIntent{ID: "4c065799-5ad0-4df4-9b09-8dcc541507d2", Source: agency.ContactSourceSearchResultCard}

	first, err := store.RecordContactIntent(ctx, created.ID, intent)
	if err != nil {
		t.Fatalf("first RecordContactIntent: %v", err)
	}
	replay, err := store.RecordContactIntent(ctx, created.ID, intent)
	if err != nil {
		t.Fatalf("replayed RecordContactIntent: %v", err)
	}
	if !first.Created || replay.Created || !first.Recorded || !replay.Recorded {
		t.Fatalf("receipts = first %+v, replay %+v", first, replay)
	}
	properties, err := store.List(ctx, owner.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := properties[0].ContactCount; got != 1 {
		t.Fatalf("ContactCount = %d, want 1", got)
	}
}

func TestStore_ArchivedAgencyPropertyLeavesBuyerSearch(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	provider := auth.NewLocal(store)
	owner, err := provider.SignUp(ctx, auth.Registration{
		Email: uniqueEmail(), Password: "alquileres-caba", Name: "Lounge Propiedades", Role: auth.RoleRealtor,
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	created, err := store.Add(ctx, agency.Owner{ID: owner.ID, Name: owner.Name}, postgresAgencyPropertyInput())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	query := search.Query{Neighborhoods: []string{"palermo"}, Operation: "alquiler", Currency: "USD", Limit: 10}
	before, err := store.Search(ctx, query)
	if err != nil {
		t.Fatalf("Search before remove: %v", err)
	}
	if before.TotalMatches != 1 {
		t.Fatalf("Search before remove returned %+v", before)
	}

	if err := store.Remove(ctx, owner.ID, created.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	after, err := store.Search(ctx, query)
	if err != nil {
		t.Fatalf("Search after remove: %v", err)
	}
	if after.TotalMatches != 0 || len(after.Matches) != 0 {
		t.Fatalf("Search after remove returned %+v, want no archived property", after)
	}
}

func postgresAgencyPropertyInput() agency.PropertyInput {
	price := 850.0
	expenses := 120000.0
	rooms := 3
	bedrooms := 2
	return agency.PropertyInput{
		URL:          "https://www.zonaprop.com.ar/propiedades/palermo-agency-test.html",
		Neighborhood: "palermo",
		Address:      "Humboldt al 1900",
		Description:  "Semipiso luminoso al contrafrente.",
		Operation:    "alquiler",
		Price:        agency.Money{Amount: &price, Currency: "USD"},
		Expenses:     agency.Money{Amount: &expenses, Currency: "ARS"},
		Rooms:        &rooms,
		Bedrooms:     &bedrooms,
	}
}

var _ agency.Catalog = (*postgres.Store)(nil)
