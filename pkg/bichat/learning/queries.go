package learning

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ValidatedQuery represents a proven SQL query pattern that successfully answered a user question.
// These queries serve as examples for similar future questions.
type ValidatedQuery struct {
	ID               uuid.UUID `json:"id"`
	TenantID         uuid.UUID `json:"tenant_id"`
	Question         string    `json:"question"`           // Original user question
	SQL              string    `json:"sql"`                // Validated SQL query
	Summary          string    `json:"summary"`            // Brief description of what query does
	TablesUsed       []string  `json:"tables_used"`        // Tables referenced in query
	DataQualityNotes []string  `json:"data_quality_notes"` // Optional: Known issues with this query/data
	UsedCount        int       `json:"used_count"`         // Track reuse frequency
	CreatedAt        time.Time `json:"created_at"`
}

// ValidatedQuerySearchOpts configures validated query search.
type ValidatedQuerySearchOpts struct {
	TenantID uuid.UUID // Required: Multi-tenant isolation
	Tables   []string  // Optional: Filter to queries using specific tables
	Limit    int       // Optional: Max results (default: 10)
}

// ValidatedQueryStore defines the interface for storing and retrieving validated query patterns.
// This is part of Feature 5 (Query Pattern Library) but types are defined here for forward compatibility.
type ValidatedQueryStore interface {
	// Save creates a new validated query entry.
	// Returns error if tenant_id is missing or storage fails.
	Save(ctx context.Context, query ValidatedQuery) error

	// Search finds relevant validated queries using full-text search on question + summary.
	// Results are ordered by relevance and used_count.
	Search(ctx context.Context, question string, opts ValidatedQuerySearchOpts) ([]ValidatedQuery, error)

	// IncrementUsage increments the used_count for a validated query.
	IncrementUsage(ctx context.Context, id uuid.UUID) error

	// Delete removes a validated query entry.
	Delete(ctx context.Context, id uuid.UUID) error
}
