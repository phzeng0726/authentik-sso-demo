package config_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"site-a/backend/internal/config"
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

func (s *ConfigSuite) TestLoad_AllUnset_UsesDefaults() {
	cfg, err := config.Load(envOf(map[string]string{}))

	s.Require().NoError(err)
	s.Equal(8081, cfg.Port)
	s.Equal("Site A", cfg.SiteName)
}

func (s *ConfigSuite) TestLoad_AllSet_UsesValues() {
	cfg, err := config.Load(envOf(map[string]string{
		"PORT":      "9100",
		"SITE_NAME": "Custom",
	}))

	s.Require().NoError(err)
	s.Equal(9100, cfg.Port)
	s.Equal("Custom", cfg.SiteName)
}

func (s *ConfigSuite) TestLoad_EmptyValues_UsesDefaults() {
	cfg, err := config.Load(envOf(map[string]string{
		"PORT":      "",
		"SITE_NAME": "",
	}))

	s.Require().NoError(err)
	s.Equal(8081, cfg.Port)
	s.Equal("Site A", cfg.SiteName)
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
			cfg, err := config.Load(envOf(map[string]string{"PORT": tc.port}))

			s.Require().NoError(err)
			s.Equal(tc.want, cfg.Port)
			s.Equal("Site A", cfg.SiteName)
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
			cfg, err := config.Load(envOf(map[string]string{"PORT": tc.port}))

			s.True(errors.Is(err, config.ErrInvalidPort), "expected ErrInvalidPort, got %v", err)
			s.Equal(config.Config{}, cfg)
		})
	}
}
