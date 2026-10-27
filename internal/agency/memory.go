package agency

import (
	"context"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

// MemoryCatalog is the local adapter used when Postgres is unavailable and by
// tests that exercise the application interface without network or disk I/O.
type MemoryCatalog struct {
	// Facts is the eligibility fact catalog that declared requirements and
	// inquiries are checked against, as the eligibility_facts table is.
	Facts eligibility.Catalog

	mu         sync.Mutex
	nextID     int
	properties map[string][]Property
	intents    map[string]string
	inquiries  []storedInquiry
}

type storedInquiry struct {
	ownerID string
	Inquiry
}

// NewMemoryCatalog returns an empty catalog.
func NewMemoryCatalog() *MemoryCatalog {
	return &MemoryCatalog{
		properties: make(map[string][]Property),
		intents:    make(map[string]string),
	}
}

// List returns a copy of one owner's active catalog.
func (m *MemoryCatalog) List(_ context.Context, ownerID string) ([]Property, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	properties := append([]Property(nil), m.properties[ownerID]...)
	for index := range properties {
		for _, stored := range m.inquiries {
			if stored.ListingID == properties[index].ID {
				properties[index].InquiryCount++
			}
		}
	}
	return properties, nil
}

func (m *MemoryCatalog) prepare(input PropertyInput) (PropertyInput, error) {
	input, err := PreparePropertyInput(input)
	if err != nil {
		return PropertyInput{}, err
	}
	input.Requirements, err = PrepareRequirements(input.Requirements, m.Facts)
	return input, err
}

// Add creates a property under exactly one owner.
func (m *MemoryCatalog) Add(_ context.Context, owner Owner, input PropertyInput) (Property, error) {
	input, err := m.prepare(input)
	if err != nil {
		return Property{}, err
	}
	if owner.ID == "" {
		return Property{}, ValidationError{Field: "owner", Reason: "es obligatorio"}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	property := Property{
		ID:            strconv.Itoa(m.nextID),
		Agency:        owner.Name,
		Source:        "agency",
		PropertyInput: input,
	}
	m.properties[owner.ID] = append(m.properties[owner.ID], property)
	return property, nil
}

// Update replaces editable fields while preserving ownership and counters.
func (m *MemoryCatalog) Update(_ context.Context, ownerID, propertyID string, input PropertyInput) (Property, error) {
	input, err := m.prepare(input)
	if err != nil {
		return Property{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	properties := m.properties[ownerID]
	for index := range properties {
		if properties[index].ID != propertyID {
			continue
		}
		properties[index].PropertyInput = input
		m.properties[ownerID] = properties
		return properties[index], nil
	}
	return Property{}, ErrPropertyNotFound
}

// Remove takes a property out of the active catalog. Durable adapters may
// archive the row rather than deleting its history.
func (m *MemoryCatalog) Remove(_ context.Context, ownerID, propertyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	properties := m.properties[ownerID]
	for index := range properties {
		if properties[index].ID != propertyID {
			continue
		}
		m.properties[ownerID] = append(properties[:index], properties[index+1:]...)
		return nil
	}
	return ErrPropertyNotFound
}

// RecordContactIntent increments one active property's aggregate once per
// intent ID, even when the caller retries.
func (m *MemoryCatalog) RecordContactIntent(_ context.Context, propertyID string, intent ContactIntent) (ContactReceipt, error) {
	if err := ValidateContactIntent(intent); err != nil {
		return ContactReceipt{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if recordedPropertyID, ok := m.intents[intent.ID]; ok {
		if recordedPropertyID != propertyID {
			return ContactReceipt{}, ErrContactIntentConflict
		}
		return ContactReceipt{
			IntentID:  intent.ID,
			ListingID: propertyID,
			Recorded:  true,
		}, nil
	}

	for ownerID, properties := range m.properties {
		for index := range properties {
			if properties[index].ID != propertyID {
				continue
			}
			properties[index].ContactCount++
			m.properties[ownerID] = properties
			m.intents[intent.ID] = propertyID
			return ContactReceipt{
				IntentID:  intent.ID,
				ListingID: propertyID,
				Recorded:  true,
				Created:   true,
			}, nil
		}
	}
	return ContactReceipt{}, ErrPropertyNotFound
}

// PreviewInquiry builds the inquiry a searcher would send, storing nothing.
func (m *MemoryCatalog) PreviewInquiry(_ context.Context, propertyID string, input InquiryInput) (Inquiry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	inquiry, _, err := m.buildInquiry(propertyID, input)
	return inquiry, err
}

// SendInquiry delivers an inquiry once per inquiry id, even when retried.
func (m *MemoryCatalog) SendInquiry(_ context.Context, propertyID string, input InquiryInput) (Inquiry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := ValidateInquiryID(input.ID); err != nil {
		return Inquiry{}, err
	}
	for _, stored := range m.inquiries {
		if stored.ID != input.ID {
			continue
		}
		if stored.ListingID != propertyID {
			return Inquiry{}, ErrInquiryConflict
		}
		return stored.Inquiry, nil
	}
	inquiry, ownerID, err := m.buildInquiry(propertyID, input)
	if err != nil {
		return Inquiry{}, err
	}
	if err := ValidateInquiryContact(inquiry, input.ID); err != nil {
		return Inquiry{}, err
	}
	if input.PreviewRevision != inquiry.PreviewRevision {
		return Inquiry{}, ErrInquiryPreviewChanged
	}
	now := time.Now().UTC()
	inquiry.ID, inquiry.CreatedAt = input.ID, &now
	m.inquiries = append(m.inquiries, storedInquiry{ownerID: ownerID, Inquiry: inquiry})
	inquiry.Created = true
	return inquiry, nil
}

// Inquiries returns the inquiries about one owner's properties, newest first.
func (m *MemoryCatalog) Inquiries(_ context.Context, ownerID string) ([]Inquiry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var inquiries []Inquiry
	for _, stored := range slices.Backward(m.inquiries) {
		if stored.ownerID == ownerID {
			inquiries = append(inquiries, stored.Inquiry)
		}
	}
	return inquiries, nil
}

func (m *MemoryCatalog) buildInquiry(propertyID string, input InquiryInput) (Inquiry, string, error) {
	for ownerID, properties := range m.properties {
		for _, property := range properties {
			if property.ID != propertyID {
				continue
			}
			item := listing.Listing{
				Agency: property.Agency, Address: property.Address, Neighborhood: property.Neighborhood,
				Price:    listing.Money{Amount: property.Price.Amount, Currency: property.Price.Currency},
				Expenses: listing.Money{Amount: property.Expenses.Amount, Currency: property.Expenses.Currency},
			}
			inquiry, err := BuildInquiry(propertyID, item, property.Requirements, input, m.Facts)
			return inquiry, ownerID, err
		}
	}
	return Inquiry{}, "", ErrPropertyNotFound
}

var _ Catalog = (*MemoryCatalog)(nil)
