package credentials

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type BootstrapArtifact struct {
	TokenID   string    `json:"token_id"`
	Secret    string    `json:"secret,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	Subject   string    `json:"subject"`
}

func NewBootstrapArtifact(subject string, ttl time.Duration, includeSecret bool) (BootstrapArtifact, error) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	id, err := randomToken(16)
	if err != nil {
		return BootstrapArtifact{}, err
	}
	secret := ""
	if includeSecret {
		secret, err = randomToken(32)
		if err != nil {
			return BootstrapArtifact{}, err
		}
	}
	return BootstrapArtifact{
		TokenID:   "boot_" + id,
		Secret:    secret,
		ExpiresAt: time.Now().UTC().Add(ttl),
		Subject:   subject,
	}, nil
}

func (a BootstrapArtifact) JSON() (string, error) {
	payload, err := json.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("marshal bootstrap artifact: %w", err)
	}
	return string(payload), nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
