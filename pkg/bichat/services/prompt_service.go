package services

import (
	"context"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
)

// PromptService provides dynamic prompt rendering capabilities.
// It allows agents to generate context-aware system prompts based on
// database schemas, business context, and other runtime data.
type PromptService interface {
	// GetPromptData retrieves all data needed for prompt rendering.
	// This includes database schemas, business context, and configuration.
	GetPromptData(ctx context.Context) (*PromptData, error)

	// RenderPrompt renders a prompt template with the given data.
	// templateName identifies which prompt template to use.
	RenderPrompt(ctx context.Context, templateName string, data *PromptData) (string, error)
}

// PromptData contains all contextual data for prompt rendering
type PromptData struct {
	// Database schema information
	Tables []bichatsql.TableInfo

	// Business context
	TenantName       string
	TenantID         string
	OrganizationName string

	// User context
	UserName  string
	UserEmail string
	UserRole  string

	// Available tools
	Tools []ToolInfo

	// Custom context (for extensibility)
	CustomData map[string]any
}

// ToolInfo provides metadata about an available tool
type ToolInfo struct {
	Name        string
	Description string
	Parameters  string // JSON schema
}

// WithCustomData adds custom data to the prompt context
func (p *PromptData) WithCustomData(key string, value any) *PromptData {
	if p.CustomData == nil {
		p.CustomData = make(map[string]any)
	}
	p.CustomData[key] = value
	return p
}

// GetCustomData retrieves custom data by key
func (p *PromptData) GetCustomData(key string) (any, bool) {
	if p.CustomData == nil {
		return nil, false
	}
	val, ok := p.CustomData[key]
	return val, ok
}
