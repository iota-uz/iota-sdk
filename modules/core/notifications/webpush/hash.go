package webpush

import "crypto/sha256"

// hashPrefix is the first 16 bytes of SHA-256 of s.
func hashPrefix(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:16]
}
