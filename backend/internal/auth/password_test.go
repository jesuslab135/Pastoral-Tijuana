package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("contraseña-segura")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$") {
		t.Errorf("not a PHC argon2id string: %q", h)
	}
	if !VerifyPassword("contraseña-segura", h) {
		t.Error("correct password must verify")
	}
	if VerifyPassword("otra-cosa", h) {
		t.Error("wrong password must not verify")
	}
}

func TestHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("x")
	b, _ := HashPassword("x")
	if a == b {
		t.Error("two hashes of the same password must differ (random salt)")
	}
}

func TestVerifyRejectsGarbage(t *testing.T) {
	for _, bad := range []string{
		"",
		"plaintext",
		"$argon2id$v=19$m=65536,t=1,p=4$!!$??",
		"$argon2i$v=19$m=1,t=1,p=1$AA$AA",
	} {
		if VerifyPassword("x", bad) {
			t.Errorf("VerifyPassword must reject %q", bad)
		}
	}
}
