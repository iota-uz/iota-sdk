package persistence

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ValidatedQueryRepository implements learning.ValidatedQueryStore using PostgreSQL.
// All queries enforce multi-tenant isolation via tenant_id filtering.
type ValidatedQueryRepository struct {
	pool *pgxpool.Pool
}

// NewValidatedQueryRepository creates a new PostgreSQL validated query repository.
func NewValidatedQueryRepository(pool *pgxpool.Pool) learning.ValidatedQueryStore {
	return &ValidatedQueryRepository{pool: pool}
}

// Save creates a new validated query entry.
func (r *ValidatedQueryRepository) Save(ctx context.Context, query learning.ValidatedQuery) error {
	const op serrors.Op = "ValidatedQueryRepository.Save"

	// Validate tenant ID
	if query.TenantID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "tenant_id is required")
	}

	// Validate SQL is SELECT/WITH only
	normalizedSQL := strings.TrimSpace(strings.ToUpper(query.SQL))
	if !strings.HasPrefix(normalizedSQL, "SELECT") && !strings.HasPrefix(normalizedSQL, "WITH") {
		return serrors.E(op, serrors.KindValidation, "only SELECT and WITH queries can be saved")
	}

	// Use pool directly for queries
	conn := r.pool

	// Generate ID if not provided
	if query.ID == uuid.Nil {
		query.ID = uuid.New()
	}

	const sqlQuery = `
		INSERT INTO bichat.validated_queries (
			id, tenant_id, question, sql, summary,
			tables_used, data_quality_notes, used_count, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, sql_hash) DO UPDATE SET
			question = EXCLUDED.question,
			summary = EXCLUDED.summary,
			tables_used = EXCLUDED.tables_used,
			data_quality_notes = EXCLUDED.data_quality_notes,
			used_count = bichat.validated_queries.used_count + 1
	`

	_, err := conn.Exec(ctx, sqlQuery,
		query.ID,
		query.TenantID,
		query.Question,
		query.SQL,
		query.Summary,
		query.TablesUsed,
		query.DataQualityNotes,
		query.UsedCount,
		query.CreatedAt,
	)
	if err != nil {
		return serrors.E(op, err, "failed to insert validated query")
	}

	return nil
}

// Search finds relevant validated queries using PostgreSQL full-text search.
func (r *ValidatedQueryRepository) Search(ctx context.Context, question string, opts learning.ValidatedQuerySearchOpts) ([]learning.ValidatedQuery, error) {
	const op serrors.Op = "ValidatedQueryRepository.Search"

	// Validate tenant ID
	if opts.TenantID == uuid.Nil {
		return nil, serrors.E(op, serrors.KindValidation, "tenant_id is required")
	}

	// Use pool directly for queries
	conn := r.pool

	// Set default limit
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}

	// Build query with optional filters
	sqlQuery := `
		SELECT
			id, tenant_id, question, sql, summary,
			tables_used, data_quality_notes, used_count, created_at
		FROM bichat.validated_queries
		WHERE tenant_id = $1
	`
	args := []interface{}{opts.TenantID}
	argIndex := 2

	// Add tables filter if provided (array overlap)
	if len(opts.Tables) > 0 {
		sqlQuery += fmt.Sprintf(" AND tables_used && $%d", argIndex)
		args = append(args, opts.Tables)
		argIndex++
	}

	// Add full-text search if query provided
	if question != "" {
		// Use PostgreSQL full-text search with fallback to ILIKE for short queries
		if len(question) > 3 {
			// Full-text search for longer queries
			sqlQuery += fmt.Sprintf(`
				AND to_tsvector('english', question || ' ' || summary)
				@@ plainto_tsquery('english', $%d)
			`, argIndex)
			args = append(args, question)
			argIndex++
			// Order by relevance and used_count
			sqlQuery += `
				ORDER BY
					ts_rank(to_tsvector('english', question || ' ' || summary), plainto_tsquery('english', $` + fmt.Sprintf("%d", argIndex-1) + `)) DESC,
					used_count DESC,
					created_at DESC
			`
		} else {
			// ILIKE fallback for short queries
			sqlQuery += fmt.Sprintf(`
				AND (question ILIKE $%d OR summary ILIKE $%d)
			`, argIndex, argIndex)
			args = append(args, "%"+question+"%")
			argIndex++
			// Order by used_count and recency
			sqlQuery += ` ORDER BY used_count DESC, created_at DESC`
		}
	} else {
		// No search query - order by used_count and recency
		sqlQuery += ` ORDER BY used_count DESC, created_at DESC`
	}

	// Add limit
	sqlQuery += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, limit)

	// Execute query
	rows, err := conn.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, serrors.E(op, err, "failed to search validated queries")
	}
	defer rows.Close()

	// Parse results
	var queries []learning.ValidatedQuery
	for rows.Next() {
		var q learning.ValidatedQuery

		err := rows.Scan(
			&q.ID,
			&q.TenantID,
			&q.Question,
			&q.SQL,
			&q.Summary,
			&q.TablesUsed,
			&q.DataQualityNotes,
			&q.UsedCount,
			&q.CreatedAt,
		)
		if err != nil {
			return nil, serrors.E(op, err, "failed to scan validated query")
		}

		queries = append(queries, q)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err, "error iterating validated queries")
	}

	return queries, nil
}

// IncrementUsage increments the used_count for a validated query.
func (r *ValidatedQueryRepository) IncrementUsage(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "ValidatedQueryRepository.IncrementUsage"

	// Get tenant ID for multi-tenant isolation
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	// Use pool directly for queries
	conn := r.pool

	const query = `
		UPDATE bichat.validated_queries
		SET used_count = used_count + 1
		WHERE id = $1 AND tenant_id = $2
	`

	result, err := conn.Exec(ctx, query, id, tenantID)
	if err != nil {
		return serrors.E(op, err, "failed to increment usage")
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, "validated query not found")
	}

	return nil
}

// Delete removes a validated query entry.
func (r *ValidatedQueryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "ValidatedQueryRepository.Delete"

	// Get tenant ID for multi-tenant isolation
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	// Use pool directly for queries
	conn := r.pool

	const query = `
		DELETE FROM bichat.validated_queries
		WHERE id = $1 AND tenant_id = $2
	`

	result, err := conn.Exec(ctx, query, id, tenantID)
	if err != nil {
		return serrors.E(op, err, "failed to delete validated query")
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, "validated query not found")
	}

	return nil
}

// DeleteByTenant removes all validated query patterns for a tenant.
// This is used by knowledge rebuild operations.
func (r *ValidatedQueryRepository) DeleteByTenant(ctx context.Context, tenantID uuid.UUID) error {
	const op serrors.Op = "ValidatedQueryRepository.DeleteByTenant"

	if tenantID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "tenant_id is required")
	}

	const query = `
		DELETE FROM bichat.validated_queries
		WHERE tenant_id = $1
	`

	if _, err := r.pool.Exec(ctx, query, tenantID); err != nil {
		return serrors.E(op, err, "failed to delete tenant validated queries")
	}

	return nil
}
