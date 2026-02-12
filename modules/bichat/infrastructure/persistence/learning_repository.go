package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LearningRepository implements learning.LearningStore using PostgreSQL.
// All queries enforce multi-tenant isolation via tenant_id filtering.
type LearningRepository struct {
	pool *pgxpool.Pool
}

// NewLearningRepository creates a new PostgreSQL learning repository.
func NewLearningRepository(pool *pgxpool.Pool) learning.LearningStore {
	return &LearningRepository{pool: pool}
}

// Save creates a new learning entry.
func (r *LearningRepository) Save(ctx context.Context, l learning.Learning) error {
	const op serrors.Op = "LearningRepository.Save"

	// Validate tenant ID
	if l.TenantID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "tenant_id is required")
	}

	// Use pool directly for queries
	conn := r.pool

	// Generate ID if not provided
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}

	const query = `
		INSERT INTO bichat.learnings (
			id, tenant_id, category, trigger_text, lesson,
			table_name, sql_patch, used_count, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, content_hash) DO UPDATE SET
			lesson = EXCLUDED.lesson,
			sql_patch = EXCLUDED.sql_patch,
			used_count = bichat.learnings.used_count + 1
	`

	_, err := conn.Exec(ctx, query,
		l.ID,
		l.TenantID,
		string(l.Category),
		l.Trigger,
		l.Lesson,
		nullString(l.TableName),
		nullString(l.SQLPatch),
		l.UsedCount,
		l.CreatedAt,
	)
	if err != nil {
		return serrors.E(op, err, "failed to insert learning")
	}

	return nil
}

