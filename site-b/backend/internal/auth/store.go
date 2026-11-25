// Package auth holds the OIDC sign-in building blocks: the authentik provider
// and the server-side store for sessions and in-progress logins.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

const (
	// SessionTTL is how long a session stays valid after it is created.
	SessionTTL = 8 * time.Hour
	// PendingLoginTTL is how long a started login may take to come back to the callback.
	PendingLoginTTL = 10 * time.Minute
)

// User is the signed-in user, taken from the ID token claims.
type User struct {
	Sub      string
	Username string
	Name     string
	Email    string
}

// Session is a signed-in browser session.
type Session struct {
	ID        string
	User      User
	IDToken   string // raw ID token, sent as id_token_hint on logout
	CreatedAt time.Time
}

// PendingLogin holds the values generated when a login starts, which the
// callback must check: state (against CSRF), nonce (against ID token replay)
// and the PKCE verifier (proves the same client is redeeming the code).
// Silent marks a prompt=none attempt, whose "not signed in" errors are expected.
type PendingLogin struct {
	ID        string
	State     string
	Nonce     string
	Verifier  string
	Silent    bool
	CreatedAt time.Time
}

// Store keeps sessions and pending logins in memory. It is safe for concurrent use.
type Store struct {
	mu       sync.Mutex
	now      func() time.Time
	sessions map[string]Session
	pending  map[string]PendingLogin
}

// NewStore creates an empty store. now is used as the clock; nil means time.Now.
func NewStore(now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{
		now:      now,
		sessions: make(map[string]Session),
		pending:  make(map[string]PendingLogin),
	}
}

// CreateSession stores a new session for user and returns it.
func (s *Store) CreateSession(user User, idToken string) Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess := Session{ID: RandomString(), User: user, IDToken: idToken, CreatedAt: s.now()}
	s.sessions[sess.ID] = sess
	return sess
}

// GetSession returns the session with id if it exists and has not expired.
// An expired session is removed.
func (s *Store) GetSession(id string) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[id]
	if !ok {
		return Session{}, false
	}
	if !s.now().Before(sess.CreatedAt.Add(SessionTTL)) {
		delete(s.sessions, id)
		return Session{}, false
	}
	return sess, true
}

// DeleteSession removes the session with id, if any.
func (s *Store) DeleteSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, id)
}

// CreatePendingLogin stores the values of a login that has just started.
func (s *Store) CreatePendingLogin(state, nonce, verifier string, silent bool) PendingLogin {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := PendingLogin{ID: RandomString(), State: state, Nonce: nonce, Verifier: verifier, Silent: silent, CreatedAt: s.now()}
	s.pending[p.ID] = p
	return p
}

// TakePendingLogin returns and removes the pending login with id, so it can be
// used only once. It reports false if it does not exist or has expired.
func (s *Store) TakePendingLogin(id string) (PendingLogin, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.pending[id]
	if !ok {
		return PendingLogin{}, false
	}
	delete(s.pending, id)
	if !s.now().Before(p.CreatedAt.Add(PendingLoginTTL)) {
		return PendingLogin{}, false
	}
	return p, true
}

// RandomString returns 32 cryptographically random bytes, base64url-encoded.
func RandomString() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // never returns an error since Go 1.24; it panics instead
	return base64.RawURLEncoding.EncodeToString(b)
}
