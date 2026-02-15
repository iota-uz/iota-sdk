package crm

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
	return "crm.clients"
}

func (p *spotlightProvider) Capabilities() spotlight.ProviderCapabilities {
	return spotlight.ProviderCapabilities{SupportsWatch: false, EntityTypes: []string{"client"}}
}

func (p *spotlightProvider) ListDocuments(ctx context.Context, scope spotlight.ProviderScope) ([]spotlight.SearchDocument, error) {
	rows, err := p.db.Query(ctx, `
SELECT id, first_name, last_name, middle_name, phone_number, email, updated_at
FROM clients
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
		var lastName *string
		var middleName *string
		var phone *string
		var email *string
		var updatedAt time.Time
		if err := rows.Scan(&id, &firstName, &lastName, &middleName, &phone, &email, &updatedAt); err != nil {
			return nil, err
		}
		nameParts := []string{firstName}
		if middleName != nil {
			nameParts = append(nameParts, *middleName)
		}
		if lastName != nil {
			nameParts = append(nameParts, *lastName)
		}
		title := strings.TrimSpace(strings.Join(nameParts, " "))
		if title == "" {
			title = fmt.Sprintf("Client %d", id)
		}
		bodyParts := []string{}
		if phone != nil {
			bodyParts = append(bodyParts, *phone)
		}
		if email != nil {
			bodyParts = append(bodyParts, *email)
		}
		out = append(out, spotlight.SearchDocument{
			ID:         fmt.Sprintf("crm:client:%d", id),
			EntityType: "client",
			Title:      title,
			Body:       strings.TrimSpace(strings.Join(bodyParts, " ")),
			URL:        fmt.Sprintf("/crm/clients?tab=profile&view=%d", id),
			Language:   scope.Language,
			Metadata: map[string]string{
				"source": "crm.clients",
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

func (p *spotlightProvider) Watch(_ context.Context, _ spotlight.ProviderScope) (<-chan spotlight.DocumentEvent, error) {
	return nil, nil
}
