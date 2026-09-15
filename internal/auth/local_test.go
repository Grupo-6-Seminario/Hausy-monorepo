package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
)

func newProvider(t *testing.T, now func() time.Time) *auth.Local {
	t.Helper()
	return auth.NewLocal(auth.NewMemoryStore(), auth.WithClock(now))
}

func fixedClock(at time.Time) func() time.Time { return func() time.Time { return at } }

var registration = auth.Registration{
	Email:    "Sofia@Example.com",
	Password: "departamento-luminoso",
	Name:     "Sofía",
	Role:     auth.RoleSearcher,
}

func TestLocal_SignedInTokenAuthenticatesTheRegisteredUser(t *testing.T) {
	ctx := context.Background()
	provider := newProvider(t, time.Now)

	created, err := provider.SignUp(ctx, registration)
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if created.Email != "sofia@example.com" || created.Name != "Sofía" || created.Role != auth.RoleSearcher {
		t.Fatalf("unexpected user %+v", created)
	}

	session, err := provider.SignIn(ctx, "  SOFIA@example.com ", "departamento-luminoso")
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if session.Token == "" || session.User != created {
		t.Fatalf("unexpected session %+v", session)
	}

	user, err := provider.Authenticate(ctx, session.Token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if user != created {
		t.Fatalf("expected %+v, got %+v", created, user)
	}
}

func TestLocal_RejectsWrongPasswordAndUnknownEmailAlike(t *testing.T) {
	ctx := context.Background()
	provider := newProvider(t, time.Now)
	if _, err := provider.SignUp(ctx, registration); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if _, err := provider.SignIn(ctx, "sofia@example.com", "otra-clave-cualquiera"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("wrong password: expected ErrInvalidCredentials, got %v", err)
	}
	if _, err := provider.SignIn(ctx, "nadie@example.com", "departamento-luminoso"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("unknown email: expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLocal_EmailIsUniqueRegardlessOfCase(t *testing.T) {
	ctx := context.Background()
	provider := newProvider(t, time.Now)
	if _, err := provider.SignUp(ctx, registration); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	again := registration
	again.Email = "sofia@EXAMPLE.com"
	again.Role = auth.RoleRealtor
	if _, err := provider.SignUp(ctx, again); !errors.Is(err, auth.ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestLocal_RejectsInvalidRegistrations(t *testing.T) {
	cases := map[string]func(*auth.Registration){
		"missing @ in email":     func(r *auth.Registration) { r.Email = "sofia.example.com" },
		"password under 8 chars": func(r *auth.Registration) { r.Password = "corta12" },
		"unknown role":           func(r *auth.Registration) { r.Role = "admin" },
		"blank name":             func(r *auth.Registration) { r.Name = "   " },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			input := registration
			mutate(&input)
			_, err := newProvider(t, time.Now).SignUp(context.Background(), input)
			if !errors.Is(err, auth.ErrInvalidRegistration) {
				t.Fatalf("expected ErrInvalidRegistration, got %v", err)
			}
		})
	}
}

func TestLocal_SignOutRevokesTheToken(t *testing.T) {
	ctx := context.Background()
	provider := newProvider(t, time.Now)
	if _, err := provider.SignUp(ctx, registration); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	session, err := provider.SignIn(ctx, registration.Email, registration.Password)
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	if err := provider.SignOut(ctx, session.Token); err != nil {
		t.Fatalf("SignOut: %v", err)
	}
	if _, err := provider.Authenticate(ctx, session.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated after sign-out, got %v", err)
	}
}

func TestLocal_SessionsExpireAfterThirtyDays(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	store := auth.NewMemoryStore()
	signIn := auth.NewLocal(store, auth.WithClock(fixedClock(start)))
	if _, err := signIn.SignUp(ctx, registration); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	session, err := signIn.SignIn(ctx, registration.Email, registration.Password)
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if want := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC); !session.ExpiresAt.Equal(want) {
		t.Fatalf("expected expiry %v, got %v", want, session.ExpiresAt)
	}

	dayBefore := auth.NewLocal(store, auth.WithClock(fixedClock(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))))
	if _, err := dayBefore.Authenticate(ctx, session.Token); err != nil {
		t.Fatalf("expected the session to be valid a day before expiry, got %v", err)
	}
	after := auth.NewLocal(store, auth.WithClock(fixedClock(time.Date(2026, 10, 14, 12, 0, 1, 0, time.UTC))))
	if _, err := after.Authenticate(ctx, session.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated after expiry, got %v", err)
	}
}

func TestLocal_UnknownTokenIsUnauthenticated(t *testing.T) {
	_, err := newProvider(t, time.Now).Authenticate(context.Background(), "not-a-real-token")
	if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}
