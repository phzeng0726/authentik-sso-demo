package auth

//go:generate go tool mockgen -source=provider.go -destination=mock/provider_mock.go -package=mock

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"site-a/backend/internal/config"
)

var (
	// ErrTokenExchange is returned when the authorization code cannot be exchanged for tokens.
	ErrTokenExchange = errors.New("token exchange failed")
	// ErrInvalidIDToken is returned when the ID token is missing or fails verification.
	ErrInvalidIDToken = errors.New("invalid id token")
)

// IDToken is a verified ID token.
type IDToken struct {
	Raw   string
	User  User
	Nonce string
}

// Provider talks to the OIDC identity provider (authentik).
type Provider interface {
	// AuthCodeURL returns the authorization endpoint URL that starts a login
	// with the given state, nonce and PKCE verifier (sent as an S256 challenge).
	// It sends no prompt: authentik signs the user in at once when it already
	// has a session, and shows its login page otherwise.
	AuthCodeURL(state, nonce, verifier string) string
	// Exchange redeems the authorization code with the PKCE verifier, then
	// verifies the returned ID token (signature, issuer, audience, expiry).
	// It does not check the nonce; the caller compares IDToken.Nonce.
	Exchange(ctx context.Context, code, verifier string) (IDToken, error)
	// EndSessionURL returns the logout URL. When idTokenHint is not empty it
	// adds id_token_hint and the configured post_logout_redirect_uri.
	EndSessionURL(idTokenHint string) string
}

// OIDCProvider implements Provider with go-oidc and x/oauth2.
type OIDCProvider struct {
	oauth              oauth2.Config
	verifier           *oidc.IDTokenVerifier
	endSession         string
	postLogoutRedirect string
}

// NewOIDCProvider fetches the discovery document from cfg.Issuer and builds the provider.
func NewOIDCProvider(ctx context.Context, cfg config.OIDCConfig) (*OIDCProvider, error) {
	p, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery for %s: %w", cfg.Issuer, err)
	}

	var meta struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := p.Claims(&meta); err != nil {
		return nil, fmt.Errorf("read discovery metadata: %w", err)
	}

	return &OIDCProvider{
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     p.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		verifier:           p.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		endSession:         meta.EndSessionEndpoint,
		postLogoutRedirect: cfg.PostLogoutRedirectURL,
	}, nil
}

// AuthCodeURL implements Provider.
func (p *OIDCProvider) AuthCodeURL(state, nonce, verifier string) string {
	return p.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
}

// Exchange implements Provider.
func (p *OIDCProvider) Exchange(ctx context.Context, code, verifier string) (IDToken, error) {
	tok, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return IDToken{}, fmt.Errorf("%w: %w", ErrTokenExchange, err)
	}

	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return IDToken{}, fmt.Errorf("%w: token response has no id_token", ErrInvalidIDToken)
	}

	idt, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return IDToken{}, fmt.Errorf("%w: %w", ErrInvalidIDToken, err)
	}

	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Name              string `json:"name"`
		Email             string `json:"email"`
	}
	if err := idt.Claims(&claims); err != nil {
		return IDToken{}, fmt.Errorf("%w: %w", ErrInvalidIDToken, err)
	}

	return IDToken{
		Raw: raw,
		User: User{
			Sub:      idt.Subject,
			Username: claims.PreferredUsername,
			Name:     claims.Name,
			Email:    claims.Email,
		},
		Nonce: idt.Nonce,
	}, nil
}

// EndSessionURL implements Provider.
func (p *OIDCProvider) EndSessionURL(idTokenHint string) string {
	return BuildEndSessionURL(p.endSession, idTokenHint, p.postLogoutRedirect)
}

// BuildEndSessionURL adds id_token_hint and post_logout_redirect_uri to
// endpoint when hint is not empty. Without a hint it returns endpoint as is,
// because the IdP may reject post_logout_redirect_uri without id_token_hint.
// An empty postLogoutRedirect is left out.
func BuildEndSessionURL(endpoint, hint, postLogoutRedirect string) string {
	if hint == "" {
		return endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	q := u.Query()
	q.Set("id_token_hint", hint)
	if postLogoutRedirect != "" {
		q.Set("post_logout_redirect_uri", postLogoutRedirect)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
