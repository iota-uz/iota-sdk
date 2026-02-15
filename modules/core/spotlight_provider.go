package core

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	return spotlight.ProviderCapabilities{SupportsWatch: false, EntityTypes: []string{"user", "group", "role"}}
}

func (p *spotlightProvider) ListDocuments(ctx context.Context, scope spotlight.ProviderScope) ([]spotlight.SearchDocument, error) {
	rows, err := p.db.Query(ctx, `
SELECT id, first_name, last_name, email, phone, updated_at
FROM users
WHERE tenant_id = $1
LIMIT 1000
`, scope.TenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]spotlight.SearchDocument, 0, 256)
	for rows.Next() {
		var id int64
		var firstName string
		var lastName string
		var email string
		var phone *string
		var updatedAt time.Time
		if err := rows.Scan(&id, &firstName, &lastName, &email, &phone, &updatedAt); err != nil {
			return nil, err
		}
		title := strings.TrimSpace(firstName + " " + lastName)
		if title == "" {
			title = fmt.Sprintf("User %d", id)
		}
		out = append(out, spotlight.SearchDocument{
			ID:         fmt.Sprintf("core:user:%d", id),
			EntityType: "user",
			Title:      title,
			Body:       strings.TrimSpace(email + " " + strings.Join(optional(phone), " ")),
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
		return nil, err
	}
	return out, nil
}

func optional(values ...*string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		out = append(out, strings.TrimSpace(*value))
	}
	return out
}

func (p *spotlightProvider) Watch(_ context.Context, _ spotlight.ProviderScope) (<-chan spotlight.DocumentEvent, error) {
	return nil, nil
}
