// Package config reads runtime configuration from environment variables.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL   string
	Port          string
	ParishTZ      string
	PublicBaseURL string
	RedisAddr     string
	TrustedProxy  string // CIDR of the reverse proxy; empty = trust nobody
	AuthSecret    string // HMAC key for magic-link tokens
	SMTPHost      string // empty = LogMailer (dev)
	SMTPPort      string
	SMTPUser      string
	SMTPPass      string
	SMTPFrom      string

	// QuietStart and QuietEnd bound the hours (parish local time) during
	// which the difusión engine holds messages instead of ringing phones.
	// Equal values disable quiet hours entirely.
	QuietStart int
	QuietEnd   int
	// StaggerSeconds spaces consecutive deliveries of one announcement, so a
	// fanout across many channels does not arrive as a single burst.
	StaggerSeconds int
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
		RedisAddr:     getenv("REDIS_ADDR", "localhost:6379"),
		TrustedProxy:  getenv("TRUSTED_PROXY", ""),
		AuthSecret:    getenv("AUTH_SECRET", "dev-secret-change-me"),
		SMTPHost:      getenv("SMTP_HOST", ""),
		SMTPPort:      getenv("SMTP_PORT", "587"),
		SMTPUser:      getenv("SMTP_USER", ""),
		SMTPPass:      getenv("SMTP_PASS", ""),
		SMTPFrom:      getenv("SMTP_FROM", ""),

		QuietStart:     getenvInt("QUIET_START", 22),
		QuietEnd:       getenvInt("QUIET_END", 7),
		StaggerSeconds: getenvInt("STAGGER_SECONDS", 8),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getenvInt falls back on anything non-numeric rather than on strconv's zero:
// a typo in QUIET_START must not read as "quiet hours start at midnight".
func getenvInt(key string, fallback int) int {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return n
}
