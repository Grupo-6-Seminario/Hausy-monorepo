package agency

import (
	"context"
	"strconv"
	"sync"
)

// MemoryCatalog is the local adapter used when Postgres is unavailable and by
// tests that exercise the application interface without network or disk I/O.
type MemoryCatalog struct {
	mu         sync.Mutex
	nextID     int
	properties map[string][]Property
	intents    map[string]string
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

	properties := m.properties[ownerID]
	return append([]Property(nil), properties...), nil
}

// Add creates a property under exactly one owner.
func (m *MemoryCatalog) Add(_ context.Context, owner Owner, input PropertyInput) (Property, error) {
	var err error
	input, err = PreparePropertyInput(input)
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
	var err error
	input, err = PreparePropertyInput(input)
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

var _ Catalog = (*MemoryCatalog)(nil)
