package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/suite"

	"site-a/backend/internal/auth"
	"site-a/backend/internal/config"
)

// fakeIDP is a minimal OIDC provider: discovery, JWKS and token endpoints.
type fakeIDP struct {
	server *httptest.Server
	key    *rsa.PrivateKey

	// tokenStatus and idToken control the token endpoint response.
	tokenStatus int
	idToken     string
	// lastTokenForm is the form the token endpoint last received.
	lastTokenForm url.Values
}

func newFakeIDP(t *testing.T) *fakeIDP {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIDP{key: key, tokenStatus: http.StatusOK}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                f.issuer(),
			"authorization_endpoint":                f.server.URL + "/authorize",
			"token_endpoint":                        f.server.URL + "/token",
			"jwks_uri":                              f.server.URL + "/jwks",
			"end_session_endpoint":                  f.server.URL + "/end-session/",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &f.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"},
		}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.lastTokenForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.tokenStatus)
		if f.tokenStatus != http.StatusOK {
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		resp := map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 300}
		if f.idToken != "" {
			resp["id_token"] = f.idToken
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeIDP) issuer() string { return f.server.URL + "/" }

// sign returns an RS256 ID token with standard claims plus extra.
func (f *fakeIDP) sign(t *testing.T, key *rsa.PrivateKey, std jwt.Claims, extra map[string]any) string {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(std).Claims(extra).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (f *fakeIDP) validClaims() jwt.Claims {
	now := time.Now()
	return jwt.Claims{
		Issuer:   f.issuer(),
		Subject:  "abc",
		Audience: jwt.Audience{"client-1"},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}
}

var userClaims = map[string]any{
	"nonce":              "nonce-1",
	"preferred_username": "alice",
	"name":               "Alice",
	"email":              "alice@example.com",
}

type ProviderSuite struct {
	suite.Suite
	idp      *fakeIDP
	provider *auth.OIDCProvider
}

func TestProviderSuite(t *testing.T) {
	suite.Run(t, new(ProviderSuite))
}

func (s *ProviderSuite) SetupTest() {
	s.idp = newFakeIDP(s.T())
	p, err := auth.NewOIDCProvider(context.Background(), config.OIDCConfig{
		Issuer:       s.idp.issuer(),
		ClientID:     "client-1",
		ClientSecret: "secret-1",
		RedirectURL:  "http://localhost:3001/api/auth/callback",
	})
	s.Require().NoError(err)
	s.provider = p
}

func (s *ProviderSuite) TestNewOIDCProvider_DiscoveryFails_ReturnsError() {
	_, err := auth.NewOIDCProvider(context.Background(), config.OIDCConfig{
		Issuer:   s.idp.server.URL + "/does-not-exist/",
		ClientID: "client-1",
	})

	s.ErrorContains(err, "oidc discovery for "+s.idp.server.URL+"/does-not-exist/")
}

func (s *ProviderSuite) TestAuthCodeURL_ContainsAllParameters() {
	raw := s.provider.AuthCodeURL("state-1", "nonce-1", "verifier-1", false)

	u, err := url.Parse(raw)
	s.Require().NoError(err)
	s.Equal(s.idp.server.URL+"/authorize", u.Scheme+"://"+u.Host+u.Path)

	q := u.Query()
	s.Equal("code", q.Get("response_type"))
	s.Equal("client-1", q.Get("client_id"))
	s.Equal("http://localhost:3001/api/auth/callback", q.Get("redirect_uri"))
	s.Equal("openid profile email", q.Get("scope"))
	s.Equal("state-1", q.Get("state"))
	s.Equal("nonce-1", q.Get("nonce"))
	s.Equal("S256", q.Get("code_challenge_method"))
	// S256 challenge of "verifier-1" = base64url(sha256("verifier-1")), computed with:
	// printf 'verifier-1' | openssl dgst -sha256 -binary | base64 | tr '+/' '-_' | tr -d '='
	s.Equal("xTYRBLTDt1gS3-rw_3FBVafZ0iNg9bZYBbG8fRPvkEs", q.Get("code_challenge"))
	s.False(q.Has("prompt"))
}

func (s *ProviderSuite) TestAuthCodeURL_Silent_AddsPromptNone() {
	raw := s.provider.AuthCodeURL("state-1", "nonce-1", "verifier-1", true)

	u, err := url.Parse(raw)
	s.Require().NoError(err)
	q := u.Query()
	s.Equal("none", q.Get("prompt"))
	s.Equal("state-1", q.Get("state"))
	s.Equal("nonce-1", q.Get("nonce"))
	s.Equal("xTYRBLTDt1gS3-rw_3FBVafZ0iNg9bZYBbG8fRPvkEs", q.Get("code_challenge"))
}

func (s *ProviderSuite) TestExchange_Success_ReturnsVerifiedToken() {
	s.idp.idToken = s.idp.sign(s.T(), s.idp.key, s.idp.validClaims(), userClaims)

	got, err := s.provider.Exchange(context.Background(), "code-1", "verifier-1")

	s.Require().NoError(err)
	s.Equal(auth.IDToken{
		Raw:   s.idp.idToken,
		User:  auth.User{Sub: "abc", Username: "alice", Name: "Alice", Email: "alice@example.com"},
		Nonce: "nonce-1",
	}, got)
	s.Equal("authorization_code", s.idp.lastTokenForm.Get("grant_type"))
	s.Equal("code-1", s.idp.lastTokenForm.Get("code"))
	s.Equal("verifier-1", s.idp.lastTokenForm.Get("code_verifier"))
	s.Equal("http://localhost:3001/api/auth/callback", s.idp.lastTokenForm.Get("redirect_uri"))
}

func (s *ProviderSuite) TestExchange_TokenEndpointFails_ReturnsErrTokenExchange() {
	s.idp.tokenStatus = http.StatusBadRequest

	_, err := s.provider.Exchange(context.Background(), "bad-code", "verifier-1")

	s.True(errors.Is(err, auth.ErrTokenExchange), "got %v", err)
}

func (s *ProviderSuite) TestExchange_InvalidIDToken_ReturnsErrInvalidIDToken() {
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)

	wrongAudience := s.idp.validClaims()
	wrongAudience.Audience = jwt.Audience{"other-client"}
	wrongIssuer := s.idp.validClaims()
	wrongIssuer.Issuer = "http://evil/"
	expired := s.idp.validClaims()
	expired.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Minute))

	cases := []struct {
		name    string
		idToken func() string
	}{
		{name: "no id_token", idToken: func() string { return "" }},
		{name: "signed by another key", idToken: func() string { return s.idp.sign(s.T(), otherKey, s.idp.validClaims(), userClaims) }},
		{name: "wrong audience", idToken: func() string { return s.idp.sign(s.T(), s.idp.key, wrongAudience, userClaims) }},
		{name: "wrong issuer", idToken: func() string { return s.idp.sign(s.T(), s.idp.key, wrongIssuer, userClaims) }},
		{name: "expired", idToken: func() string { return s.idp.sign(s.T(), s.idp.key, expired, userClaims) }},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.idp.idToken = tc.idToken()

			got, err := s.provider.Exchange(context.Background(), "code-1", "verifier-1")

			s.True(errors.Is(err, auth.ErrInvalidIDToken), "got %v", err)
			s.Equal(auth.IDToken{}, got)
		})
	}
}

func (s *ProviderSuite) TestEndSessionURL() {
	s.Equal(s.idp.server.URL+"/end-session/", s.provider.EndSessionURL(""))
	s.Equal(s.idp.server.URL+"/end-session/?id_token_hint=tok-1", s.provider.EndSessionURL("tok-1"))
}
