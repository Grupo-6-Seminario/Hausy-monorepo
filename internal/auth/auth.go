// Package auth identifies the people who sign in to Hausy.
//
// Searching stays anonymous; an account only lets a searcher's agent remember
// them and lets a realtor manage their properties. Everything outside this
// package talks to Provider, so the local email-and-password implementation
// can be replaced by a managed identity service (AWS Cognito) by writing one
// more Provider, without touching the HTTP layer or the interface.
package auth

import (
	"context"
	"errors"
	"time"
)

// Role is the kind of account. It decides what a signed-in user may do.
type Role string

const (
	// RoleSearcher looks for a home; signing in lets the agent remember them.
	RoleSearcher Role = "searcher"
	// RoleRealtor is a real estate agent who manages published properties.
	RoleRealtor Role = "realtor"
)

// Valid reports whether r is one of the known roles.
func (r Role) Valid() bool { return r == RoleSearcher || r == RoleRealtor }

// User is an account as the rest of the application sees it. ID is a string
// because external identity providers issue opaque subjects, not integers.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  Role   `json:"role"`
}

// Registration is what a person submits to create an account.
type Registration struct {
	Email    string
	Password string
	Name     string
	Role     Role
}

// Session is proof of a successful sign-in. Token is a bearer credential: the
// caller must keep it secret and present it on every authenticated request.
type Session struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

var (
	// ErrInvalidRegistration means the submitted account details are unusable.
	ErrInvalidRegistration = errors.New("auth: invalid registration")
	// ErrEmailTaken means an account already exists for that email.
	ErrEmailTaken = errors.New("auth: email already registered")
	// ErrInvalidCredentials deliberately does not say whether the email or the
	// password was wrong, so sign-in cannot be used to enumerate accounts.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	// ErrUnauthenticated means a token is missing, unknown, revoked or expired.
	ErrUnauthenticated = errors.New("auth: unauthenticated")
)

// RegistrationError explains, in words fit to show the person, why an account
// could not be created. It matches ErrInvalidRegistration under errors.Is.
type RegistrationError struct{ Reason string }

func (e RegistrationError) Error() string { return e.Reason }
func (e RegistrationError) Unwrap() error { return ErrInvalidRegistration }

// Provider is the seam between Hausy and whatever proves who a user is.
type Provider interface {
	SignUp(ctx context.Context, registration Registration) (User, error)
	SignIn(ctx context.Context, email, password string) (Session, error)
	Authenticate(ctx context.Context, token string) (User, error)
	SignOut(ctx context.Context, token string) error
}
