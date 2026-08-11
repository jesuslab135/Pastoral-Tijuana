package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const secret = "un-secreto-de-prueba"

func TestMagicTokenRoundTrip(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	tok, jti, err := IssueMagicToken(secret, id, now)
	if err != nil {
		t.Fatalf("IssueMagicToken: %v", err)
	}
	gotID, gotJTI, err := ParseMagicToken(secret, tok, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ParseMagicToken: %v", err)
	}
	if gotID != id || gotJTI != jti {
		t.Errorf("round trip mismatch: %s/%s vs %s/%s", gotID, gotJTI, id, jti)
	}
}

func TestMagicTokenExpires(t *testing.T) {
	now := time.Now()
	tok, _, _ := IssueMagicToken(secret, uuid.New(), now)
	_, _, err := ParseMagicToken(secret, tok, now.Add(MagicLinkTTL+time.Minute))
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestMagicTokenRejectsWrongSecret(t *testing.T) {
	now := time.Now()
	tok, _, _ := IssueMagicToken(secret, uuid.New(), now)
	if _, _, err := ParseMagicToken("otro-secreto", tok, now); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestMagicTokenRejectsTamperedPayload(t *testing.T) {
	now := time.Now()
	tok, _, _ := IssueMagicToken(secret, uuid.New(), now)
	encoded, mac, _ := strings.Cut(tok, ".")
	// Flip a byte of the payload while keeping the original signature.
	tampered := "A" + encoded[1:] + "." + mac
	if _, _, err := ParseMagicToken(secret, tampered, now); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestMagicTokenRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "sinpunto", ".", "a.b"} {
		if _, _, err := ParseMagicToken(secret, bad, time.Now()); err == nil {
			t.Errorf("must reject %q", bad)
		}
	}
}

func TestMagicTokenJTIsAreUnique(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	_, a, _ := IssueMagicToken(secret, id, now)
	_, b, _ := IssueMagicToken(secret, id, now)
	if a == b {
		t.Error("each issued token needs its own jti or single-use breaks")
	}
}
