package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The methods below make Store an auth.Store, so accounts live in the same
// database as the listings they will eventually manage.

const uniqueViolation = "23505"

// CreateUser inserts an account, reporting a duplicate email as auth.ErrEmailTaken.
func (s *Store) CreateUser(ctx context.Context, input auth.NewUser) (auth.User, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, name, role, password_hash) VALUES ($1, $2, $3, $4) RETURNING id`,
		input.Email, input.Name, string(input.Role), input.PasswordHash,
	).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return auth.User{}, auth.ErrEmailTaken
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("postgres: create user: %w", err)
	}
	return auth.User{ID: strconv.FormatInt(id, 10), Email: input.Email, Name: input.Name, Role: input.Role}, nil
}

// UserByEmail returns the account and its password hash.
func (s *Store) UserByEmail(ctx context.Context, email string) (auth.User, string, error) {
	var user auth.User
	var id int64
	var hash string
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, name, role, password_hash FROM users WHERE email = $1`, email,
	).Scan(&id, &user.Email, &user.Name, &user.Role, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, "", auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, "", fmt.Errorf("postgres: read user: %w", err)
	}
	user.ID = strconv.FormatInt(id, 10)
	return user, hash, nil
}

// CreateSession records a session by the hash of its token.
func (s *Store) CreateSession(ctx context.Context, tokenHash []byte, userID string, expiresAt time.Time) error {
	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return auth.ErrUserNotFound
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, id, expiresAt,
	); err != nil {
		return fmt.Errorf("postgres: create session: %w", err)
	}
	return nil
}

// SessionUser resolves a token hash to its account and expiry.
func (s *Store) SessionUser(ctx context.Context, tokenHash []byte) (auth.User, time.Time, error) {
	var user auth.User
	var id int64
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT u.id, u.email, u.name, u.role, s.expires_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1`, tokenHash,
	).Scan(&id, &user.Email, &user.Name, &user.Role, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, time.Time{}, auth.ErrUnauthenticated
	}
	if err != nil {
		return auth.User{}, time.Time{}, fmt.Errorf("postgres: read session: %w", err)
	}
	user.ID = strconv.FormatInt(id, 10)
	return user, expiresAt, nil
}

// DeleteSession revokes a session; an unknown hash is not an error.
func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("postgres: delete session: %w", err)
	}
	return nil
}
