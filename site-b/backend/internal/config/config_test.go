package config_test

import (
	"errors"
	"maps"
	"testing"

	"github.com/stretchr/testify/suite"

	"site-b/backend/internal/config"
)

type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

// envOf returns a LookupFunc backed by the given map.
func envOf(env map[string]string) config.LookupFunc {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

// withRequired returns env plus the required OIDC credentials.
func withRequired(env map[string]string) map[string]string {
	out := map[string]string{
		"OIDC_CLIENT_ID":     "id",
		"OIDC_CLIENT_SECRET": "secret",
	}
	maps.Copy(out, env)
	return out
}

func (s *ConfigSuite) TestLoad_OnlyRequiredSet_UsesDefaults() {
	cfg, err := config.Load(envOf(withRequired(nil)))

	s.Require().NoError(err)
	s.Equal(config.Config{
		Port:     8082,
		SiteName: "Site B",
		OIDC: config.OIDCConfig{
			Issuer:                "http://localhost:9000/application/o/site-b/",
			ClientID:              "id",
			ClientSecret:          "secret",
			RedirectURL:           "http://localhost:3002/api/auth/callback",
			PostLogoutRedirectURL: "http://localhost:3002/",
		},
	}, cfg)
}

func (s *ConfigSuite) TestLoad_AllSet_UsesValues() {
	cfg, err := config.Load(envOf(map[string]string{
		"PORT":                          "9100",
		"SITE_NAME":                     "Custom",
		"OIDC_ISSUER":                   "http://idp/",
		"OIDC_REDIRECT_URL":             "http://app/cb",
		"OIDC_POST_LOGOUT_REDIRECT_URL": "http://app/",
		"OIDC_CLIENT_ID":                "my-id",
		"OIDC_CLIENT_SECRET":            "my-secret",
	}))

	s.Require().NoError(err)
	s.Equal(config.Config{
		Port:     9100,
		SiteName: "Custom",
		OIDC: config.OIDCConfig{
			Issuer:                "http://idp/",
			ClientID:              "my-id",
			ClientSecret:          "my-secret",
			RedirectURL:           "http://app/cb",
			PostLogoutRedirectURL: "http://app/",
		},
	}, cfg)
}

func (s *ConfigSuite) TestLoad_EmptyOptionalValues_UsesDefaults() {
	cfg, err := config.Load(envOf(withRequired(map[string]string{
		"PORT":                          "",
		"SITE_NAME":                     "",
		"OIDC_ISSUER":                   "",
		"OIDC_REDIRECT_URL":             "",
		"OIDC_POST_LOGOUT_REDIRECT_URL": "",
	})))

	s.Require().NoError(err)
	s.Equal(8082, cfg.Port)
	s.Equal("Site B", cfg.SiteName)
	s.Equal("http://localhost:9000/application/o/site-b/", cfg.OIDC.Issuer)
	s.Equal("http://localhost:3002/api/auth/callback", cfg.OIDC.RedirectURL)
	s.Equal("http://localhost:3002/", cfg.OIDC.PostLogoutRedirectURL)
}

func (s *ConfigSuite) TestLoad_PortBoundaries_Valid() {
	cases := []struct {
		name string
		port string
		want int
	}{
		{name: "lower bound", port: "1", want: 1},
		{name: "upper bound", port: "65535", want: 65535},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cfg, err := config.Load(envOf(withRequired(map[string]string{"PORT": tc.port})))

			s.Require().NoError(err)
			s.Equal(tc.want, cfg.Port)
		})
	}
}

func (s *ConfigSuite) TestLoad_InvalidPort_ReturnsErrInvalidPort() {
	cases := []struct {
		name string
		port string
	}{
		{name: "below range", port: "0"},
		{name: "above range", port: "65536"},
		{name: "negative", port: "-1"},
		{name: "not a number", port: "abc"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cfg, err := config.Load(envOf(withRequired(map[string]string{"PORT": tc.port})))

			s.True(errors.Is(err, config.ErrInvalidPort), "expected ErrInvalidPort, got %v", err)
			s.Equal(config.Config{}, cfg)
		})
	}
}

func (s *ConfigSuite) TestLoad_MissingCredentials_ReturnsErrMissingConfig() {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{name: "client id unset", env: map[string]string{"OIDC_CLIENT_SECRET": "secret"}},
		{name: "client id empty", env: map[string]string{"OIDC_CLIENT_ID": "", "OIDC_CLIENT_SECRET": "secret"}},
		{name: "client secret unset", env: map[string]string{"OIDC_CLIENT_ID": "id"}},
		{name: "client secret empty", env: map[string]string{"OIDC_CLIENT_ID": "id", "OIDC_CLIENT_SECRET": ""}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			cfg, err := config.Load(envOf(tc.env))

			s.True(errors.Is(err, config.ErrMissingConfig), "expected ErrMissingConfig, got %v", err)
			s.Equal(config.Config{}, cfg)
		})
	}
}
