package webpush

import (
	"crypto/ecdh"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Subscription is a browser PushSubscription as PushSubscription.toJSON()
// returns it. Its JSON form is what is stored as the device token.
type Subscription struct {
	Endpoint string           `json:"endpoint"`
	Keys     SubscriptionKeys `json:"keys"`
}

// SubscriptionKeys are the user agent's key material, base64url encoded.
type SubscriptionKeys struct {
	P256DH string `json:"p256dh"`
	Auth   string `json:"auth"`
}

const maxEndpointLength = 2048

// ParseToken reads a device token written by Token.
func ParseToken(token string) (Subscription, error) {
	var sub Subscription
	if err := json.Unmarshal([]byte(token), &sub); err != nil {
		return Subscription{}, errors.New("push token is not a web push subscription")
	}
	return sub, nil
}

// Token is the canonical device token of a subscription. Only the fields the
// sender needs are kept (an expiration time or other browser extras are not).
func (s Subscription) Token() (string, error) {
	b, err := json.Marshal(Subscription{Endpoint: s.Endpoint, Keys: s.Keys})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// validate checks the shape of the subscription: an https endpoint URL without
// credentials, a valid P-256 key and a 16-byte auth secret.
func (s Subscription) validate(allowHTTP bool) (*url.URL, error) {
	if s.Endpoint == "" || len(s.Endpoint) > maxEndpointLength || !utf8.ValidString(s.Endpoint) || strings.ContainsFunc(s.Endpoint, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return nil, errors.New("invalid push subscription endpoint")
	}
	u, err := url.Parse(s.Endpoint)
	if err != nil || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, errors.New("invalid push subscription endpoint")
	}
	if u.Scheme != "https" && (!allowHTTP || u.Scheme != "http") {
		return nil, errors.New("push subscription endpoint must be https")
	}
	if _, _, err := s.decodeKeys(); err != nil {
		return nil, err
	}
	return u, nil
}

func (s Subscription) decodeKeys() (*ecdh.PublicKey, []byte, error) {
	rawKey, err := decodeBase64URL(s.Keys.P256DH)
	if err != nil {
		return nil, nil, errBadUserKey
	}
	public, err := ecdh.P256().NewPublicKey(rawKey)
	if err != nil {
		return nil, nil, errBadUserKey
	}
	auth, err := decodeBase64URL(s.Keys.Auth)
	if err != nil || len(auth) != 16 {
		return nil, nil, errBadAuthSecret
	}
	return public, auth, nil
}

// DeviceID derives a stable device identifier from a subscription's endpoint,
// so registering the same browser subscription again refreshes its row instead
// of creating a second device.
func DeviceID(sub Subscription) string {
	return fmt.Sprintf("web:%x", hashPrefix(sub.Endpoint))
}
