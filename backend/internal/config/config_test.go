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
	if c.ParishTZ != "America/Tijuana" {
		t.Errorf("ParishTZ default wrong: %q", c.ParishTZ)
	}
	if c.PublicBaseURL != "http://localhost:8080" {
		t.Errorf("PublicBaseURL default wrong: %q", c.PublicBaseURL)
	}
}

func TestLoadAuthDefaults(t *testing.T) {
	for _, k := range []string{"REDIS_ADDR", "TRUSTED_PROXY", "AUTH_SECRET",
		"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASS", "SMTP_FROM"} {
		t.Setenv(k, "")
	}
	c := Load()
	if c.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr default wrong: %q", c.RedisAddr)
	}
	if c.TrustedProxy != "" {
		t.Errorf("TrustedProxy must default empty, got %q", c.TrustedProxy)
	}
	if c.AuthSecret != "dev-secret-change-me" {
		t.Errorf("AuthSecret default wrong: %q", c.AuthSecret)
	}
	if c.SMTPHost != "" || c.SMTPPort != "587" {
		t.Errorf("SMTP defaults wrong: host=%q port=%q", c.SMTPHost, c.SMTPPort)
	}
}

func TestLoadDifusionDefaults(t *testing.T) {
	for _, k := range []string{"QUIET_START", "QUIET_END", "STAGGER_SECONDS"} {
		t.Setenv(k, "")
	}
	c := Load()
	if c.QuietStart != 22 || c.QuietEnd != 7 {
		t.Errorf("quiet hours default wrong: %d-%d", c.QuietStart, c.QuietEnd)
	}
	if c.StaggerSeconds != 8 {
		t.Errorf("StaggerSeconds default wrong: %d", c.StaggerSeconds)
	}

	t.Setenv("QUIET_START", "9")
	if c := Load(); c.QuietStart != 9 {
		t.Errorf("QUIET_START should come from env, got %d", c.QuietStart)
	}

	// A typo must not silently disable quiet hours by parsing as zero.
	t.Setenv("QUIET_START", "chido")
	if c := Load(); c.QuietStart != 22 {
		t.Errorf("non-numeric QUIET_START should fall back to 22, got %d", c.QuietStart)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	c := Load()
	if c.Port != "9999" {
		t.Errorf("Port should come from env, got %q", c.Port)
	}
}
