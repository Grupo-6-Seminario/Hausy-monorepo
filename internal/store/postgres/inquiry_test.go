package postgres_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/agency"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/quality"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"
)

func TestStore_AnAgencyListingIsSearchableWithItsDeclaredRequirementsAndTakesInquiries(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	owner, err := auth.NewLocal(store).SignUp(ctx, auth.Registration{
		Email: uniqueEmail(), Password: "alquileres-caba", Name: "Inmobiliaria Demo", Role: auth.RoleRealtor,
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	input := postgresAgencyPropertyInput()
	price := 600000.0
	input.Price = agency.Money{Amount: &price, Currency: "ARS"}
	input.Requirements = []eligibility.Rule{
		{Fact: "guarantee", Operator: eligibility.OneOf, Values: []string{"propietaria", "caucion"}, Hardness: eligibility.Hard},
		{Fact: "pets", Operator: eligibility.SubsetOf, Values: []string{"cat"}, Hardness: eligibility.Discretionary},
	}
	created, err := store.Add(ctx, agency.Owner{ID: owner.ID, Name: owner.Name}, input)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got := created.Requirements; len(got) != 2 || got[1].Evidence != "Admite gato" || !slices.Equal(got[1].Values, []string{"none", "cat"}) {
		t.Fatalf("requirements = %+v", got)
	}

	// Published when saved, although no review has passed it.
	candidates, err := store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo"}})
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	found := slices.IndexFunc(candidates, func(c eligibility.Candidate) bool {
		return strconv.FormatInt(c.Listing.ID, 10) == created.ID
	})
	if found < 0 || !candidates[found].Listing.Managed || len(candidates[found].Rules) != 2 || candidates[found].Rules[0].Source != "declared" {
		t.Fatalf("search does not show the agency's listing with its requirements: %+v", candidates)
	}

	inquiry := agency.InquiryInput{
		ID: "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed", Name: "Ana", Phone: "+54 9 11 5555-5555",
		Qualification: eligibility.Qualification{"guarantee": {"caucion"}, "pets": {"cat"}, "income_band": {"2000000-3000000"}},
	}
	preview, err := store.PreviewInquiry(ctx, created.ID, inquiry)
	if err != nil {
		t.Fatal(err)
	}
	inquiry.PreviewRevision = preview.PreviewRevision
	sent, err := store.SendInquiry(ctx, created.ID, inquiry)
	if err != nil || !sent.Created || sent.Eligibility != eligibility.ConditionallyEligible {
		t.Fatalf("SendInquiry = %+v, %v", sent, err)
	}
	again, err := store.SendInquiry(ctx, created.ID, inquiry)
	if err != nil || again.Created || again.ID != inquiry.ID {
		t.Fatalf("retried SendInquiry = %+v, %v", again, err)
	}

	inbox, err := store.Inquiries(ctx, owner.ID)
	if err != nil || len(inbox) != 1 || inbox[0].Name != "Ana" || inbox[0].CreatedAt == nil {
		t.Fatalf("Inquiries = %+v, %v", inbox, err)
	}
	stored := inbox[0]
	stored.ID, stored.CreatedAt = "", nil
	if !reflect.DeepEqual(stored, preview) {
		t.Fatalf("stored brief differs from confirmed preview: %+v, want %+v", stored, preview)
	}
	other, err := auth.NewLocal(store).SignUp(ctx, auth.Registration{Email: uniqueEmail(), Password: "alquileres-caba", Name: "Other", Role: auth.RoleRealtor})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.Inquiries(ctx, other.ID); err != nil || len(got) != 0 {
		t.Fatalf("another owner sees inquiry: %+v, %v", got, err)
	}
	for _, requirement := range inbox[0].Requirements {
		if requirement.Fact == "income_band" {
			t.Fatalf("the inquiry carries an income band the property never asked about: %+v", inbox[0])
		}
	}
	properties, err := store.List(ctx, owner.ID)
	if err != nil || properties[0].InquiryCount != 1 {
		t.Fatalf("List = %+v, %v", properties, err)
	}

	input.Requirements = nil
	if _, err := store.Update(ctx, owner.ID, created.ID, input); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if properties, _ := store.List(ctx, owner.ID); len(properties[0].Requirements) != 0 {
		t.Fatalf("requirements left after declaring none: %+v", properties[0].Requirements)
	}
	stale := inquiry
	stale.ID = "331c7ac0-320f-4e9b-9c35-ebd5fcb2b54d"
	if _, err := store.SendInquiry(ctx, created.ID, stale); !errors.Is(err, agency.ErrInquiryPreviewChanged) {
		t.Fatalf("changed preview silently sent: %v", err)
	}
	if err := store.Remove(ctx, owner.ID, created.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := store.SendInquiry(ctx, created.ID, inquiry); err != nil || got.Created || !reflect.DeepEqual(got.Requirements, sent.Requirements) {
		t.Fatalf("retry after edit/archive: %+v %v", got, err)
	}
	if _, err := store.SendInquiry(ctx, "999", inquiry); !errors.Is(err, agency.ErrInquiryConflict) {
		t.Fatalf("ID reused for another property: %v", err)
	}
}

func TestStore_AScrapedListingCannotReceiveAnInquiry(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	item := listing.Listing{Source: "zonaprop", URL: "https://example.com/scraped", Neighborhood: "palermo", Description: "Dos ambientes.", Operation: "alquiler", ScrapedAt: time.Now()}
	if err := store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReview(ctx, item.URL, quality.Review{Status: quality.Passed}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ByURL(ctx, item.URL)
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatInt(stored.ID, 10)
	input := agency.InquiryInput{ID: "331c7ac0-320f-4e9b-9c35-ebd5fcb2b54d", Name: "Ana", Email: "ana@example.com"}
	if _, err := store.PreviewInquiry(ctx, id, input); !errors.Is(err, agency.ErrPropertyNotFound) {
		t.Fatalf("scraped listing preview: %v", err)
	}
	if _, err := store.SendInquiry(ctx, id, input); !errors.Is(err, agency.ErrPropertyNotFound) {
		t.Fatalf("scraped listing send: %v", err)
	}
}

func TestStore_AnAgencyEditDoesNotRepublishAWithheldListing(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	owner, err := auth.NewLocal(store).SignUp(ctx, auth.Registration{
		Email: uniqueEmail(), Password: "alquileres-caba", Name: "Demo", Role: auth.RoleRealtor,
	})
	if err != nil {
		t.Fatal(err)
	}
	input := postgresAgencyPropertyInput()
	created, err := store.Add(ctx, agency.Owner{ID: owner.ID, Name: owner.Name}, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReview(ctx, input.URL, quality.Review{Status: quality.Withheld,
		Conflicts: []quality.Conflict{{First: "luminoso", Second: "sin luz natural", Detail: "luz"}}}); err != nil {
		t.Fatal(err)
	}
	input.Description += " Actualizado."
	if _, err := store.Update(ctx, owner.ID, created.ID, input); err != nil {
		t.Fatal(err)
	}
	candidates, err := store.Candidates(ctx, search.Query{Neighborhoods: []string{"palermo"}})
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(candidates, func(c eligibility.Candidate) bool { return strconv.FormatInt(c.Listing.ID, 10) == created.ID }) {
		t.Fatal("editing republished a listing withheld by a quality review")
	}
	if _, err := store.PreviewInquiry(ctx, created.ID, agency.InquiryInput{}); !errors.Is(err, agency.ErrPropertyNotFound) {
		t.Fatalf("withheld listing takes inquiries: %v", err)
	}
}
