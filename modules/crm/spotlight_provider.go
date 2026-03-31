// Package crm provides this package.
package crm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/sirupsen/logrus"
)

type spotlightProvider struct {
	db           spotlight.Queryer
	maxDocuments int
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
	return spotlight.ProviderCapabilities{SupportsWatch: false, EntityTypes: []string{"client"}}
}

func (p *spotlightProvider) StreamDocuments(ctx context.Context, scope spotlight.ProviderScope, emit spotlight.DocumentBatchEmitter) error {
	const op serrors.Op = "crm.spotlightProvider.ListDocuments"
	limit := p.maxDocuments

	var query string
	var args []any
	if limit > 0 {
		query = `
SELECT id, first_name, last_name, middle_name, updated_at
FROM clients
WHERE tenant_id = $1
ORDER BY id ASC
LIMIT $2`
		args = []any{scope.TenantID, limit}
	} else {
		query = `
SELECT id, first_name, last_name, middle_name, updated_at
FROM clients
WHERE tenant_id = $1
ORDER BY id ASC`
		args = []any{scope.TenantID}
	}

	rows, err := p.db.Query(ctx, query, args...)
	if err != nil {
		return serrors.E(op, err)
	}
	defer rows.Close()

	out := make([]spotlight.SearchDocument, 0, spotlight.ProviderStreamBatchSize)
	count := 0
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
		count++
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
	if limit > 0 && count == limit {
		logrus.WithFields(logrus.Fields{
			"tenant_id": scope.TenantID.String(),
			"limit":     limit,
		}).Warn("crm spotlight provider reached document cap, results may be truncated")
	}
	if len(out) > 0 {
		if err := emit(out); err != nil {
			return serrors.E(op, err)
		}
	}
	return nil
}

func (p *spotlightProvider) Watch(_ context.Context, _ spotlight.ProviderScope) (<-chan spotlight.DocumentEvent, error) {
	changes := make(chan spotlight.DocumentEvent)
	close(changes)
	return changes, nil
}
