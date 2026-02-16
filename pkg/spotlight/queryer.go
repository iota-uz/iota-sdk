package spotlight

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Queryer is a minimal interface for database query operations needed by spotlight providers.
// This allows spotlight providers to query the database directly for document indexing.
type Queryer interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}
