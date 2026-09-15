package auth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SessionLifetime is how long a sign-in stays valid.
const SessionLifetime = 30 * 24 * time.Hour

// Password hashing follows the OWASP recommendation for PBKDF2-HMAC-SHA256.
const (
	hashIterations = 600_000
	saltBytes      = 16
	keyBytes       = 32
	minPassword    = 8
)

// NewUser is an account ready to be stored, password already hashed.
type NewUser struct {
	Email        string
	Name         string
	Role         Role
	PasswordHash string
}

// ErrUserNotFound is returned by a Store when no account has the given email.
var ErrUserNotFound = errors.New("auth: user not found")

// Store persists accounts and sessions for Local. Sessions are keyed by the
// SHA-256 of their token, so a leaked table cannot be replayed as logins.
type Store interface {
	// CreateUser returns ErrEmailTaken when the email is already registered.
	CreateUser(ctx context.Context, user NewUser) (User, error)
	// UserByEmail returns ErrUserNotFound when no account matches.
	UserByEmail(ctx context.Context, email string) (User, string, error)
	CreateSession(ctx context.Context, tokenHash []byte, userID string, expiresAt time.Time) error
	// SessionUser returns ErrUnauthenticated when no session matches.
	SessionUser(ctx context.Context, tokenHash []byte) (User, time.Time, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
}

// Local is a Provider that keeps accounts in Hausy's own database.
type Local struct {
	store Store
	now   func() time.Time
	// dummyHash is verified against when an email is unknown, so a sign-in for
	// a missing account takes as long as one with a wrong password.
	dummyHash string
}

// LocalOption configures a Local provider.
type LocalOption func(*Local)

// WithClock replaces the wall clock, for tests that cross session expiry.
func WithClock(now func() time.Time) LocalOption {
	return func(l *Local) { l.now = now }
}

// NewLocal returns a Provider backed by store.
func NewLocal(store Store, options ...LocalOption) *Local {
	local := &Local{store: store, now: time.Now}
	for _, option := range options {
		option(local)
	}
	local.dummyHash, _ = hashPassword("hausy-timing-equalizer")
	return local
}

// SignUp validates and stores a new account.
func (l *Local) SignUp(ctx context.Context, registration Registration) (User, error) {
	email := normalizeEmail(registration.Email)
	name := strings.TrimSpace(registration.Name)
	switch {
	case !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@"):
		return User{}, RegistrationError{Reason: "El email no es válido."}
	case len([]rune(registration.Password)) < minPassword:
		return User{}, RegistrationError{Reason: fmt.Sprintf("La contraseña debe tener al menos %d caracteres.", minPassword)}
	case name == "":
		return User{}, RegistrationError{Reason: "El nombre es obligatorio."}
	case !registration.Role.Valid():
		return User{}, RegistrationError{Reason: "El tipo de cuenta no es válido."}
	}

	hash, err := hashPassword(registration.Password)
	if err != nil {
		return User{}, err
	}
	return l.store.CreateUser(ctx, NewUser{Email: email, Name: name, Role: registration.Role, PasswordHash: hash})
}

// SignIn checks the password and opens a session.
func (l *Local) SignIn(ctx context.Context, email, password string) (Session, error) {
	user, hash, err := l.store.UserByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, ErrUserNotFound) {
		verifyPassword(l.dummyHash, password)
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	if !verifyPassword(hash, password) {
		return Session{}, ErrInvalidCredentials
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, fmt.Errorf("auth: generate token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expiresAt := l.now().Add(SessionLifetime)
	if err := l.store.CreateSession(ctx, tokenHash(token), user.ID, expiresAt); err != nil {
		return Session{}, err
	}
	return Session{Token: token, ExpiresAt: expiresAt, User: user}, nil
}

// Authenticate resolves a bearer token to its user.
func (l *Local) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthenticated
	}
	user, expiresAt, err := l.store.SessionUser(ctx, tokenHash(token))
	if err != nil {
		return User{}, err
	}
	if !l.now().Before(expiresAt) {
		return User{}, ErrUnauthenticated
	}
	return user, nil
}

// SignOut revokes a token. Signing out an unknown token is not an error.
func (l *Local) SignOut(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return l.store.DeleteSession(ctx, tokenHash(token))
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func tokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// hashPassword encodes as pbkdf2-sha256$<iterations>$<salt>$<key>, so the cost
// can be raised later without invalidating existing hashes.
func hashPassword(password string) (string, error) {
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: generate salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, hashIterations, keyBytes)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	encode := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", hashIterations, encode(salt), encode(key)), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}
