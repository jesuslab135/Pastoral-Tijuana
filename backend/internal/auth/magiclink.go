package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MagicLinkTTL is both the token lifetime and the window a spent jti is
// remembered for, so a replayed token can never outlive its own record.
const MagicLinkTTL = 15 * time.Minute

var (
	ErrTokenInvalid = errors.New("magic token invalid")
	ErrTokenExpired = errors.New("magic token expired")
)

// IssueMagicToken returns "<payload>.<mac>", where payload is
// base64url(userID|jti|expiresUnix). The token carries its own expiry; the
// jti is what the caller records to enforce single use.
func IssueMagicToken(secret string, userID uuid.UUID, now time.Time) (token, jti string, err error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	jti = base64.RawURLEncoding.EncodeToString(raw)
	payload := fmt.Sprintf("%s|%s|%d", userID, jti, now.Add(MagicLinkTTL).Unix())
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return encoded + "." + sign(secret, encoded), jti, nil
}

// ParseMagicToken verifies the signature and expiry, returning the user and
// jti. It never reports why a token failed beyond invalid/expired.
func ParseMagicToken(secret, token string, now time.Time) (uuid.UUID, string, error) {
	encoded, mac, ok := strings.Cut(token, ".")
	if !ok {
		return uuid.Nil, "", ErrTokenInvalid
	}
	if !hmac.Equal([]byte(mac), []byte(sign(secret, encoded))) {
		return uuid.Nil, "", ErrTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return uuid.Nil, "", ErrTokenInvalid
	}
	parts := strings.Split(string(payload), "|")
	if len(parts) != 3 {
		return uuid.Nil, "", ErrTokenInvalid
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, "", ErrTokenInvalid
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return uuid.Nil, "", ErrTokenInvalid
	}
	if now.After(time.Unix(exp, 0)) {
		return uuid.Nil, "", ErrTokenExpired
	}
	return id, parts[1], nil
}

func sign(secret, encoded string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(encoded))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
