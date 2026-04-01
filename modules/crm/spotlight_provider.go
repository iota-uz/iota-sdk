// Package crm provides this package.
package crm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

type spotlightProvider struct {
	db spotlight.Queryer
}

var _ spotlight.SearchProvider = &spotlightProvider{}

func newSpotlightProvider(db spotlight.Queryer) *spotlightProvider {
	return &spotlightProvider{
		db: db,
	}
}

func (p *spotlightProvider) ProviderID() string {
	return "crm.clients"
}

func (p *spotlightProvider) Capabilities() spotlight.ProviderCapabilities {
	return spotlight.ProviderCapabilities{EntityTypes: []string{"client"}}
}

func (p *spotlightProvider) StreamDocuments(ctx context.Context, scope spotlight.ProviderScope, emit spotlight.DocumentBatchEmitter) error {
	const op serrors.Op = "crm.spotlightProvider.ListDocuments"

	query := `
SELECT id, first_name, last_name, middle_name, updated_at
FROM clients
WHERE tenant_id = $1
ORDER BY id ASC`

	rows, err := p.db.Query(ctx, query, scope.TenantID)
	if err != nil {
		return serrors.E(op, err)
	}
	defer rows.Close()

	out := make([]spotlight.SearchDocument, 0, spotlight.ProviderStreamBatchSize)
	for rows.Next() {
		var id int64
		var firstName string
		var lastName *string
		var middleName *string
		var updatedAt time.Time
		if err := rows.Scan(&id, &firstName, &lastName, &middleName, &updatedAt); err != nil {
			return serrors.E(op, err)
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
		out = append(out, spotlight.SearchDocument{
			ID:          fmt.Sprintf("crm:client:%d", id),
			TenantID:    scope.TenantID,
			Provider:    p.ProviderID(),
			EntityType:  "client",
			Domain:      spotlight.ResultDomainLookup,
			Title:       title,
			Description: title,
			Body:        title,
			SearchText:  title,
			ExactTerms:  spotlight.ExpandExactTerms(fmt.Sprintf("%d", id), title),
			URL:         fmt.Sprintf("/crm/clients?tab=profile&view=%d", id),
			Language:    scope.Language,
			Metadata: map[string]string{
				"source":          "crm.clients",
				"group_key":       "people",
				"group_title_key": "Spotlight.Group.People",
			},
			Access:    spotlight.AccessPolicy{Visibility: spotlight.VisibilityPublic},
			UpdatedAt: updatedAt,
		})
		if len(out) == spotlight.ProviderStreamBatchSize {
			if err := emit(out); err != nil {
				return serrors.E(op, err)
			}
			out = make([]spotlight.SearchDocument, 0, spotlight.ProviderStreamBatchSize)
		}
	}
	if err := rows.Err(); err != nil {
		return serrors.E(op, err)
	}
	if len(out) > 0 {
		if err := emit(out); err != nil {
			return serrors.E(op, err)
		}
	}
	return nil
}

