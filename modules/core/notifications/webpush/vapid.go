package webpush

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// vapidLifetime is how long one signed VAPID token is valid. RFC 8292 caps it
// at 24 hours; half a day leaves room for clock skew.
const vapidLifetime = 12 * time.Hour

// KeyPair is a VAPID application server key pair (RFC 8292) in the encodings
// browsers and web push tooling use: base64url without padding, the public key
// as the 65-byte uncompressed P-256 point (the "applicationServerKey" a page
// passes to pushManager.subscribe) and the private key as the 32-byte scalar.
type KeyPair struct {
	Public  string
	Private string
}

// GenerateKeys creates a VAPID key pair. Run it once per deployment and keep
// the private key in the secret store; never commit it.
func GenerateKeys() (KeyPair, error) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{
		Public:  base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		Private: base64.RawURLEncoding.EncodeToString(key.Bytes()),
	}, nil
}

// vapidSigner signs VAPID authorization tokens.
type vapidSigner struct {
	key     *ecdsa.PrivateKey
	public  string
	subject string
}

// newVAPIDSigner parses the configured keys and verifies that they belong
// together, so a swapped or truncated key fails at startup, not at the first send.
func newVAPIDSigner(public, private, subject string) (*vapidSigner, error) {
	raw, err := decodeBase64URL(private)
	if err != nil {
		return nil, errors.New("webpush: the VAPID private key is not base64url")
	}
	priv, err := ecdh.P256().NewPrivateKey(raw)
	if err != nil {
		return nil, errors.New("webpush: the VAPID private key is not a P-256 scalar")
	}
	derived := priv.PublicKey().Bytes()
	configured, err := decodeBase64URL(public)
	if err != nil || string(configured) != string(derived) {
		return nil, errors.New("webpush: the VAPID public key does not match the private key")
	}
	if err := validateSubject(subject); err != nil {
		return nil, err
	}
	// ecdsa.PrivateKey is assembled from the validated ECDH key: the public
	// point is the 0x04 || X || Y encoding checked above.
	key := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(derived[1:33]), Y: new(big.Int).SetBytes(derived[33:])},
		D:         new(big.Int).SetBytes(raw),
	}
	return &vapidSigner{key: key, public: base64.RawURLEncoding.EncodeToString(derived), subject: subject}, nil
}

func validateSubject(subject string) error {
	u, err := url.Parse(subject)
	if err != nil || (u.Scheme != "mailto" && u.Scheme != "https") || (u.Scheme == "mailto" && u.Opaque == "") || (u.Scheme == "https" && u.Host == "") {
		return errors.New("webpush: the VAPID subject must be a mailto: or https: URL the push service can contact")
	}
	return nil
}

// authorization returns the Authorization header value for a push service
// origin: "vapid t=<jwt>, k=<public key>" (RFC 8292 section 3).
func (s *vapidSigner) authorization(audience string, now time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"aud": audience,
		"exp": now.Add(vapidLifetime).Unix(),
		"sub": s.subject,
	})
	signed, err := token.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("webpush: sign VAPID token: %w", err)
	}
	return "vapid t=" + signed + ", k=" + s.public, nil
}

func decodeBase64URL(s string) ([]byte, error) {
	// Browsers emit unpadded base64url; tolerate padding and the standard alphabet.
	s = strings.TrimRight(strings.TrimSpace(s), "=")
	s = strings.NewReplacer("+", "-", "/", "_").Replace(s)
	return base64.RawURLEncoding.DecodeString(s)
}
