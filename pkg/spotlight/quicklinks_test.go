package spotlight

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewQuickLink_HasPublicAccess(t *testing.T) {
	link := NewQuickLink("test.key", "/test/path")

	require.Equal(t, "test.key", link.trKey)
	require.Equal(t, "/test/path", link.link)
	require.Equal(t, VisibilityPublic, link.access.Visibility)
	require.Empty(t, link.keywords)
}

func TestQuickLinkBuilder_WithPermissions(t *testing.T) {
	link := NewQuickLinkBuilder("finance.expenses", "/finance/expenses").
		WithPermissions("finance.expenses.view", "finance.expenses.read").
		Build()

	require.Equal(t, VisibilityRestricted, link.access.Visibility)
	require.Equal(t, []string{"finance.expenses.view", "finance.expenses.read"}, link.access.AllowedPermissions)
}

func TestQuickLinkBuilder_WithRoles(t *testing.T) {
	link := NewQuickLinkBuilder("admin.users", "/admin/users").
		WithRoles("admin", "superuser").
		Build()

	require.Equal(t, VisibilityRestricted, link.access.Visibility)
	require.Equal(t, []string{"admin", "superuser"}, link.access.AllowedRoles)
}

func TestQuickLinkBuilder_WithUsers(t *testing.T) {
	link := NewQuickLinkBuilder("private.link", "/private").
		WithUsers("user-1", "user-2").
		Build()

	require.Equal(t, VisibilityRestricted, link.access.Visibility)
	require.Equal(t, []string{"user-1", "user-2"}, link.access.AllowedUsers)
}

func TestQuickLinkBuilder_WithOwner(t *testing.T) {
	link := NewQuickLinkBuilder("owner.only", "/owner").
		WithOwner("owner-42").
		Build()

	require.Equal(t, VisibilityOwner, link.access.Visibility)
	require.Equal(t, "owner-42", link.access.OwnerID)
}

func TestQuickLinkBuilder_Public(t *testing.T) {
	link := NewQuickLinkBuilder("public.link", "/public").
		Public().
		Build()

	require.Equal(t, VisibilityPublic, link.access.Visibility)
}

func TestQuickLinkBuilder_WithKeywords(t *testing.T) {
	link := NewQuickLinkBuilder("finance.expenses", "/finance/expenses").
		Public().
		WithKeywords("costs", "spending", "money out").
		Build()

	require.Equal(t, []string{"costs", "spending", "money out"}, link.keywords)
}

func TestQuickLinkBuilder_WithKeywords_Appends(t *testing.T) {
	link := NewQuickLinkBuilder("finance.expenses", "/finance/expenses").
		Public().
		WithKeywords("costs", "spending").
		WithKeywords("money out", "expenditure").
		Build()

	require.Equal(t, []string{"costs", "spending", "money out", "expenditure"}, link.keywords)
}

func TestQuickLinkBuilder_WithAccess(t *testing.T) {
	customAccess := AccessPolicy{
		Visibility:         VisibilityRestricted,
		AllowedRoles:       []string{"admin"},
		AllowedPermissions: []string{"read"},
		AllowedUsers:       []string{"user-1"},
	}

	link := NewQuickLinkBuilder("custom.link", "/custom").
		WithAccess(customAccess).
		Build()

	require.Equal(t, customAccess, link.access)
}

func TestQuickLinkBuilder_ChainedCalls(t *testing.T) {
	link := NewQuickLinkBuilder("finance.link", "/finance").
		WithPermissions("finance.view").
		WithRoles("accountant").
		WithKeywords("money", "accounting").
		Build()

	require.Equal(t, VisibilityRestricted, link.access.Visibility)
	require.Equal(t, []string{"finance.view"}, link.access.AllowedPermissions)
	require.Equal(t, []string{"accountant"}, link.access.AllowedRoles)
	require.Equal(t, []string{"money", "accounting"}, link.keywords)
}

