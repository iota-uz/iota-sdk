package context

import (
	"fmt"
	"time"
)

// ContextBuilder provides a fluent API for building context graphs.
//
// Usage:
//
//	builder := NewBuilder()
//	builder.
//	    System(codec, systemRules).
//	    Reference(schemaCodec, schemas).
//	    History(historyCodec, messages).
//	    Turn(turnCodec, userMessage)
//
//	compiled, err := builder.Compile(renderer, policy)
type ContextBuilder struct {
	graph        *ContextGraph
	blockCounter int
}

// NewBuilder creates a new context builder.
func NewBuilder() *ContextBuilder {
	return &ContextBuilder{
		graph:        NewContextGraph(),
		blockCounter: 0,
	}
}

// BlockOptions contains optional metadata for blocks.
type BlockOptions struct {
	Sensitivity SensitivityLevel
	Source      string
	Tags        []string
}

// System adds a pinned system rules block.
func (b *ContextBuilder) System(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	return b.add(KindPinned, codec, payload, opts...)
}

// Reference adds a reference block (tool schema, external doc, etc.).
func (b *ContextBuilder) Reference(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	return b.add(KindReference, codec, payload, opts...)
}

// Memory adds a memory block (long-term memory, RAG results).
func (b *ContextBuilder) Memory(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	return b.add(KindMemory, codec, payload, opts...)
}

// State adds a state block (current workflow/session state).
func (b *ContextBuilder) State(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	return b.add(KindState, codec, payload, opts...)
}

// ToolOutput adds a tool output block.
func (b *ContextBuilder) ToolOutput(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	return b.add(KindToolOutput, codec, payload, opts...)
}

// History adds a conversation history block.
func (b *ContextBuilder) History(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	return b.add(KindHistory, codec, payload, opts...)
}

// Turn adds a user turn block (current user message).
func (b *ContextBuilder) Turn(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	return b.add(KindTurn, codec, payload, opts...)
}

// Add adds a block with early validation.
func (b *ContextBuilder) Add(kind BlockKind, codec Codec, payload any, opts ...BlockOptions) error {
	if err := codec.Validate(payload); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	b.add(kind, codec, payload, opts...)
	return nil
}

// MustSystem adds a system block and panics on validation error.
func (b *ContextBuilder) MustSystem(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	if err := b.Add(KindPinned, codec, payload, opts...); err != nil {
		panic(fmt.Sprintf("MustSystem failed: %v", err))
	}
	return b
}

// MustReference adds a reference block and panics on validation error.
func (b *ContextBuilder) MustReference(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	if err := b.Add(KindReference, codec, payload, opts...); err != nil {
		panic(fmt.Sprintf("MustReference failed: %v", err))
	}
	return b
}

// MustMemory adds a memory block and panics on validation error.
func (b *ContextBuilder) MustMemory(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	if err := b.Add(KindMemory, codec, payload, opts...); err != nil {
		panic(fmt.Sprintf("MustMemory failed: %v", err))
	}
	return b
}

// MustState adds a state block and panics on validation error.
func (b *ContextBuilder) MustState(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	if err := b.Add(KindState, codec, payload, opts...); err != nil {
		panic(fmt.Sprintf("MustState failed: %v", err))
	}
	return b
}

// MustToolOutput adds a tool output block and panics on validation error.
func (b *ContextBuilder) MustToolOutput(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	if err := b.Add(KindToolOutput, codec, payload, opts...); err != nil {
		panic(fmt.Sprintf("MustToolOutput failed: %v", err))
	}
	return b
}

// MustHistory adds a history block and panics on validation error.
func (b *ContextBuilder) MustHistory(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	if err := b.Add(KindHistory, codec, payload, opts...); err != nil {
		panic(fmt.Sprintf("MustHistory failed: %v", err))
	}
	return b
}

// MustTurn adds a turn block and panics on validation error.
func (b *ContextBuilder) MustTurn(codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	if err := b.Add(KindTurn, codec, payload, opts...); err != nil {
		panic(fmt.Sprintf("MustTurn failed: %v", err))
	}
	return b
}

// GetGraph returns the underlying context graph.
func (b *ContextBuilder) GetGraph() *ContextGraph {
	return b.graph
}

// GetBlockCount returns the number of blocks in the builder.
func (b *ContextBuilder) GetBlockCount() int {
	return b.graph.GetBlockCount()
}

// Clear removes all blocks from the builder.
func (b *ContextBuilder) Clear() {
	b.graph.Clear()
	b.blockCounter = 0
}

// add is the internal method for adding blocks.
func (b *ContextBuilder) add(kind BlockKind, codec Codec, payload any, opts ...BlockOptions) *ContextBuilder {
	// Validate payload (panic on error for fluent API)
	if err := codec.Validate(payload); err != nil {
		panic(fmt.Sprintf("validation failed for %s block: %v", kind, err))
	}

	// Canonicalize and compute hash
	canonicalized, err := codec.Canonicalize(payload)
	if err != nil {
		panic(fmt.Sprintf("canonicalization failed for %s block: %v", kind, err))
	}

	// Extract options
	var options BlockOptions
	if len(opts) > 0 {
		options = opts[0]
	}
	if options.Sensitivity == "" {
		options.Sensitivity = SensitivityPublic
	}

	// Build metadata
	meta := BlockMeta{
		Kind:         kind,
		Sensitivity:  options.Sensitivity,
		CodecID:      codec.ID(),
		CodecVersion: codec.Version(),
		Timestamp:    time.Now(),
		Source:       options.Source,
		Tags:         options.Tags,
	}

	// Compute hash
	hash := ComputeBlockHash(meta.ToStableSubset(), canonicalized)

	// Create block
	block := ContextBlock{
		Hash:    hash,
		Meta:    meta,
		Payload: payload,
	}

	// Add to graph
	b.graph.AddBlock(block, nil, nil)
	b.blockCounter++

	return b
}
