package spotlight

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type countingPrincipalResolver struct {
	principal Principal
	err       error
	calls     int
}

func (r *countingPrincipalResolver) Resolve(_ context.Context, _ SearchRequest) (Principal, error) {
	r.calls++
	return r.principal, r.err
}

func TestStrictACLEvaluator_FilterAuthorized_ResolvesPrincipalOnce(t *testing.T) {
	t.Helper()

	tenantID := uuid.New()
	resolver := &countingPrincipalResolver{
		principal: Principal{
			UserID:      "42",
			Roles:       []string{"admin"},
			Permissions: []string{"core.roles.view"},
		},
	}
	evaluator := NewStrictACLEvaluator(resolver)
	req := SearchRequest{
		TenantID: tenantID,
	}
	hits := []SearchHit{
		{Document: SearchDocument{TenantID: tenantID, Access: AccessPolicy{Visibility: VisibilityPublic}}},
		{Document: SearchDocument{TenantID: tenantID, Access: AccessPolicy{Visibility: VisibilityRestricted, AllowedRoles: []string{"admin"}}}},
		{Document: SearchDocument{TenantID: tenantID, Access: AccessPolicy{Visibility: VisibilityRestricted, AllowedUsers: []string{"100"}}}},
		{Document: SearchDocument{TenantID: tenantID, Access: AccessPolicy{Visibility: VisibilityOwner, OwnerID: "42"}}},
		{Document: SearchDocument{TenantID: uuid.New(), Access: AccessPolicy{Visibility: VisibilityPublic}}},
	}

	filtered := evaluator.FilterAuthorized(context.Background(), req, hits)

	require.Len(t, filtered, 3)
	require.Equal(t, 1, resolver.calls)
}

func TestStrictACLEvaluator_FilterAuthorized_SkipsResolveForPublicOnly(t *testing.T) {
	t.Helper()

	tenantID := uuid.New()
	resolver := &countingPrincipalResolver{
		principal: Principal{UserID: "42"},
	}
	evaluator := NewStrictACLEvaluator(resolver)
	req := SearchRequest{
		TenantID: tenantID,
		UserID:   "42",
	}
	hits := []SearchHit{
		{Document: SearchDocument{TenantID: tenantID, Access: AccessPolicy{Visibility: VisibilityPublic}}},
		{Document: SearchDocument{TenantID: tenantID, Access: AccessPolicy{Visibility: VisibilityPublic}}},
	}

	filtered := evaluator.FilterAuthorized(context.Background(), req, hits)

	require.Len(t, filtered, 2)
	require.Equal(t, 0, resolver.calls)
}
