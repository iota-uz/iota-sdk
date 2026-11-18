package notify

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// PgPublished lists published versions that carry notices or templates.
type PgPublished struct{ DB *pgxpool.Pool }

var _ Published = PgPublished{}

// PublishedRefs implements Published.
func (p PgPublished) PublishedRefs(ctx context.Context) ([]string, error) {
	const op = serrors.Op("extruntime.notify.PgPublished.PublishedRefs")
	rows, err := p.DB.Query(ctx, `SELECT namespace || '@' || version FROM extruntime.versions
		WHERE manifest -> 'resources' @> '[{"kind": "notice"}]'::jsonb
		   OR coalesce(jsonb_array_length(manifest -> 'templates'), 0) > 0
		ORDER BY created_at, id`)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		out = append(out, ref)
	}
	return out, serrors.FromDB(op, rows.Err())
}