// Search finds relevant learnings using PostgreSQL full-text search.
func (r *LearningRepository) Search(ctx context.Context, query string, opts learning.SearchOpts) ([]learning.Learning, error) {
	const op serrors.Op = "LearningRepository.Search"

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
			id, tenant_id, category, trigger_text, lesson,
			COALESCE(table_name, '') as table_name,
			COALESCE(sql_patch, '') as sql_patch,
			used_count, created_at
		FROM bichat.learnings
		WHERE tenant_id = $1
	`
	args := []interface{}{opts.TenantID}
	argIndex := 2

	// Add category filter if provided
	if opts.Category != nil {
		sqlQuery += fmt.Sprintf(" AND category = $%d", argIndex)
		args = append(args, string(*opts.Category))
		argIndex++
	}

	// Add table filter if provided
	if opts.TableName != "" {
		sqlQuery += fmt.Sprintf(" AND table_name = $%d", argIndex)
		args = append(args, opts.TableName)
		argIndex++
	}

	// Add full-text search if query provided
	if query != "" {
		// Use PostgreSQL full-text search with fallback to ILIKE for short queries
		if len(query) > 3 {
			// Full-text search for longer queries
			sqlQuery += fmt.Sprintf(`
				AND to_tsvector('english', trigger_text || ' ' || lesson)
				@@ plainto_tsquery('english', $%d)
			`, argIndex)
			args = append(args, query)
			argIndex++
			// Order by relevance and used_count
			sqlQuery += `
				ORDER BY
					ts_rank(to_tsvector('english', trigger_text || ' ' || lesson), plainto_tsquery('english', $` + fmt.Sprintf("%d", argIndex-1) + `)) DESC,
					used_count DESC,
					created_at DESC
			`
		} else {
			// ILIKE fallback for short queries
			sqlQuery += fmt.Sprintf(`
				AND (trigger_text ILIKE $%d OR lesson ILIKE $%d)
			`, argIndex, argIndex)
			args = append(args, "%"+query+"%")
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
		return nil, serrors.E(op, err, "failed to search learnings")
	}
	defer rows.Close()

	// Parse results
	var learnings []learning.Learning
	for rows.Next() {
		var l learning.Learning
		var tableName, sqlPatch string

		err := rows.Scan(
			&l.ID,
			&l.TenantID,
			&l.Category,
			&l.Trigger,
			&l.Lesson,
			&tableName,
			&sqlPatch,
			&l.UsedCount,
			&l.CreatedAt,
		)
		if err != nil {
			return nil, serrors.E(op, err, "failed to scan learning")
		}

		l.TableName = tableName
		l.SQLPatch = sqlPatch
		learnings = append(learnings, l)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err, "error iterating learnings")
	}

	return learnings, nil
}

// IncrementUsage increments the used_count for a learning.
func (r *LearningRepository) IncrementUsage(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "LearningRepository.IncrementUsage"

	// Get tenant ID for multi-tenant isolation
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	// Use pool directly for queries
	conn := r.pool

	const query = `
		UPDATE bichat.learnings
		SET used_count = used_count + 1
		WHERE id = $1 AND tenant_id = $2
	`

	result, err := conn.Exec(ctx, query, id, tenantID)
	if err != nil {
		return serrors.E(op, err, "failed to increment usage")
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, "learning not found")
	}

	return nil
}

// Delete removes a learning entry.
func (r *LearningRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "LearningRepository.Delete"

	// Get tenant ID for multi-tenant isolation
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	// Use pool directly for queries
	conn := r.pool

	const query = `
		DELETE FROM bichat.learnings
		WHERE id = $1 AND tenant_id = $2
	`

	result, err := conn.Exec(ctx, query, id, tenantID)
	if err != nil {
		return serrors.E(op, err, "failed to delete learning")
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, "learning not found")
	}

	return nil
}

// DeleteByTenant removes all learnings for a tenant.
func (r *LearningRepository) DeleteByTenant(ctx context.Context, tenantID uuid.UUID) error {
	const op serrors.Op = "LearningRepository.DeleteByTenant"

	if tenantID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "tenant_id is required")
	}

	const query = `
		DELETE FROM bichat.learnings
		WHERE tenant_id = $1
	`

	if _, err := r.pool.Exec(ctx, query, tenantID); err != nil {
		return serrors.E(op, err, "failed to delete tenant learnings")
	}

	return nil
}

// ListByTable retrieves all learnings related to a specific table.
func (r *LearningRepository) ListByTable(ctx context.Context, tenantID uuid.UUID, tableName string, limit int) ([]learning.Learning, error) {
	const op serrors.Op = "LearningRepository.ListByTable"

	// Validate tenant ID
	if tenantID == uuid.Nil {
		return nil, serrors.E(op, serrors.KindValidation, "tenant_id is required")
	}

	// Use pool directly for queries
	conn := r.pool

	// Set default limit
	if limit == 0 {
		limit = 10
	}

	const query = `
		SELECT
			id, tenant_id, category, trigger_text, lesson,
			COALESCE(table_name, '') as table_name,
			COALESCE(sql_patch, '') as sql_patch,
			used_count, created_at
		FROM bichat.learnings
		WHERE tenant_id = $1 AND table_name = $2
		ORDER BY used_count DESC, created_at DESC
		LIMIT $3
	`

	rows, err := conn.Query(ctx, query, tenantID, tableName, limit)
	if err != nil {
		return nil, serrors.E(op, err, "failed to list learnings by table")
	}
	defer rows.Close()

	// Parse results
	var learnings []learning.Learning
	for rows.Next() {
		var l learning.Learning
		var tn, sp string

		err := rows.Scan(
			&l.ID,
			&l.TenantID,
			&l.Category,
			&l.Trigger,
			&l.Lesson,
			&tn,
			&sp,
			&l.UsedCount,
			&l.CreatedAt,
		)
		if err != nil {
			return nil, serrors.E(op, err, "failed to scan learning")
		}

		l.TableName = tn
		l.SQLPatch = sp
		learnings = append(learnings, l)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err, "error iterating learnings")
	}

	return learnings, nil
}

// nullString returns a valid pgx value for optional string fields.
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
