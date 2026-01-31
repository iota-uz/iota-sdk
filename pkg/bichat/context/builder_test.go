package context_test

import (
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
)

func TestBuilder_FluentAPI(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	historyCodec := codecs.NewConversationHistoryCodec()
	schemaCodec := codecs.NewDatabaseSchemaCodec()

	// Test fluent chaining
	builder.
		System(systemCodec, "You are a helpful assistant").
		Reference(schemaCodec, codecs.DatabaseSchemaPayload{
			SchemaName: "public",
			Tables: []codecs.TableSchema{
				{
					Name: "users",
					Columns: []codecs.TableColumn{
						{Name: "id", Type: "integer", Nullable: false},
						{Name: "name", Type: "text", Nullable: true},
					},
				},
			},
		}).
		History(historyCodec, codecs.ConversationHistoryPayload{
			Messages: []codecs.ConversationMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi!"},
			},
		}).
		Turn(systemCodec, "What is my name?")

	// Verify block count
	if builder.GetBlockCount() != 4 {
		t.Errorf("Expected 4 blocks, got %d", builder.GetBlockCount())
	}

	// Verify blocks were added
	graph := builder.GetGraph()
	blocks := graph.GetAllBlocks()
	if len(blocks) != 4 {
		t.Errorf("Expected 4 blocks in graph, got %d", len(blocks))
	}
}

func TestBuilder_Validation(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()

	// Test Add with valid payload
	err := builder.Add(context.KindPinned, systemCodec, "Valid system prompt", context.BlockOptions{})
	if err != nil {
		t.Errorf("Expected no error for valid payload, got: %v", err)
	}

	// Test Add with invalid payload (empty string should fail validation)
	err = builder.Add(context.KindPinned, systemCodec, "", context.BlockOptions{})
	if err == nil {
		t.Error("Expected validation error for empty string, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Expected 'validation failed' in error, got: %v", err)
	}
}

func TestBuilder_MustAdd(t *testing.T) {
	t.Parallel()

	t.Run("MustSystem panics on invalid payload", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid payload")
			}
		}()

		builder := context.NewBuilder()
		systemCodec := codecs.NewSystemRulesCodec()

		// This should panic because empty string is invalid
		builder.MustSystem(systemCodec, "")
	})

	t.Run("MustReference succeeds with valid payload", func(t *testing.T) {
		builder := context.NewBuilder()
		schemaCodec := codecs.NewDatabaseSchemaCodec()

		// This should not panic
		builder.MustReference(schemaCodec, codecs.DatabaseSchemaPayload{
			SchemaName: "test",
			Tables: []codecs.TableSchema{
				{
					Name: "test_table",
					Columns: []codecs.TableColumn{
						{Name: "id", Type: "int", Nullable: false},
					},
				},
			},
		})

		if builder.GetBlockCount() != 1 {
			t.Errorf("Expected 1 block, got %d", builder.GetBlockCount())
		}
	})

	t.Run("MustHistory panics on invalid payload", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid history payload")
			}
		}()

		builder := context.NewBuilder()
		historyCodec := codecs.NewConversationHistoryCodec()

		// This should panic because empty messages array is invalid
		builder.MustHistory(historyCodec, codecs.ConversationHistoryPayload{
			Messages: []codecs.ConversationMessage{},
		})
	})
}

func TestBuilder_BlockOrdering(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	historyCodec := codecs.NewConversationHistoryCodec()
	schemaCodec := codecs.NewDatabaseSchemaCodec()

	// Add blocks in non-standard order
	builder.
		Turn(systemCodec, "Current message").
		System(systemCodec, "System rules").
		History(historyCodec, codecs.ConversationHistoryPayload{
			Messages: []codecs.ConversationMessage{
				{Role: "user", Content: "Hi"},
			},
		}).
		Reference(schemaCodec, codecs.DatabaseSchemaPayload{
			SchemaName: "test",
			Tables: []codecs.TableSchema{
				{
					Name:    "test",
					Columns: []codecs.TableColumn{{Name: "id", Type: "int"}},
				},
			},
		})

	// Verify all blocks were added
	if builder.GetBlockCount() != 4 {
		t.Errorf("Expected 4 blocks, got %d", builder.GetBlockCount())
	}

	// Verify kinds are tracked
	graph := builder.GetGraph()
	blocks := graph.GetAllBlocks()

	kindCounts := make(map[context.BlockKind]int)
	for _, block := range blocks {
		kindCounts[block.Meta.Kind]++
	}

	if kindCounts[context.KindPinned] != 1 {
		t.Errorf("Expected 1 Pinned block, got %d", kindCounts[context.KindPinned])
	}
	if kindCounts[context.KindReference] != 1 {
		t.Errorf("Expected 1 Reference block, got %d", kindCounts[context.KindReference])
	}
	if kindCounts[context.KindHistory] != 1 {
		t.Errorf("Expected 1 History block, got %d", kindCounts[context.KindHistory])
	}
	if kindCounts[context.KindTurn] != 1 {
		t.Errorf("Expected 1 Turn block, got %d", kindCounts[context.KindTurn])
	}
}

