package webpush

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Constants of the aes128gcm content coding (RFC 8188) as profiled by
// RFC 8291 for Web Push.
const (
	recordSize = 4096
	saltLength = 16
	// headerLength is salt (16) + record size (4) + key id length (1) + the
	// 65-byte uncompressed sender key that serves as the key id.
	headerLength = saltLength + 4 + 1 + 65
	// MaxPayload is the largest plaintext one push message can carry: one
	// 4096-byte record minus the header, the AEAD tag and the delimiter byte.
	MaxPayload = recordSize - headerLength - 16 - 1
)

var (
	errPayloadTooLarge = fmt.Errorf("push payload exceeds %d bytes", MaxPayload)
	errBadUserKey      = errors.New("invalid push subscription public key")
	errBadAuthSecret   = errors.New("invalid push subscription auth secret")
)

// encrypt implements RFC 8291 section 3-4 for a single record: it encrypts
// plaintext for the user agent identified by its P-256 public key and 16-byte
// authentication secret and returns the complete request body (RFC 8188
// header followed by the ciphertext). The sender key pair and the salt are
// parameters so the RFC's test vector can pin the output.
func encrypt(plaintext []byte, uaPublic *ecdh.PublicKey, authSecret []byte, sender *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	if len(plaintext) > MaxPayload {
		return nil, errPayloadTooLarge
	}
	if uaPublic == nil || uaPublic.Curve() != ecdh.P256() {
		return nil, errBadUserKey
	}
	if len(authSecret) != 16 {
		return nil, errBadAuthSecret
	}
	if len(salt) != saltLength {
		return nil, errors.New("invalid salt")
	}
	secret, err := sender.ECDH(uaPublic)
	if err != nil {
		return nil, fmt.Errorf("ecdh: %w", err)
	}
	senderPublic := sender.PublicKey().Bytes()

	// PRK_key = HMAC(auth_secret, ecdh_secret); IKM = HKDF-Expand(PRK_key, key_info || 0x01, 32)
	prkKey, err := hkdf.Extract(sha256.New, secret, authSecret)
	if err != nil {
		return nil, err
	}
	keyInfo := append(append([]byte("WebPush: info\x00"), uaPublic.Bytes()...), senderPublic...)
	ikm, err := hkdf.Expand(sha256.New, prkKey, string(keyInfo), 32)
	if err != nil {
		return nil, err
	}
	// The content-encryption key and nonce derive from IKM and the salt (RFC 8188 section 2.2-2.3).
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// A single, final record: the plaintext followed by the 0x02 delimiter.
	record := append(append(make([]byte, 0, len(plaintext)+1), plaintext...), 0x02)

	body := make([]byte, 0, headerLength+len(record)+aead.Overhead())
	body = append(body, salt...)
	body = binary.BigEndian.AppendUint32(body, recordSize)
	body = append(body, byte(len(senderPublic)))
	body = append(body, senderPublic...)
	return aead.Seal(body, nonce, record, nil), nil
}

// Encrypt encrypts plaintext for a subscription with a fresh sender key pair
// and a random salt.
func Encrypt(plaintext []byte, sub Subscription) ([]byte, error) {
	uaPublic, authSecret, err := sub.decodeKeys()
	if err != nil {
		return nil, err
	}
	sender, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return encrypt(plaintext, uaPublic, authSecret, sender, salt)
}
