package learning

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Category represents the type of learning captured by the agent.
type Category string

const (
	// CategorySQLError captures SQL execution errors and their fixes.
	CategorySQLError Category = "sql_error"

	// CategoryTypeMismatch captures type casting issues and solutions.
	CategoryTypeMismatch Category = "type_mismatch"

	// CategoryUserCorrection captures user-provided corrections and feedback.
	CategoryUserCorrection Category = "user_correction"

	// CategoryBusinessRule captures business rules and domain knowledge.
	CategoryBusinessRule Category = "business_rule"
)

// Learning represents a captured insight about SQL errors, type mismatches, or user corrections.
// Learnings help the agent avoid repeating mistakes across conversations.
type Learning struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Category  Category  `json:"category"`
	Trigger   string    `json:"trigger"`    // What caused this learning (error message, user input)
	Lesson    string    `json:"lesson"`     // What to do/avoid next time
	TableName string    `json:"table_name"` // Optional: Related table for schema-specific learnings
	SQLPatch  string    `json:"sql_patch"`  // Optional: SQL fix or pattern to apply
	UsedCount int       `json:"used_count"` // Track how often this learning has been retrieved
	CreatedAt time.Time `json:"created_at"`
}

// SearchOpts configures learning search queries.
type SearchOpts struct {
	TenantID  uuid.UUID // Required: Multi-tenant isolation
	Category  *Category // Optional: Filter by learning category
	TableName string    // Optional: Filter by related table
	Limit     int       // Optional: Max results (default: 10)
}

// LearningStore defines the interface for storing and retrieving agent learnings.
// Implementations must ensure multi-tenant isolation via tenant_id filtering.
type LearningStore interface {
	// Save creates a new learning entry.
	// Returns error if tenant_id is missing or storage fails.
	Save(ctx context.Context, learning Learning) error

	// Search finds relevant learnings using full-text search on trigger + lesson.
	// The query is matched against both trigger and lesson fields.
	// Results are ordered by relevance (text search rank) and used_count.
	Search(ctx context.Context, query string, opts SearchOpts) ([]Learning, error)

	// IncrementUsage increments the used_count for a learning.
	// This tracks which learnings are most valuable for future prioritization.
	IncrementUsage(ctx context.Context, id uuid.UUID) error

	// Delete removes a learning entry.
	// Requires both id and tenant_id for multi-tenant isolation.
	Delete(ctx context.Context, id uuid.UUID) error

	// ListByTable retrieves all learnings related to a specific table.
	// Useful for providing context when describing table schemas.
	ListByTable(ctx context.Context, tenantID uuid.UUID, tableName string, limit int) ([]Learning, error)
}
