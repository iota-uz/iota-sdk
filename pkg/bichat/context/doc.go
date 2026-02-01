// Package context provides content-addressed context management for BI-Chat agents.
//
// This package is a Go port of @diyor28/context with improvements for BI use cases.
// It implements:
//   - Content-addressed blocks with SHA-256 hashing
//   - Immutable context graphs
//   - Provider-agnostic rendering (Anthropic, OpenAI, Gemini)
//   - Token budget enforcement with overflow handling
//   - Early validation in builder
//   - Query DSL for block filtering
//
// # Basic Usage
//
// Build a context using the fluent Builder API:
//
//	builder := context.NewBuilder()
//	builder.
//	    System(systemCodec, systemRules).
//	    Reference(schemaCodec, dbSchemas).
//	    Memory(ragCodec, kbResults).
//	    History(historyCodec, messages).
//	    Turn(turnCodec, userMessage)
//
// Compile to provider-specific format with token budgeting:
//
//	renderer := renderers.NewAnthropicRenderer()
//	policy := context.DefaultPolicy()
//	compiled, err := builder.Compile(renderer, policy)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Use the compiled context with your LLM provider:
//
//	response, err := llm.Generate(compiled.SystemPrompt, compiled.Messages)
//
// # Block Kinds
//
// Blocks are ordered by kind in compiled context:
//   - KindPinned: System rules (always first)
//   - KindReference: Tool schemas, documentation
//   - KindMemory: Long-term memory, RAG results
//   - KindState: Current workflow/session state
//   - KindToolOutput: Tool execution results
//   - KindHistory: Conversation history
//   - KindTurn: Current user message (always last)
//
// # Content-Addressed Blocks
//
// Each block is hashed based on its stable metadata and canonicalized payload.
// This enables:
//   - Deduplication: Same content produces same hash
//   - Deterministic ordering: Blocks sorted by kind then hash
//   - Efficient caching: Hash as cache key
//
// # Provider Renderers
//
// The package provides three built-in renderers:
//   - AnthropicRenderer: For Claude models (Anthropic, Bedrock, Vertex AI)
//   - OpenAIRenderer: For GPT models (OpenAI, Azure OpenAI)
//   - GeminiRenderer: For Gemini models (Google AI)
//
// Custom renderers can be implemented for other providers.
//
// # Token Budgeting
//
// The ContextPolicy configures token limits and overflow handling:
//
//	policy := context.ContextPolicy{
//	    ContextWindow:     180000, // Claude 3.5 Sonnet
//	    CompletionReserve: 8000,
//	    OverflowStrategy:  context.OverflowTruncate,
//	    KindPriorities:    context.DefaultKindPriorities(),
//	}
//
// When compiled context exceeds budget:
//   - OverflowError: Returns error
//   - OverflowTruncate: Removes truncatable blocks from end
//   - OverflowCompact: Runs compaction (summarize history, prune old tool outputs)
//
// # Custom Codecs
//
// Implement the Codec interface for custom block types:
//
//	type MyCodec struct {
//	    *context.BaseCodec
//	}
//
//	func (c *MyCodec) Validate(payload any) error {
//	    // Validate payload structure
//	}
//
//	func (c *MyCodec) Canonicalize(payload any) ([]byte, error) {
//	    // Convert to canonical form for hashing
//	}
//
// # Query DSL
//
// Filter blocks using the query DSL:
//
//	blocks := builder.GetGraph().Select(
//	    context.Kind(context.KindHistory).
//	        And(context.Sensitivity(context.SensitivityPublic)).
//	        And(context.HasTag("recent")),
//	)
//
// # BI-Specific Codecs
//
// The codecs package provides BI-specific block types:
//   - DatabaseSchemaCodec: Database table schemas
//   - QueryResultCodec: SQL query results (with auto-truncation)
//   - ChartDataCodec: Chart data for visualization
//   - KBSearchResultsCodec: Knowledge base search results
//
// Example:
//
//	schemaCodec := codecs.NewDatabaseSchemaCodec()
//	builder.Reference(schemaCodec, DatabaseSchemaPayload{
//	    SchemaName: "public",
//	    Tables: []TableSchema{...},
//	})
//
// # Thread Safety
//
// ContextGraph is thread-safe for concurrent reads and writes.
// ContextBuilder is not thread-safe and should be used from a single goroutine.
package context