func TestBuilder_BlockOptions(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()

	// Add block with custom options
	builder.System(systemCodec, "System prompt", context.BlockOptions{
		Sensitivity: context.SensitivityRestricted,
		Source:      "test-source",
		Tags:        []string{"tag1", "tag2"},
	})

	blocks := builder.GetGraph().GetAllBlocks()
	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(blocks))
	}

	block := blocks[0]
	if block.Meta.Sensitivity != context.SensitivityRestricted {
		t.Errorf("Expected Restricted sensitivity, got %s", block.Meta.Sensitivity)
	}
	if block.Meta.Source != "test-source" {
		t.Errorf("Expected source 'test-source', got '%s'", block.Meta.Source)
	}
	if len(block.Meta.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(block.Meta.Tags))
	}
}

func TestBuilder_Clear(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()

	// Add some blocks
	builder.
		System(systemCodec, "System 1").
		System(systemCodec, "System 2").
		Turn(systemCodec, "User message")

	if builder.GetBlockCount() != 3 {
		t.Fatalf("Expected 3 blocks before clear, got %d", builder.GetBlockCount())
	}

	// Clear all blocks
	builder.Clear()

	if builder.GetBlockCount() != 0 {
		t.Errorf("Expected 0 blocks after clear, got %d", builder.GetBlockCount())
	}

	// Verify we can add blocks after clear
	builder.System(systemCodec, "New system")
	if builder.GetBlockCount() != 1 {
		t.Errorf("Expected 1 block after adding to cleared builder, got %d", builder.GetBlockCount())
	}
}

func TestBuilder_MultipleBlocksSameKind(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()

	// Add multiple system blocks
	builder.
		System(systemCodec, "System rule 1").
		System(systemCodec, "System rule 2").
		System(systemCodec, "System rule 3")

	if builder.GetBlockCount() != 3 {
		t.Errorf("Expected 3 blocks, got %d", builder.GetBlockCount())
	}

	blocks := builder.GetGraph().GetAllBlocks()
	pinnedCount := 0
	for _, block := range blocks {
		if block.Meta.Kind == context.KindPinned {
			pinnedCount++
		}
	}

	if pinnedCount != 3 {
		t.Errorf("Expected 3 Pinned blocks, got %d", pinnedCount)
	}
}

func TestBuilder_DefaultSensitivity(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()

	// Add block without specifying sensitivity
	builder.System(systemCodec, "System prompt")

	blocks := builder.GetGraph().GetAllBlocks()
	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(blocks))
	}

	// Default sensitivity should be Public
	if blocks[0].Meta.Sensitivity != context.SensitivityPublic {
		t.Errorf("Expected default sensitivity Public, got %s", blocks[0].Meta.Sensitivity)
	}
}

func TestBuilder_ContentAddressing(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()

	// Add two blocks with same content
	builder.
		System(systemCodec, "Same content").
		System(systemCodec, "Same content")

	blocks := builder.GetGraph().GetAllBlocks()
	if len(blocks) != 2 {
		t.Fatalf("Expected 2 blocks, got %d", len(blocks))
	}

	// Both blocks should have the same hash (content-addressed)
	if blocks[0].Hash != blocks[1].Hash {
		t.Errorf("Expected identical hashes for identical content, got %s and %s", blocks[0].Hash, blocks[1].Hash)
	}
}

func TestBuilder_AllKindMethods(t *testing.T) {
	t.Parallel()

	builder := context.NewBuilder()
	systemCodec := codecs.NewSystemRulesCodec()
	schemaCodec := codecs.NewDatabaseSchemaCodec()
	historyCodec := codecs.NewConversationHistoryCodec()

	schema := codecs.DatabaseSchemaPayload{
		SchemaName: "test",
		Tables: []codecs.TableSchema{
			{Name: "t", Columns: []codecs.TableColumn{{Name: "id", Type: "int"}}},
		},
	}

	history := codecs.ConversationHistoryPayload{
		Messages: []codecs.ConversationMessage{{Role: "user", Content: "hi"}},
	}

	// Test all kind methods
	builder.
		System(systemCodec, "system").
		Reference(schemaCodec, schema).
		Memory(systemCodec, "memory").
		State(systemCodec, "state").
		ToolOutput(systemCodec, "tool output").
		History(historyCodec, history).
		Turn(systemCodec, "turn")

	if builder.GetBlockCount() != 7 {
		t.Errorf("Expected 7 blocks (one of each kind), got %d", builder.GetBlockCount())
	}

	// Verify each kind is present
	blocks := builder.GetGraph().GetAllBlocks()
	kinds := make(map[context.BlockKind]bool)
	for _, block := range blocks {
		kinds[block.Meta.Kind] = true
	}

	expectedKinds := []context.BlockKind{
		context.KindPinned,
		context.KindReference,
		context.KindMemory,
		context.KindState,
		context.KindToolOutput,
		context.KindHistory,
		context.KindTurn,
	}

	for _, kind := range expectedKinds {
		if !kinds[kind] {
			t.Errorf("Expected kind %s to be present", kind)
		}
	}
}
