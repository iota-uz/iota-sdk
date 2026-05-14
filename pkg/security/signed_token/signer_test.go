package signed_token

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type sample struct {
	UserID string `json:"u"`
	Scope  string `json:"s"`
}

func newTestSigner(t *testing.T) *hmacSigner[sample] {
	t.Helper()
	s := NewHMAC[sample]([]byte("secret-current-32-bytes-padding!!"), nil).(*hmacSigner[sample])
	return s
}

func TestSignVerifyRoundTrip(t *testing.T) {
	s := newTestSigner(t)
	tok, err := s.Sign(sample{UserID: "u1", Scope: "read"}, time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	got, err := s.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.UserID != "u1" || got.Scope != "read" {
		t.Fatalf("payload mismatch: %+v", got)
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	s := newTestSigner(t)
	tok, _ := s.Sign(sample{UserID: "u1"}, time.Minute)
	// Flip a byte in the signature segment.
	parts := strings.Split(tok, ".")
	parts[2] = parts[2][:len(parts[2])-1] + "A"
	bad := strings.Join(parts, ".")
	if _, err := s.Verify(bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	s := newTestSigner(t)
	tok, _ := s.Sign(sample{UserID: "u1"}, time.Minute)
	parts := strings.Split(tok, ".")
	parts[1] = parts[1][:len(parts[1])-1] + "A"
	bad := strings.Join(parts, ".")
	if _, err := s.Verify(bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func TestVerifyRejectsBadFormat(t *testing.T) {
	s := newTestSigner(t)
	for _, bad := range []string{"", "v1.only-two", "v2.x.y", "notbase64.!.??"} {
		if _, err := s.Verify(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("input %q expected ErrInvalid, got %v", bad, err)
		}
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	s := newTestSigner(t)
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	tok, _ := s.Sign(sample{UserID: "u1"}, time.Minute)
	s.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := s.Verify(tok); !errors.Is(err, ErrExpired) {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

func TestVerifyAcceptsPreviousSecretDuringRotation(t *testing.T) {
	old := []byte("secret-previous-32-bytes-pad!!!!!")
	cur := []byte("secret-current-32-bytes-padding!!")
	// Sign with old.
	oldSigner := NewHMAC[sample](old, nil).(*hmacSigner[sample])
	tok, _ := oldSigner.Sign(sample{UserID: "u1"}, time.Minute)
	// Verify with new signer that knows old as previous.
	newSigner := NewHMAC[sample](cur, old)
	if _, err := newSigner.Verify(tok); err != nil {
		t.Fatalf("expected previous-secret token to verify, got %v", err)
	}
	// New token signed with current also verifies.
	tok2, _ := newSigner.Sign(sample{UserID: "u2"}, time.Minute)
	if _, err := newSigner.Verify(tok2); err != nil {
		t.Fatalf("expected current-secret token to verify, got %v", err)
	}
}
