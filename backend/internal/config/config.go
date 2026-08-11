// Package config reads runtime configuration from environment variables.
package config

import (
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	DatabaseURL   string
	Port          string
	ParishTZ      string
	PublicBaseURL string
}

// PublicHost returns the hostname of PublicBaseURL. iCalendar UIDs are minted
// under this host and a phone treats a UID change as a brand-new event, so a
// base URL without a scheme (a common misconfiguration, which url.Parse
// accepts without error while yielding no host) is reported rather than
// quietly replaced by a default.
func (c Config) PublicHost() (string, error) {
	u, err := url.Parse(c.PublicBaseURL)
	if err != nil {
		return "", fmt.Errorf("PUBLIC_BASE_URL %q is not a valid URL: %w", c.PublicBaseURL, err)
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("PUBLIC_BASE_URL %q has no host; include the scheme, e.g. https://calendario.example.mx", c.PublicBaseURL)
	}
	return u.Hostname(), nil
}

func Load() Config {
	return Config{
		DatabaseURL:   getenv("DATABASE_URL", "postgres://pastoral:pastoral@localhost:5433/pastoral?sslmode=disable"),
		Port:          getenv("PORT", "8080"),
		ParishTZ:      getenv("PARISH_TZ", "America/Mexico_City"),
		PublicBaseURL: getenv("PUBLIC_BASE_URL", "http://localhost:8080"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
