// Package context provides content-addressed context management for BI-Chat.
//
// This package is a Go port of @diyor28/context with improvements for BI use cases.
// It implements content-addressed blocks with stable hashing, provider-agnostic
// rendering, and token budget enforcement.
package context

import (
	"time"
)

// BlockKind determines the ordering of blocks in compiled context.
// Blocks are always ordered by kind first, then by hash for determinism.
type BlockKind string

const (
	// KindPinned represents system rules and instructions (always first).
	KindPinned BlockKind = "pinned"

	// KindReference represents tool schemas, external documentation, etc.
	KindReference BlockKind = "reference"

	// KindMemory represents long-term memory, RAG results, etc.
	KindMemory BlockKind = "memory"

	// KindState represents current workflow/session state.
	KindState BlockKind = "state"

	// KindToolOutput represents tool execution results.
	KindToolOutput BlockKind = "tool_output"

	// KindHistory represents conversation history.
	KindHistory BlockKind = "history"

	// KindTurn represents the current user message (always last).
	KindTurn BlockKind = "turn"
)

// KindOrder defines the deterministic ordering of block kinds in compiled context.
var KindOrder = []BlockKind{
	KindPinned,
	KindReference,
	KindMemory,
	KindState,
	KindToolOutput,
	KindHistory,
	KindTurn,
}

// SensitivityLevel represents the sensitivity of block content for filtering.
type SensitivityLevel string

const (
	// SensitivityPublic indicates content safe to fork to any model.
	SensitivityPublic SensitivityLevel = "public"

	// SensitivityInternal indicates content containing business logic/PII.
	SensitivityInternal SensitivityLevel = "internal"

	// SensitivityRestricted indicates content containing credentials/secrets.
	SensitivityRestricted SensitivityLevel = "restricted"
)

// BlockMeta describes the metadata for a context block.
type BlockMeta struct {
	// Kind determines ordering in compiled context.
	Kind BlockKind

	// Sensitivity controls content filtering and forking.
	Sensitivity SensitivityLevel

	// CodecID identifies the codec used to render this block.
	CodecID string

	// CodecVersion is the version of the codec.
	CodecVersion string

	// Timestamp is when the block was created.
	Timestamp time.Time

	// Source is an optional identifier (workflow ID, session ID, etc.).
	Source string

	// Tags are optional labels for filtering.
	Tags []string
}

// StableMetaSubset represents the subset of BlockMeta used for stable hashing.
// Excludes volatile fields like Timestamp, Source, and Tags.
type StableMetaSubset struct {
	Kind         BlockKind
	Sensitivity  SensitivityLevel
	CodecID      string
	CodecVersion string
}

// ContextBlock is a content-addressed block with metadata and payload.
type ContextBlock struct {
	// Hash is the content-addressed hash (SHA-256 of meta + payload).
	Hash string

	// Meta contains block metadata.
	Meta BlockMeta

	// Payload is the codec-specific content.
	Payload any
}

// BlockRef is a reference to a block by hash only.
type BlockRef struct {
	Hash string
}

// ToStableSubset extracts the stable subset of metadata for hashing.
func (m BlockMeta) ToStableSubset() StableMetaSubset {
	return StableMetaSubset{
		Kind:         m.Kind,
		Sensitivity:  m.Sensitivity,
		CodecID:      m.CodecID,
		CodecVersion: m.CodecVersion,
	}
}
