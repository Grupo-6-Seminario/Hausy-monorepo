package agency

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/eligibility"
	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/listing"
)

// ErrIneligibleInquiry means a strict requirement of the property rules the
// searcher out, so they cannot send an inquiry about it (CONTEXT.md, Inquiry).
var ErrIneligibleInquiry = errors.New("agency: a strict requirement rules the searcher out")

// ErrInquiryConflict means one inquiry id was reused for another property.
var ErrInquiryConflict = errors.New("agency: inquiry belongs to another property")

// ErrInvalidInquiry means the inquiry cannot be sent as written.
var ErrInvalidInquiry = errors.New("agency: invalid inquiry")

// ErrInquiryPreviewChanged requires the searcher to confirm the current brief.
var ErrInquiryPreviewChanged = errors.New("agency: inquiry preview changed")

// InquiryError names the field that keeps an inquiry from being sent. Reason
// is what the searcher reads.
type InquiryError struct {
	Field  string
	Reason string
}

func (e InquiryError) Error() string { return e.Field + ": " + e.Reason }

func (e InquiryError) Unwrap() error { return ErrInvalidInquiry }

// InquiryInput is what the searcher writes and declares. ID is generated once
// per deliberate send, so a retry delivers the inquiry once.
type InquiryInput struct {
	ID              string                    `json:"inquiry_id"`
	Name            string                    `json:"name"`
	Email           string                    `json:"email"`
	Phone           string                    `json:"phone"`
	Message         string                    `json:"message"`
	Searching       []string                  `json:"searching"`
	Qualification   eligibility.Qualification `json:"qualification"`
	PreviewRevision string                    `json:"preview_revision"`
}

// Inquiry is exactly what the agency receives and what the searcher sees
// before sending. Of what the searcher declared it keeps only the facts the
// property's requirements name; it never carries anything else they searched.
type Inquiry struct {
	ID              string               `json:"id,omitempty"`
	ListingID       string               `json:"listing_id"`
	Property        string               `json:"property"`
	Agency          string               `json:"agency"`
	Name            string               `json:"name"`
	Email           string               `json:"email,omitempty"`
	Phone           string               `json:"phone,omitempty"`
	Message         string               `json:"message,omitempty"`
	Searching       []string             `json:"searching,omitempty"`
	Eligibility     eligibility.State    `json:"eligibility"`
	Requirements    []InquiryRequirement `json:"requirements"`
	CreatedAt       *time.Time           `json:"created_at,omitempty"`
	PreviewRevision string               `json:"preview_revision"`
	// Created is transport metadata used to choose 201 versus an idempotent 200.
	Created bool `json:"-"`
}

// InquiryRequirement is one requirement of the property and what the searcher
// declared about it, by the catalog's labels. Declared is empty when they
// declared nothing about it.
type InquiryRequirement struct {
	Fact        string               `json:"fact"`
	Label       string               `json:"label"`
	Requirement string               `json:"requirement"`
	Hardness    eligibility.Hardness `json:"hardness"`
	Declared    []string             `json:"declared"`
	Outcome     eligibility.Reason   `json:"outcome"`
}

