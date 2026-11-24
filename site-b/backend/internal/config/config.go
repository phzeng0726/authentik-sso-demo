// Package config loads the backend settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

const (
	// DefaultPort is the listen port used when PORT is unset or empty.
	DefaultPort = 8082
	// DefaultSiteName is the site name used when SITE_NAME is unset or empty.
	DefaultSiteName = "Site B"
	// DefaultOIDCIssuer is the authentik issuer used when OIDC_ISSUER is unset or empty.
	DefaultOIDCIssuer = "http://localhost:9000/application/o/site-b/"
	// DefaultOIDCRedirectURL is the callback URL used when OIDC_REDIRECT_URL is unset or empty.
	DefaultOIDCRedirectURL = "http://localhost:3002/api/auth/callback"

	minPort = 1
	maxPort = 65535
)

var (
	// ErrInvalidPort is returned when PORT is not an integer from 1 to 65535.
	ErrInvalidPort = errors.New("invalid port")
	// ErrMissingConfig is returned when a required variable is unset or empty.
	ErrMissingConfig = errors.New("missing required config")
)

// Config holds the backend settings.
type Config struct {
	Port     int
	SiteName string
	OIDC     OIDCConfig
}

// OIDCConfig holds the settings for signing in through authentik.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// LookupFunc reports the value of an environment variable and whether it is set.
type LookupFunc func(key string) (string, bool)

// LoadFromEnv loads the config from the process environment.
func LoadFromEnv() (Config, error) {
	return Load(os.LookupEnv)
}

// Load loads the config using lookup, falling back to defaults for unset or empty
// optional values and failing when a required value is missing.
func Load(lookup LookupFunc) (Config, error) {
	get := func(key, fallback string) string {
		if v, ok := lookup(key); ok && v != "" {
			return v
		}
		return fallback
	}

	cfg := Config{
		Port:     DefaultPort,
		SiteName: get("SITE_NAME", DefaultSiteName),
		OIDC: OIDCConfig{
			Issuer:       get("OIDC_ISSUER", DefaultOIDCIssuer),
			ClientID:     get("OIDC_CLIENT_ID", ""),
			ClientSecret: get("OIDC_CLIENT_SECRET", ""),
			RedirectURL:  get("OIDC_REDIRECT_URL", DefaultOIDCRedirectURL),
		},
	}

	if v := get("PORT", ""); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < minPort || port > maxPort {
			return Config{}, fmt.Errorf("%w: %q must be an integer from %d to %d", ErrInvalidPort, v, minPort, maxPort)
		}
		cfg.Port = port
	}

	if cfg.OIDC.ClientID == "" {
		return Config{}, fmt.Errorf("%w: OIDC_CLIENT_ID is not set", ErrMissingConfig)
	}
	if cfg.OIDC.ClientSecret == "" {
		return Config{}, fmt.Errorf("%w: OIDC_CLIENT_SECRET is not set", ErrMissingConfig)
	}

	return cfg, nil
}
