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

	minPort = 1
	maxPort = 65535
)

// ErrInvalidPort is returned when PORT is not an integer from 1 to 65535.
var ErrInvalidPort = errors.New("invalid port")

// Config holds the backend settings.
type Config struct {
	Port     int
	SiteName string
}

// LookupFunc reports the value of an environment variable and whether it is set.
type LookupFunc func(key string) (string, bool)

// LoadFromEnv loads the config from the process environment.
func LoadFromEnv() (Config, error) {
	return Load(os.LookupEnv)
}

// Load loads the config using lookup, falling back to defaults for unset or empty values.
func Load(lookup LookupFunc) (Config, error) {
	cfg := Config{
		Port:     DefaultPort,
		SiteName: DefaultSiteName,
	}

	if v, ok := lookup("PORT"); ok && v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < minPort || port > maxPort {
			return Config{}, fmt.Errorf("%w: %q must be an integer from %d to %d", ErrInvalidPort, v, minPort, maxPort)
		}
		cfg.Port = port
	}

	if v, ok := lookup("SITE_NAME"); ok && v != "" {
		cfg.SiteName = v
	}

	return cfg, nil
}
