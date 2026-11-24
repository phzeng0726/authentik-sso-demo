package auth_test

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"site-a/backend/internal/auth"
)

type StoreSuite struct {
	suite.Suite
	now   time.Time
	store *auth.Store
}

func TestStoreSuite(t *testing.T) {
	suite.Run(t, new(StoreSuite))
}

func (s *StoreSuite) SetupTest() {
	s.now = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	s.store = auth.NewStore(func() time.Time { return s.now })
}

var alice = auth.User{Sub: "abc", Username: "alice", Name: "Alice", Email: "alice@example.com"}

func (s *StoreSuite) TestCreateSession_ThenGet_ReturnsSameSession() {
	created := s.store.CreateSession(alice, "raw-id-token")

	s.Len(created.ID, 43) // 32 bytes, base64url without padding
	s.Equal(alice, created.User)
	s.Equal("raw-id-token", created.IDToken)
	s.Equal(s.now, created.CreatedAt)

	got, ok := s.store.GetSession(created.ID)
	s.Require().True(ok)
	s.Equal(created, got)
}

func (s *StoreSuite) TestCreateSession_GeneratesUniqueIDs() {
	a := s.store.CreateSession(alice, "t1")
	b := s.store.CreateSession(alice, "t2")

	s.NotEqual(a.ID, b.ID)
}

func (s *StoreSuite) TestGetSession_UnknownID_ReturnsFalse() {
	got, ok := s.store.GetSession("unknown")

	s.False(ok)
	s.Equal(auth.Session{}, got)
}

func (s *StoreSuite) TestGetSession_ExpiryBoundaries() {
	cases := []struct {
		name      string
		elapsed   time.Duration
		wantValid bool
	}{
		{name: "just before 8 hours", elapsed: 8*time.Hour - time.Nanosecond, wantValid: true},
		{name: "exactly 8 hours", elapsed: 8 * time.Hour, wantValid: false},
		{name: "after 8 hours", elapsed: 9 * time.Hour, wantValid: false},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			created := s.store.CreateSession(alice, "raw-id-token")
			s.now = s.now.Add(tc.elapsed)

			got, ok := s.store.GetSession(created.ID)

			s.Equal(tc.wantValid, ok)
			if tc.wantValid {
				s.Equal(created, got)
			} else {
				s.Equal(auth.Session{}, got)
			}
		})
	}
}

func (s *StoreSuite) TestGetSession_Expired_IsRemoved() {
	created := s.store.CreateSession(alice, "raw-id-token")
	s.now = s.now.Add(8 * time.Hour)
	_, _ = s.store.GetSession(created.ID)

	// Even if the clock went back, the expired session must be gone.
	s.now = s.now.Add(-8 * time.Hour)
	_, ok := s.store.GetSession(created.ID)
	s.False(ok)
}

func (s *StoreSuite) TestDeleteSession_RemovesOnlyThatSession() {
	a := s.store.CreateSession(alice, "t1")
	b := s.store.CreateSession(alice, "t2")

	s.store.DeleteSession(a.ID)

	_, okA := s.store.GetSession(a.ID)
	gotB, okB := s.store.GetSession(b.ID)
	s.False(okA)
	s.True(okB)
	s.Equal(b, gotB)
}

func (s *StoreSuite) TestCreatePendingLogin_ThenTake_ReturnsValues() {
	created := s.store.CreatePendingLogin("state-1", "nonce-1", "verifier-1", false)

	s.Len(created.ID, 43)
	s.Equal("state-1", created.State)
	s.Equal("nonce-1", created.Nonce)
	s.Equal("verifier-1", created.Verifier)
	s.False(created.Silent)
	s.Equal(s.now, created.CreatedAt)

	got, ok := s.store.TakePendingLogin(created.ID)
	s.Require().True(ok)
	s.Equal(created, got)
}

func (s *StoreSuite) TestCreatePendingLogin_Silent_IsKept() {
	created := s.store.CreatePendingLogin("state-1", "nonce-1", "verifier-1", true)

	got, ok := s.store.TakePendingLogin(created.ID)
	s.Require().True(ok)
	s.True(got.Silent)
	s.Equal(created, got)
}

func (s *StoreSuite) TestTakePendingLogin_CanOnlyBeUsedOnce() {
	created := s.store.CreatePendingLogin("state-1", "nonce-1", "verifier-1", false)
	_, _ = s.store.TakePendingLogin(created.ID)

	got, ok := s.store.TakePendingLogin(created.ID)

	s.False(ok)
	s.Equal(auth.PendingLogin{}, got)
}

func (s *StoreSuite) TestTakePendingLogin_UnknownID_ReturnsFalse() {
	got, ok := s.store.TakePendingLogin("unknown")

	s.False(ok)
	s.Equal(auth.PendingLogin{}, got)
}

func (s *StoreSuite) TestTakePendingLogin_ExpiryBoundaries() {
	cases := []struct {
		name      string
		elapsed   time.Duration
		wantValid bool
	}{
		{name: "just before 10 minutes", elapsed: 10*time.Minute - time.Nanosecond, wantValid: true},
		{name: "exactly 10 minutes", elapsed: 10 * time.Minute, wantValid: false},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			created := s.store.CreatePendingLogin("state-1", "nonce-1", "verifier-1", false)
			s.now = s.now.Add(tc.elapsed)

			got, ok := s.store.TakePendingLogin(created.ID)

			s.Equal(tc.wantValid, ok)
			if tc.wantValid {
				s.Equal(created, got)
			} else {
				s.Equal(auth.PendingLogin{}, got)
			}
		})
	}
}

func (s *StoreSuite) TestRandomString_Is32BytesBase64URL() {
	v := auth.RandomString()

	decoded, err := base64.RawURLEncoding.DecodeString(v)
	s.Require().NoError(err)
	s.Len(decoded, 32)
	s.NotEqual(v, auth.RandomString())
}
