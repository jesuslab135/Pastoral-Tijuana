package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "")
	t.Setenv("PARISH_TZ", "")
	t.Setenv("PUBLIC_BASE_URL", "")
	c := Load()
	if c.DatabaseURL != "postgres://pastoral:pastoral@localhost:5433/pastoral?sslmode=disable" {
		t.Errorf("DatabaseURL default wrong: %q", c.DatabaseURL)
	}
	if c.Port != "8080" {
		t.Errorf("Port default wrong: %q", c.Port)
	}
	if c.ParishTZ != "America/Mexico_City" {
		t.Errorf("ParishTZ default wrong: %q", c.ParishTZ)
	}
	if c.PublicBaseURL != "http://localhost:8080" {
		t.Errorf("PublicBaseURL default wrong: %q", c.PublicBaseURL)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	c := Load()
	if c.Port != "9999" {
		t.Errorf("Port should come from env, got %q", c.Port)
	}
}
