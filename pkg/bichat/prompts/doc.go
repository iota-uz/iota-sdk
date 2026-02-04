// Package prompts provides extensible prompt loading for BiChat agents.
//
// # Overview
//
// The prompts package enables domain-specific customization of agent system prompts.
// Instead of hardcoding prompts in agent implementations, consumers can provide
// their own prompt templates that include domain context, business rules, and
// custom instructions.
//
// # Usage
//
// Basic usage with embedded filesystem:
//
//	//go:embed prompts/*.md
//	var promptFS embed.FS
//
//	loader := prompts.NewFilePromptLoader(promptFS, prompts.WithBasePath("prompts"))
//
//	vars := map[string]any{
//	    "Timezone":      "Asia/Tashkent",
//	    "BusinessRules": "OSAGO policy requires valid driver license...",
//	    "DataSources":   []string{"policies", "claims", "customers"},
//	}
//
//	prompt, err := loader.Load(ctx, "parent", vars)
//
// # Template Variables
//
// Prompt templates use Go's text/template syntax. Common variables:
//   - Timezone: User's timezone for date/time context
//   - BusinessRules: Domain-specific rules and constraints
//   - DataSources: Available database tables or data sources
//   - CurrentDate: Today's date for temporal queries
//
// # Implementations
//
// The package provides several PromptLoader implementations:
//   - FilePromptLoader: Loads from embedded or real filesystem
//   - StaticPromptLoader: Returns fixed prompt (testing)
//   - MapPromptLoader: In-memory map of prompts (testing/programmatic)
//
// Consumers can implement PromptLoader for custom sources (database, API, etc.).
package prompts
