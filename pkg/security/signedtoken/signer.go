// Package signedtoken provides a generic HMAC-SHA256-signed, expiring,
// versioned token for passing trusted payloads through URLs.
//
// Wire format: v1.<base64url(payload-json)>.<base64url(hmac-sha256)>
// The HMAC covers "v1.<payload-segment>" (the prefix up to but not including
// the second dot) so payload and version are both authenticated.
//
// Tokens carry an absolute expiry (UnixSeconds). Verify rejects expired or
// tampered tokens with ErrExpired / ErrInvalid.
package signedtoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const tokenVersion = "v1"

type Signer[T any] interface {
	Sign(payload T, ttl time.Duration) (string, error)
	Verify(token string) (T, error)
}

type envelope[T any] struct {
	Payload   T     `json:"p"`
	ExpiresAt int64 `json:"exp"`
	IssuedAt  int64 `json:"iat"`
}

type hmacSigner[T any] struct {
	current  []byte
	previous []byte
	now      func() time.Time
}

// NewHMAC returns a Signer that signs with current and verifies against
// current OR previous (for zero-downtime secret rotation; pass nil if unused).
func NewHMAC[T any](current, previous []byte) Signer[T] {
	return &hmacSigner[T]{current: current, previous: previous, now: time.Now}
}

func (s *hmacSigner[T]) Sign(payload T, ttl time.Duration) (string, error) {
	now := s.now()
	env := envelope[T]{
		Payload:   payload,
		ExpiresAt: now.Add(ttl).Unix(),
		IssuedAt:  now.Unix(),
	}
	body, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("signedtoken: marshal: %w", err)
	}
	payloadSeg := base64.RawURLEncoding.EncodeToString(body)
	signedPrefix := tokenVersion + "." + payloadSeg
	mac := hmacSum(s.current, signedPrefix)
	return signedPrefix + "." + base64.RawURLEncoding.EncodeToString(mac), nil
}

func (s *hmacSigner[T]) Verify(token string) (T, error) {
	var zero T
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != tokenVersion {
		return zero, ErrInvalid
	}
	signedPrefix := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return zero, ErrInvalid
	}
	if !verifyAny(signedPrefix, sig, s.current, s.previous) {
		return zero, ErrInvalid
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return zero, ErrInvalid
	}
	var env envelope[T]
	if err := json.Unmarshal(body, &env); err != nil {
		return zero, ErrInvalid
	}
	if env.ExpiresAt > 0 && s.now().Unix() > env.ExpiresAt {
		return zero, ErrExpired
	}
	return env.Payload, nil
}

func hmacSum(key []byte, msg string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(msg))
	return h.Sum(nil)
}

func verifyAny(msg string, sig, current, previous []byte) bool {
	if len(current) > 0 && hmac.Equal(sig, hmacSum(current, msg)) {
		return true
	}
	if len(previous) > 0 && hmac.Equal(sig, hmacSum(previous, msg)) {
		return true
	}
	return false
}