func TestQuickLinks_ListDocuments_UsesConfiguredAccess(t *testing.T) {
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLink("public.link", "/public"),
		NewQuickLinkBuilder("restricted.link", "/restricted").
			WithPermissions("admin.view").
			Build(),
		NewQuickLinkBuilder("role.link", "/role").
			WithRoles("manager").
			Build(),
	)

	tenantID := uuid.New()
	docs, err := ql.ListDocuments(context.Background(), ProviderScope{
		TenantID: tenantID,
		Language: "en",
	})

	require.NoError(t, err)
	require.Len(t, docs, 3)

	// First document: public
	require.Equal(t, VisibilityPublic, docs[0].Access.Visibility)

	// Second document: restricted by permission
	require.Equal(t, VisibilityRestricted, docs[1].Access.Visibility)
	require.Equal(t, []string{"admin.view"}, docs[1].Access.AllowedPermissions)

	// Third document: restricted by role
	require.Equal(t, VisibilityRestricted, docs[2].Access.Visibility)
	require.Equal(t, []string{"manager"}, docs[2].Access.AllowedRoles)
}

func TestQuickLinks_ListDocuments_IncludesKeywordsInBody(t *testing.T) {
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLinkBuilder("expenses.link", "/expenses").
			Public().
			WithKeywords("costs", "spending").
			Build(),
	)

	docs, err := ql.ListDocuments(context.Background(), ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	})

	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Contains(t, docs[0].Body, "costs")
	require.Contains(t, docs[0].Body, "spending")
}

func TestQuickLinks_ListDocuments_NoKeywords(t *testing.T) {
	ql := NewQuickLinks(nil, nil)
	ql.Add(NewQuickLink("simple.link", "/simple"))

	docs, err := ql.ListDocuments(context.Background(), ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	})

	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, "simple.link", docs[0].Body)
}

func TestQuickLinks_FilterAuthorized_RestrictedByPermission(t *testing.T) {
	tenantID := uuid.New()
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLink("public.link", "/public"),
		NewQuickLinkBuilder("restricted.link", "/restricted").
			WithPermissions("finance.view").
			Build(),
	)

	docs, err := ql.ListDocuments(context.Background(), ProviderScope{
		TenantID: tenantID,
		Language: "en",
	})
	require.NoError(t, err)

	hits := make([]SearchHit, len(docs))
	for i, doc := range docs {
		hits[i] = SearchHit{Document: doc}
	}

	principalWithPermission := Principal{
		UserID:      "1",
		Permissions: []string{"finance.view"},
	}
	principalWithoutPermission := Principal{
		UserID:      "2",
		Permissions: []string{"other.permission"},
	}

	// User with permission can see both
	for _, hit := range hits {
		if hit.Document.Access.Visibility == VisibilityPublic {
			require.True(t, canReadPolicy(hit.Document.Access, principalWithPermission))
		} else {
			require.True(t, canReadPolicy(hit.Document.Access, principalWithPermission))
		}
	}

	// User without permission can only see public
	for _, hit := range hits {
		if hit.Document.Access.Visibility == VisibilityPublic {
			require.True(t, canReadPolicy(hit.Document.Access, principalWithoutPermission))
		} else {
			require.False(t, canReadPolicy(hit.Document.Access, principalWithoutPermission))
		}
	}
}

func TestQuickLinks_FilterAuthorized_RestrictedByRole(t *testing.T) {
	tenantID := uuid.New()
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLinkBuilder("admin.link", "/admin").
			WithRoles("admin").
			Build(),
	)

	docs, err := ql.ListDocuments(context.Background(), ProviderScope{
		TenantID: tenantID,
		Language: "en",
	})
	require.NoError(t, err)
	require.Len(t, docs, 1)

	adminPrincipal := Principal{UserID: "1", Roles: []string{"admin"}}
	userPrincipal := Principal{UserID: "2", Roles: []string{"user"}}

	require.True(t, canReadPolicy(docs[0].Access, adminPrincipal))
	require.False(t, canReadPolicy(docs[0].Access, userPrincipal))
}

func TestQuickLinkBuilder_DefaultRestrictedVisibility(t *testing.T) {
	link := NewQuickLinkBuilder("test.key", "/test").Build()

	require.Equal(t, VisibilityRestricted, link.access.Visibility)
	require.Empty(t, link.access.AllowedPermissions)
	require.Empty(t, link.access.AllowedRoles)
	require.Empty(t, link.access.AllowedUsers)
}
