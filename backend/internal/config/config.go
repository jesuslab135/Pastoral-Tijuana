// Package config reads runtime configuration from environment variables.
package config

import "os"

type Config struct {
	DatabaseURL   string
	Port          string
	ParishTZ      string
	PublicBaseURL string
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