// BuildInquiry assesses the searcher against the property's requirements and
// returns the inquiry they would send, or ErrIneligibleInquiry. It checks the
// qualification, not the contact details: a preview may come before them.
func BuildInquiry(propertyID string, item listing.Listing, rules []eligibility.Rule, input InquiryInput, facts eligibility.Catalog) (Inquiry, error) {
	q := input.Qualification
	if q == nil {
		q = eligibility.Qualification{}
	}
	eligibility.DropRetired(q)
	if err := eligibility.ValidateQualification(facts, q); err != nil {
		return Inquiry{}, err
	}
	verdict := eligibility.Assess(q, item, rules, facts)
	if verdict.State == eligibility.Ineligible {
		return Inquiry{}, ErrIneligibleInquiry
	}
	inquiry := Inquiry{
		ListingID:    propertyID,
		Property:     cmp.Or(item.Address, "Propiedad en "+item.Neighborhood),
		Agency:       item.Agency,
		Name:         strings.TrimSpace(input.Name),
		Email:        strings.TrimSpace(input.Email),
		Phone:        strings.TrimSpace(input.Phone),
		Message:      strings.TrimSpace(input.Message),
		Eligibility:  verdict.State,
		Requirements: []InquiryRequirement{},
	}
	if validPhone(inquiry.Phone) {
		inquiry.Phone = "+" + strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, inquiry.Phone)
	}
	for _, phrase := range input.Searching {
		if phrase = strings.TrimSpace(phrase); phrase != "" {
			inquiry.Searching = append(inquiry.Searching, phrase)
		}
	}
	if len(inquiry.Searching) > 50 {
		return Inquiry{}, InquiryError{Field: "searching", Reason: "La búsqueda puede compartir hasta 50 condiciones."}
	}
	for _, r := range rules {
		if !facts.Admissible(r.Fact) {
			continue
		}
		// One rule at a time gives that rule's own outcome.
		outcome := eligibility.Met
		if alone := eligibility.Assess(q, item, []eligibility.Rule{r}, facts); len(alone.Conditions) > 0 {
			outcome = alone.Conditions[0].Reason
		}
		declared := []string{}
		for _, v := range q[r.Fact] {
			declared = append(declared, cmp.Or(facts.ChoiceLabel(r.Fact, v), v))
		}
		inquiry.Requirements = append(inquiry.Requirements, InquiryRequirement{
			Fact: r.Fact, Label: facts[r.Fact].Label, Requirement: cmp.Or(r.Evidence, r.Fact),
			Hardness: r.Hardness, Declared: declared, Outcome: outcome,
		})
	}
	// Contact fields can be filled after previewing. Everything the agency
	// receives about the property, qualification and search must stay identical.
	brief, _ := json.Marshal(struct {
		ListingID, Property, Agency string
		Eligibility                 eligibility.State
		Requirements                []InquiryRequirement
		Searching                   []string
	}{inquiry.ListingID, inquiry.Property, inquiry.Agency, inquiry.Eligibility, inquiry.Requirements, inquiry.Searching})
	digest := sha256.Sum256(brief)
	inquiry.PreviewRevision = hex.EncodeToString(digest[:])
	return inquiry, nil
}

// ValidateInquiryID checks the idempotency key before an adapter looks it up.
func ValidateInquiryID(id string) error {
	if !validUUID(id) {
		return InquiryError{Field: "inquiry_id", Reason: "La consulta no es válida. Probá de nuevo."}
	}
	return nil
}

// ValidateInquiryContact checks what sending needs beyond a preview: an id
// and a way for the agency to answer.
func ValidateInquiryContact(inquiry Inquiry, id string) error {
	if err := ValidateInquiryID(id); err != nil {
		return err
	}
	switch {
	case inquiry.Name == "" || utf8.RuneCountInString(inquiry.Name) > 80:
		return InquiryError{Field: "name", Reason: "Contanos tu nombre."}
	case inquiry.Email == "" && inquiry.Phone == "":
		return InquiryError{Field: "contact", Reason: "Dejá un email o un WhatsApp para que la inmobiliaria te responda."}
	case inquiry.Email != "" && (len(inquiry.Email) > 254 || !strings.Contains(strings.TrimLeft(inquiry.Email, "@"), "@")):
		return InquiryError{Field: "email", Reason: "Revisá el email."}
	case inquiry.Phone != "" && !validPhone(inquiry.Phone):
		return InquiryError{Field: "phone", Reason: "Ingresá el WhatsApp con + y el código de país, por ejemplo +54 9 11 5555-1234."}
	case utf8.RuneCountInString(inquiry.Message) > 1000:
		return InquiryError{Field: "message", Reason: "El mensaje puede tener hasta 1000 caracteres."}
	}
	return nil
}

func validPhone(phone string) bool {
	if !strings.HasPrefix(phone, "+") {
		return false
	}
	digits := 0
	for _, r := range phone[1:] {
		switch {
		case r >= '0' && r <= '9':
			if digits == 0 && r == '0' {
				return false
			}
			digits++
		case !strings.ContainsRune(" -()", r):
			return false
		}
	}
	return digits >= 7 && digits <= 15
}
