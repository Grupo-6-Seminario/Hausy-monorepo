package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
)

// Accounts are not truncated between runs, so each test registers a fresh email.
func uniqueEmail() string {
	return fmt.Sprintf("user-%d@example.com", time.Now().UnixNano())
}

func TestStore_BacksLocalAuthEndToEnd(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	provider := auth.NewLocal(store)
	email := uniqueEmail()

	created, err := provider.SignUp(ctx, auth.Registration{
		Email: email, Password: "alquileres-caba", Name: "Marta", Role: auth.RoleRealtor,
	})
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if created.ID == "" || created.Email != email || created.Role != auth.RoleRealtor {
		t.Fatalf("unexpected user %+v", created)
	}

	session, err := provider.SignIn(ctx, email, "alquileres-caba")
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	user, err := provider.Authenticate(ctx, session.Token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if user != created {
		t.Fatalf("expected %+v, got %+v", created, user)
	}

	if err := provider.SignOut(ctx, session.Token); err != nil {
		t.Fatalf("SignOut: %v", err)
	}
	if _, err := provider.Authenticate(ctx, session.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated after sign-out, got %v", err)
	}
}

func TestStore_ReportsTakenEmailsAndUnknownUsers(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	provider := auth.NewLocal(store)
	email := uniqueEmail()
	registration := auth.Registration{Email: email, Password: "departamento", Name: "Sofía", Role: auth.RoleSearcher}

	if _, err := provider.SignUp(ctx, registration); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if _, err := provider.SignUp(ctx, registration); !errors.Is(err, auth.ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
	if _, err := provider.SignIn(ctx, "missing-"+email, "departamento"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if _, err := provider.Authenticate(ctx, "unknown-token"); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}
