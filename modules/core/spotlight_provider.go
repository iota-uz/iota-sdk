// Package core provides this package.
package core

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
	return &spotlightProvider{db: db}
}

func (p *spotlightProvider) ProviderID() string {
	return "core.entities"
}

func (p *spotlightProvider) Capabilities() spotlight.ProviderCapabilities {
	return spotlight.ProviderCapabilities{EntityTypes: []string{"user"}}
}

func (p *spotlightProvider) StreamDocuments(ctx context.Context, scope spotlight.ProviderScope, emit spotlight.DocumentBatchEmitter) error {
	const op serrors.Op = "core.spotlightProvider.ListDocuments"

	rows, err := p.db.Query(ctx, `
SELECT
    u.id,
    u.first_name,
    u.last_name,
    COALESCE(u.middle_name, ''),
    u.email,
    COALESCE(u.phone, ''),
    u.type,
    u.updated_at,
    COALESCE(string_agg(DISTINCT r.name, E'\n') FILTER (WHERE r.name IS NOT NULL), '')
FROM users u
LEFT JOIN user_roles ur ON ur.user_id = u.id
LEFT JOIN roles r ON r.id = ur.role_id
WHERE u.tenant_id = $1
GROUP BY u.id, u.first_name, u.last_name, u.middle_name, u.email, u.phone, u.type, u.updated_at
ORDER BY u.updated_at DESC, u.id ASC
LIMIT 5000
	`, scope.TenantID)
	if err != nil {
		return serrors.E(op, err)
	}
	defer rows.Close()

	out := make([]spotlight.SearchDocument, 0, spotlight.ProviderStreamBatchSize)
	for rows.Next() {
		var id int64
		var firstName string
		var lastName string
		var middleName string
		var email string
		var phone string
		var userType string
		var updatedAt time.Time
		var roles string
		if err := rows.Scan(&id, &firstName, &lastName, &middleName, &email, &phone, &userType, &updatedAt, &roles); err != nil {
			return serrors.E(op, err)
		}
		title := strings.TrimSpace(strings.Join([]string{firstName, middleName, lastName}, " "))
		if title == "" {
			if email != "" {
				title = email
			} else if phone != "" {
				title = phone
			} else {
				title = fmt.Sprintf("User %d", id)
			}
		}

		description := strings.TrimSpace(strings.Join([]string{email, phone}, " · "))
		searchText := spotlight.BuildSearchText(title, email, phone, roles, userType)
		exactTerms := spotlight.ExpandExactTerms(
			fmt.Sprintf("%d", id),
			title,
			email,
			phone,
		)

		out = append(out, spotlight.SearchDocument{
			ID:          fmt.Sprintf("core:user:%d", id),
			TenantID:    scope.TenantID,
			Provider:    p.ProviderID(),
			EntityType:  "user",
			Domain:      spotlight.ResultDomainLookup,
			Title:       title,
			Description: description,
			Body:        searchText,
			SearchText:  searchText,
			ExactTerms:  exactTerms,
			URL:         fmt.Sprintf("/users/%d", id),
			Language:    scope.Language,
			Metadata: map[string]string{
				"source":          "core.users",
				"group_key":       "staff",
				"group_title_key": "Spotlight.Group.Staff",
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

