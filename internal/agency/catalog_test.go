package agency_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
)

func TestMemoryCatalogAddsAndListsPropertiesForOneOwner(t *testing.T) {
	catalog := agency.NewMemoryCatalog()
	ctx := context.Background()
	owner := agency.Owner{ID: "realtor-7", Name: "Lounge Propiedades"}

	created, err := catalog.Add(ctx, owner, agency.PropertyInput{
		URL:          "https://www.zonaprop.com.ar/propiedades/palermo-101.html",
		Neighborhood: "palermo",
		Address:      "Humboldt al 1900",
		Description:  "Semipiso luminoso al contrafrente.",
		Operation:    "alquiler",
		Price:        agency.Money{Amount: float64Pointer(850), Currency: "USD"},
		Expenses:     agency.Money{Amount: float64Pointer(120000), Currency: "ARS"},
		Rooms:        intPointer(3),
		Bedrooms:     intPointer(2),
	})
	if err != nil {
		t.Fatalf("Add returned an error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("Add returned a property without an id")
	}
	if created.ContactCount != 0 {
		t.Fatalf("new property contact count = %d, want 0", created.ContactCount)
	}

	properties, err := catalog.List(ctx, owner.ID)
	if err != nil {
		t.Fatalf("List returned an error: %v", err)
	}
	if len(properties) != 1 || properties[0].Address != "Humboldt al 1900" {
		t.Fatalf("List = %+v, want the property just added", properties)
	}

	otherProperties, err := catalog.List(ctx, "realtor-8")
	if err != nil {
		t.Fatalf("List for another owner returned an error: %v", err)
	}
	if len(otherProperties) != 0 {
		t.Fatalf("another owner can see %+v, want no properties", otherProperties)
	}
}

func TestMemoryCatalogUpdatesOnlyTheOwnersProperty(t *testing.T) {
	catalog := agency.NewMemoryCatalog()
	ctx := context.Background()
	owner := agency.Owner{ID: "realtor-7", Name: "Lounge Propiedades"}
	created, err := catalog.Add(ctx, owner, validPropertyInput())
	if err != nil {
		t.Fatalf("Add returned an error: %v", err)
	}

	updatedInput := validPropertyInput()
	updatedInput.Address = "Humboldt al 2000"
	updatedInput.Price = agency.Money{Amount: float64Pointer(900), Currency: "USD"}
	updated, err := catalog.Update(ctx, owner.ID, created.ID, updatedInput)
	if err != nil {
		t.Fatalf("Update returned an error: %v", err)
	}
	if updated.Address != "Humboldt al 2000" || *updated.Price.Amount != 900 {
		t.Fatalf("Update = %+v, want the submitted address and price", updated)
	}
	if updated.Agency != owner.Name || updated.ContactCount != 0 {
		t.Fatalf("Update changed server-owned fields: %+v", updated)
	}

	if _, err := catalog.Update(ctx, "realtor-8", created.ID, updatedInput); !errors.Is(err, agency.ErrPropertyNotFound) {
		t.Fatalf("another owner Update error = %v, want ErrPropertyNotFound", err)
	}
}

func TestMemoryCatalogRemovesAPropertyWithoutExposingItToOtherOwners(t *testing.T) {
	catalog := agency.NewMemoryCatalog()
	ctx := context.Background()
	owner := agency.Owner{ID: "realtor-7", Name: "Lounge Propiedades"}
	created, err := catalog.Add(ctx, owner, validPropertyInput())
	if err != nil {
		t.Fatalf("Add returned an error: %v", err)
	}

	if err := catalog.Remove(ctx, "realtor-8", created.ID); !errors.Is(err, agency.ErrPropertyNotFound) {
		t.Fatalf("another owner Remove error = %v, want ErrPropertyNotFound", err)
	}
	if err := catalog.Remove(ctx, owner.ID, created.ID); err != nil {
		t.Fatalf("Remove returned an error: %v", err)
	}

	properties, err := catalog.List(ctx, owner.ID)
	if err != nil {
		t.Fatalf("List returned an error: %v", err)
	}
	if len(properties) != 0 {
		t.Fatalf("List after Remove = %+v, want no active properties", properties)
	}
}

func TestMemoryCatalogRecordsAContactIntentExactlyOnce(t *testing.T) {
	catalog := agency.NewMemoryCatalog()
	ctx := context.Background()
	owner := agency.Owner{ID: "realtor-7", Name: "Lounge Propiedades"}
	created, err := catalog.Add(ctx, owner, validPropertyInput())
	if err != nil {
		t.Fatalf("Add returned an error: %v", err)
	}
	intent := agency.ContactIntent{ID: "4c065799-5ad0-4df4-9b09-8dcc541507d2", Source: agency.ContactSourceSearchResultCard}

	first, err := catalog.RecordContactIntent(ctx, created.ID, intent)
	if err != nil {
		t.Fatalf("first RecordContactIntent returned an error: %v", err)
	}
	if !first.Created || !first.Recorded || first.ListingID != created.ID {
		t.Fatalf("first receipt = %+v, want one newly recorded intent", first)
	}

	replay, err := catalog.RecordContactIntent(ctx, created.ID, intent)
	if err != nil {
		t.Fatalf("replayed RecordContactIntent returned an error: %v", err)
	}
	if replay.Created || !replay.Recorded {
		t.Fatalf("replay receipt = %+v, want recorded but not created", replay)
	}

	properties, err := catalog.List(ctx, owner.ID)
	if err != nil {
		t.Fatalf("List returned an error: %v", err)
	}
	if got := properties[0].ContactCount; got != 1 {
		t.Fatalf("ContactCount = %d, want 1 after an idempotent replay", got)
	}
}

func TestMemoryCatalogRejectsAnInvalidPropertyAtTheApplicationSeam(t *testing.T) {
	catalog := agency.NewMemoryCatalog()
	input := validPropertyInput()
	input.URL = ""

	if _, err := catalog.Add(context.Background(), agency.Owner{ID: "realtor-7"}, input); !errors.Is(err, agency.ErrInvalidProperty) {
		t.Fatalf("Add error = %v, want ErrInvalidProperty", err)
	}
}

func TestPreparePropertyInputRejectsInvalidDeterministicFacts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*agency.PropertyInput)
	}{
		{name: "negative price", mutate: func(input *agency.PropertyInput) { input.Price.Amount = float64Pointer(-1) }},
		{name: "unsupported currency", mutate: func(input *agency.PropertyInput) { input.Price.Currency = "EUR" }},
		{name: "amount without currency", mutate: func(input *agency.PropertyInput) { input.Price.Currency = "" }},
		{name: "negative rooms", mutate: func(input *agency.PropertyInput) { input.Rooms = intPointer(-1) }},
		{name: "negative area", mutate: func(input *agency.PropertyInput) { input.TotalAreaM2 = float64Pointer(-1) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validPropertyInput()
			test.mutate(&input)
			if _, err := agency.PreparePropertyInput(input); !errors.Is(err, agency.ErrInvalidProperty) {
				t.Fatalf("PreparePropertyInput error = %v, want ErrInvalidProperty", err)
			}
		})
	}
}

func validPropertyInput() agency.PropertyInput {
	return agency.PropertyInput{
		URL:          "https://www.zonaprop.com.ar/propiedades/palermo-101.html",
		Neighborhood: "palermo",
		Address:      "Humboldt al 1900",
		Description:  "Semipiso luminoso al contrafrente.",
		Operation:    "alquiler",
		Price:        agency.Money{Amount: float64Pointer(850), Currency: "USD"},
		Expenses:     agency.Money{Amount: float64Pointer(120000), Currency: "ARS"},
		Rooms:        intPointer(3),
		Bedrooms:     intPointer(2),
	}
}

func float64Pointer(value float64) *float64 { return &value }
func intPointer(value int) *int             { return &value }
