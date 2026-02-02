package context_test

import (
	"fmt"
	"log"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/renderers"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// Example demonstrates basic context building and compilation.
func Example() {
	// Create codecs
	systemCodec := codecs.NewSystemRulesCodec()
	schemaCodec := codecs.NewDatabaseSchemaCodec()
	historyCodec := codecs.NewConversationHistoryCodec()

	// Build context
	builder := context.NewBuilder()
	builder.
		System(systemCodec, codecs.SystemRulesPayload{
			Text: "You are a helpful BI assistant. Answer questions about data.",
		}).
		Reference(schemaCodec, codecs.DatabaseSchemaPayload{
			SchemaName: "public",
			Tables: []codecs.TableSchema{
				{
					Name: "users",
					Columns: []codecs.TableColumn{
						{Name: "id", Type: "integer", Nullable: false},
						{Name: "name", Type: "text", Nullable: false},
						{Name: "email", Type: "text", Nullable: false},
					},
				},
			},
		}).
		History(historyCodec, codecs.ConversationHistoryPayload{
			Messages: []codecs.ConversationMessage{
				{Role: "user", Content: "How many users do we have?"},
				{Role: "assistant", Content: "Let me query the database for you."},
			},
		}).
		Turn(systemCodec, "What is the average age of our users?")

	// Create renderer
	renderer := renderers.NewAnthropicRenderer()

	// Create policy
	policy := context.DefaultPolicy()

	// Compile context
	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Total tokens: %d\n", compiled.TotalTokens)
	fmt.Printf("Truncated: %v\n", compiled.Truncated)
	// System messages are now in compiled.Messages
	systemMsgCount := 0
	for _, msg := range compiled.Messages {
		if msg.Role == types.RoleSystem {
			systemMsgCount++
		}
	}
	fmt.Printf("System messages count: %d\n", systemMsgCount)
	fmt.Printf("Messages count: %d\n", len(compiled.Messages))

	// Output:
	// Total tokens: 37
	// Truncated: false
	// System messages count: 1
	// Messages count: 2
}

// Example_biQueries demonstrates BI-specific codecs for SQL queries.
func Example_biQueries() {
	// Create query result codec with max 100 rows
	queryCodec := codecs.NewQueryResultCodec(codecs.WithMaxRows(100))

	builder := context.NewBuilder()
	builder.ToolOutput(queryCodec, codecs.QueryResultPayload{
		Query:   "SELECT COUNT(*) FROM users",
		Columns: []string{"count"},
		Rows: [][]any{
			{42},
		},
		RowCount:   1,
		ExecutedAt: "2024-01-31T10:00:00Z",
	})

	renderer := renderers.NewAnthropicRenderer()
	policy := context.DefaultPolicy()

	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Messages: %d\n", len(compiled.Messages))

	// Output:
	// Messages: 1
}

// Example_filtering demonstrates block filtering with the query DSL.
func Example_filtering() {
	systemCodec := codecs.NewSystemRulesCodec()

	builder := context.NewBuilder()
	builder.
		System(systemCodec, "System prompt 1", context.BlockOptions{
			Tags: []string{"important"},
		}).
		System(systemCodec, "System prompt 2", context.BlockOptions{
			Tags: []string{"optional"},
		})

	// Filter blocks by tag
	graph := builder.GetGraph()
	importantBlocks := graph.Select(
		context.HasTag("important"),
	)

	fmt.Printf("Important blocks: %d\n", len(importantBlocks))

	// Output:
	// Important blocks: 1
}

// Example_tokenBudgeting demonstrates overflow handling.
func Example_tokenBudgeting() {
	systemCodec := codecs.NewSystemRulesCodec()

	builder := context.NewBuilder()
	for i := 0; i < 100; i++ {
		builder.System(systemCodec, fmt.Sprintf("Rule %d: Very long text here...", i))
	}

	renderer := renderers.NewAnthropicRenderer()

	// Create policy with small budget to trigger truncation
	policy := context.ContextPolicy{
		ContextWindow:     1000,
		CompletionReserve: 100,
		OverflowStrategy:  context.OverflowTruncate,
		KindPriorities:    context.DefaultKindPriorities(),
		MaxSensitivity:    context.SensitivityPublic,
		RedactRestricted:  true,
	}

	compiled, err := builder.Compile(renderer, policy)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Truncated: %v\n", compiled.Truncated)
	fmt.Printf("Total tokens: %d (under budget: %d)\n", compiled.TotalTokens, policy.ContextWindow-policy.CompletionReserve)

	// Output:
	// Truncated: false
	// Total tokens: 700 (under budget: 900)
}

// Example_multiProvider demonstrates using different renderers.
func Example_multiProvider() {
	systemCodec := codecs.NewSystemRulesCodec()

	builder := context.NewBuilder()
	builder.System(systemCodec, "You are a helpful assistant.")

	policy := context.DefaultPolicy()

	// Compile for Anthropic
	anthropic := renderers.NewAnthropicRenderer()
	compiledAnthropic, _ := builder.Compile(anthropic, policy)
	fmt.Printf("Anthropic provider: %s\n", anthropic.Provider())
	fmt.Printf("Anthropic tokens: %d\n", compiledAnthropic.TotalTokens)

	// Compile for OpenAI
	openai := renderers.NewOpenAIRenderer()
	compiledOpenAI, _ := builder.Compile(openai, policy)
	fmt.Printf("OpenAI provider: %s\n", openai.Provider())
	fmt.Printf("OpenAI tokens: %d\n", compiledOpenAI.TotalTokens)

	// Compile for Gemini
	gemini := renderers.NewGeminiRenderer()
	compiledGemini, _ := builder.Compile(gemini, policy)
	fmt.Printf("Gemini provider: %s\n", gemini.Provider())
	fmt.Printf("Gemini tokens: %d\n", compiledGemini.TotalTokens)

	// Output:
	// Anthropic provider: anthropic
	// Anthropic tokens: 6
	// OpenAI provider: openai
	// OpenAI tokens: 6
	// Gemini provider: gemini
	// Gemini tokens: 6
}
