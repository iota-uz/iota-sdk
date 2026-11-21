package connectors

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

var keyIDRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

type keyring struct {
	active string
	keys   map[string][]byte
}

// NewKeyring is a keyring of 32-byte keys by id; active must be one of them.
func NewKeyring(active string, keys map[string][]byte) (Keyring, error) {
	if _, ok := keys[active]; !ok {
		return nil, fmt.Errorf("automation connections: active key %q is not in the keyring", active)
	}
	k := &keyring{active: active, keys: make(map[string][]byte, len(keys))}
	for id, key := range keys {
		if !keyIDRe.MatchString(id) {
			return nil, fmt.Errorf("automation connections: key id %q: letters, digits, _ . - (≤ 64)", id)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("automation connections: key %q is %d bytes, AES-256 needs 32", id, len(key))
		}
		k.keys[id] = append([]byte(nil), key...)
	}
	return k, nil
}

// ParseKeyring reads a keyring from configuration: "id:key[,id:key...]", each key 32 bytes in base64
// (standard or URL, padded or not) or 64 hex digits; the first key is active. An empty spec is no
// keyring (nil, nil): the store then reports ErrNotConfigured.
func ParseKeyring(spec string) (Keyring, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	keys := map[string][]byte{}
	active := ""
	for i, part := range strings.Split(spec, ",") {
		id, raw, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok || id == "" || raw == "" {
			return nil, fmt.Errorf("automation connections: keyring entry %d is not id:key", i+1)
		}
		if _, dup := keys[id]; dup {
			return nil, fmt.Errorf("automation connections: key id %q is given twice", id)
		}
		key, err := decodeKey(raw)
		if err != nil {
			return nil, fmt.Errorf("automation connections: key %q: %w", id, err)
		}
		keys[id] = key
		if i == 0 {
			active = id
		}
	}
	return NewKeyring(active, keys)
}

func decodeKey(raw string) ([]byte, error) {
	if len(raw) == 64 {
		if b, err := hex.DecodeString(raw); err == nil {
			return b, nil
		}
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(raw); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	return nil, fmt.Errorf("not 32 bytes of base64 or hex")
}

func (k *keyring) Active() (string, []byte) { return k.active, k.keys[k.active] }

func (k *keyring) Key(id string) ([]byte, bool) {
	key, ok := k.keys[id]
	return key, ok
}

// aad binds a sealed secret to its row.
func aad(tenantID, id uuid.UUID, kind catalog.ConnectorKind) []byte {
	return []byte(tenantID.String() + "|" + id.String() + "|" + string(kind))
}

// seal encrypts with a fresh 96-bit nonce.
func seal(key, plaintext, additional []byte) (nonce, ciphertext []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("automation connections: nonce: %w", err)
	}
	return nonce, gcm.Seal(nil, nonce, plaintext, additional), nil
}

func open(key, nonce, ciphertext, additional []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, ErrUndecryptable
	}
	out, err := gcm.Open(nil, nonce, ciphertext, additional)
	if err != nil {
		return nil, ErrUndecryptable
	}
	return out, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("automation connections: cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
