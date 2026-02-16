package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/jackc/pgx/v5"
)

type spotlightProvider struct {
	db queryer
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

var _ spotlight.SearchProvider = &spotlightProvider{}

func newSpotlightProvider(db queryer) *spotlightProvider {
	return &spotlightProvider{db: db}
}

func (p *spotlightProvider) ProviderID() string {
	return "core.entities"
}

func (p *spotlightProvider) Capabilities() spotlight.ProviderCapabilities {
	return spotlight.ProviderCapabilities{SupportsWatch: false, EntityTypes: []string{"user"}}
}

func (p *spotlightProvider) ListDocuments(ctx context.Context, scope spotlight.ProviderScope) ([]spotlight.SearchDocument, error) {
	const op serrors.Op = "core.spotlightProvider.ListDocuments"

	rows, err := p.db.Query(ctx, `
SELECT id, first_name, last_name, updated_at
FROM users
WHERE tenant_id = $1
ORDER BY id ASC
LIMIT 1000
`, scope.TenantID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	out := make([]spotlight.SearchDocument, 0, 256)
	for rows.Next() {
		var id int64
		var firstName string
		var lastName string
		var updatedAt time.Time
		if err := rows.Scan(&id, &firstName, &lastName, &updatedAt); err != nil {
			return nil, serrors.E(op, err)
		}
		title := strings.TrimSpace(firstName + " " + lastName)
		if title == "" {
			title = fmt.Sprintf("User %d", id)
		}
		out = append(out, spotlight.SearchDocument{
			ID:         fmt.Sprintf("core:user:%d", id),
			EntityType: "user",
			Title:      title,
			Body:       title,
			URL:        fmt.Sprintf("/users/%d", id),
			Language:   scope.Language,
			Metadata: map[string]string{
				"source": "core.users",
			},
			Access:    spotlight.AccessPolicy{Visibility: spotlight.VisibilityPublic},
			UpdatedAt: updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

func (p *spotlightProvider) Watch(_ context.Context, _ spotlight.ProviderScope) (<-chan spotlight.DocumentEvent, error) {
	return nil, nil
}
