package handlers

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresSecretsStore struct {
	pool      *pgxpool.Pool
	masterKey []byte
}

func NewPostgresSecretsStore(pool *pgxpool.Pool, masterKey string) (*PostgresSecretsStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	key, err := decodeMasterKey(masterKey)
	if err != nil {
		return nil, err
	}
	return &PostgresSecretsStore{
		pool:      pool,
		masterKey: key,
	}, nil
}

func (s *PostgresSecretsStore) Get(ctx context.Context, appletName, name string) (string, bool, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT cipher_text
		FROM applet_engine_secrets
		WHERE applet_id = $1 AND secret_name = $2
	`, appletName, name)
	var cipherText string
	if err := row.Scan(&cipherText); err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("postgres secrets.get: %w", err)
	}
	plain, err := decryptString(s.masterKey, cipherText)
	if err != nil {
		return "", false, fmt.Errorf("decrypt secret: %w", err)
	}
	return plain, true, nil
}

func EncryptSecretValue(masterKey string, plaintext string) (string, error) {
	key, err := decodeMasterKey(masterKey)
	if err != nil {
		return "", err
	}
	return encryptString(key, plaintext)
}

func decodeMasterKey(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("secrets master key is required")
	}
	key, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode secrets master key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("secrets master key must decode to 32 bytes (got %d)", len(key))
	}
	return key, nil
}

func encryptString(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

func decryptString(key []byte, encoded string) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode cipher payload: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return "", fmt.Errorf("cipher payload too short")
	}
	nonce, ciphertext := payload[:nonceSize], payload[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt payload: %w", err)
	}
	return string(plaintext), nil
}
