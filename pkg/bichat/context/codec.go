package context

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Codec handles payload validation, canonicalization, and hashing.
// It does NOT handle rendering - that is delegated to Renderer interface.
type Codec interface {
	// ID returns the unique codec identifier.
	ID() string

	// Version returns the codec version (semver).
	Version() string

	// Validate validates the payload and returns an error if invalid.
	Validate(payload any) error

	// Canonicalize converts the payload to a canonical form for deterministic hashing.
	// Must produce identical output for semantically equivalent inputs.
	Canonicalize(payload any) ([]byte, error)
}

// BaseCodec provides default implementations for common codec operations.
// Custom codecs can embed this to reduce boilerplate.
type BaseCodec struct {
	id      string
	version string
}

// NewBaseCodec creates a new BaseCodec with the given ID and version.
func NewBaseCodec(id, version string) *BaseCodec {
	return &BaseCodec{
		id:      id,
		version: version,
	}
}

// ID returns the codec identifier.
func (c *BaseCodec) ID() string {
	return c.id
}

// Version returns the codec version.
func (c *BaseCodec) Version() string {
	return c.version
}

// ComputeBlockHash computes the SHA-256 hash of a block's stable metadata and payload.
func ComputeBlockHash(meta StableMetaSubset, canonicalized []byte) string {
	h := sha256.New()

	// Hash metadata (deterministic order)
	h.Write([]byte(meta.Kind))
	h.Write([]byte(meta.Sensitivity))
	h.Write([]byte(meta.CodecID))
	h.Write([]byte(meta.CodecVersion))

	// Hash canonicalized payload
	h.Write(canonicalized)

	return fmt.Sprintf("%x", h.Sum(nil))
}

// SortedJSONBytes returns deterministically sorted JSON bytes for hashing.
// This is a helper for codecs that use JSON canonicalization.
func SortedJSONBytes(v any) ([]byte, error) {
	// Marshal to JSON
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	// Unmarshal and re-marshal to ensure key ordering
	var temp any
	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Re-marshal with sorted keys (Go's json package sorts keys by default)
	sorted, err := json.Marshal(temp)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal JSON: %w", err)
	}

	return sorted, nil
}
