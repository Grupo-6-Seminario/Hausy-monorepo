package auth

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// MemoryStore keeps accounts in process memory. It backs tests, and lets the
// API offer sign-in on a clone with no database; everything is lost on restart.
type MemoryStore struct {
	mu       sync.Mutex
	users    map[string]storedUser // by email
	sessions map[string]storedSession
	nextID   int
}

type storedUser struct {
	user User
	hash string
}

type storedSession struct {
	email     string
	expiresAt time.Time
}

// NewMemoryStore returns an empty in-memory Store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{users: map[string]storedUser{}, sessions: map[string]storedSession{}}
}

func (m *MemoryStore) CreateUser(_ context.Context, input NewUser) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, taken := m.users[input.Email]; taken {
		return User{}, ErrEmailTaken
	}
	m.nextID++
	user := User{ID: strconv.Itoa(m.nextID), Email: input.Email, Name: input.Name, Role: input.Role}
	m.users[input.Email] = storedUser{user: user, hash: input.PasswordHash}
	return user, nil
}

func (m *MemoryStore) UserByEmail(_ context.Context, email string) (User, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.users[email]
	if !ok {
		return User{}, "", ErrUserNotFound
	}
	return stored.user, stored.hash, nil
}

func (m *MemoryStore) CreateSession(_ context.Context, tokenHash []byte, userID string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for email, stored := range m.users {
		if stored.user.ID == userID {
			m.sessions[string(tokenHash)] = storedSession{email: email, expiresAt: expiresAt}
			return nil
		}
	}
	return ErrUserNotFound
}

func (m *MemoryStore) SessionUser(_ context.Context, tokenHash []byte) (User, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[string(tokenHash)]
	if !ok {
		return User{}, time.Time{}, ErrUnauthenticated
	}
	return m.users[session.email].user, session.expiresAt, nil
}

func (m *MemoryStore) DeleteSession(_ context.Context, tokenHash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, string(tokenHash))
	return nil
}
