package spotlight

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
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
	docs, err := CollectDocuments(context.Background(), ql, ProviderScope{
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

	docs, err := CollectDocuments(context.Background(), ql, ProviderScope{
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

	docs, err := CollectDocuments(context.Background(), ql, ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	})

	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, "simple.link", docs[0].Body)
}

func TestQuickLinks_ListDocuments_FallsBackToBundleLanguages(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	bundle.MustAddMessages(language.English, &i18n.Message{ID: "NavigationLinks.Users", Other: "Users"})
	bundle.MustAddMessages(language.Russian, &i18n.Message{ID: "NavigationLinks.Users", Other: "Пользователи"})

	ql := NewQuickLinks(bundle, nil)
	ql.Add(NewQuickLink("NavigationLinks.Users", "/users"))

	docs, err := CollectDocuments(context.Background(), ql, ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	})

	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Contains(t, docs[0].Body, "Users")
	require.Contains(t, docs[0].Body, "Пользователи")
}

func TestQuickLinks_Add_MergesDuplicateLinks(t *testing.T) {
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLinkBuilder("nav.users", "/users").
			Public().
			WithKeywords("users", "people").
			Build(),
	)
	ql.Add(
		NewQuickLinkBuilder("nav.users", "/users").
			WithPermissions("users.read").
			WithKeywords("staff", "directory").
			Build(),
	)

	docs, err := CollectDocuments(context.Background(), ql, ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	})

	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, VisibilityRestricted, docs[0].Access.Visibility)
	require.Equal(t, []string{"users.read"}, docs[0].Access.AllowedPermissions)
	require.Contains(t, docs[0].Body, "users")
	require.Contains(t, docs[0].Body, "people")
	require.Contains(t, docs[0].Body, "staff")
	require.Contains(t, docs[0].Body, "directory")
	require.Contains(t, docs[0].ExactTerms, "users")
	require.Contains(t, docs[0].ExactTerms, "people")
	require.Contains(t, docs[0].ExactTerms, "staff")
	require.Contains(t, docs[0].ExactTerms, "directory")
}

func TestQuickLinks_Add_MergesAccessMonotonically(t *testing.T) {
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLinkBuilder("owner.link", "/owner").
			WithOwner("42").
			Build(),
	)
	ql.Add(
		NewQuickLinkBuilder("owner.link", "/owner").
			WithPermissions("users.read").
			Build(),
	)

	docs, err := CollectDocuments(context.Background(), ql, ProviderScope{
		TenantID: uuid.New(),
		Language: "en",
	})

	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, VisibilityOwner, docs[0].Access.Visibility)
	require.Equal(t, "42", docs[0].Access.OwnerID)
	require.Equal(t, []string{"users.read"}, docs[0].Access.AllowedPermissions)
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

	docs, err := CollectDocuments(context.Background(), ql, ProviderScope{
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

	// User with permission can see both public and restricted
	for _, hit := range hits {
		require.True(t, canReadPolicy(hit.Document.Access, principalWithPermission),
			"user with permission should be able to read %s visibility", hit.Document.Access.Visibility)
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

	docs, err := CollectDocuments(context.Background(), ql, ProviderScope{
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

// ---------------------------------------------------------------------------
// FuzzySearch tests
// ---------------------------------------------------------------------------

func TestFuzzySearch_ExactSubstringRanksHigherThanFuzzy(t *testing.T) {
	tenantID := uuid.New()
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLinkBuilder("nav.settings", "/settings").
			Public().
			WithKeywords("settings", "preferences").
			Build(),
		NewQuickLinkBuilder("nav.sessions", "/sessions").
			Public().
			WithKeywords("sessions", "logins").
			Build(),
		// "settingz" is 1-edit away from "settings" — fuzzy match only
		NewQuickLinkBuilder("nav.settingz", "/settingz").
			Public().
			WithKeywords("settingz").
			Build(),
	)

	req := SearchRequest{
		TenantID: tenantID,
		UserID:   "1",
		Query:    "settings",
	}

	results := ql.FuzzySearch("settings", req)
	require.NotEmpty(t, results)

	// The item with exact keyword "settings" should rank first
	require.Equal(t, "/settings", results[0].Document.URL)

	// Fuzzy match "settingz" should appear but with lower score
	found := false
	for _, r := range results {
		if r.Document.URL == "/settingz" {
			found = true
			require.Less(t, r.FinalScore, results[0].FinalScore,
				"fuzzy match should have a lower score than exact match")
		}
	}
	require.True(t, found, "fuzzy match for 'settingz' should be present in results")
}

func TestFuzzySearch_RBACFiltering(t *testing.T) {
	tenantID := uuid.New()
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLinkBuilder("nav.public", "/public").
			Public().
			WithKeywords("dashboard").
			Build(),
		NewQuickLinkBuilder("nav.admin", "/admin").
			WithPermissions("admin.access").
			WithKeywords("dashboard").
			Build(),
	)

	// User without admin.access should only see public link
	reqNoAdmin := SearchRequest{
		TenantID:    tenantID,
		UserID:      "user-1",
		Permissions: []string{"basic.view"},
	}
	results := ql.FuzzySearch("dashboard", reqNoAdmin)
	require.Len(t, results, 1)
	require.Equal(t, "/public", results[0].Document.URL)

	// User with admin.access should see both
	reqAdmin := SearchRequest{
		TenantID:    tenantID,
		UserID:      "user-2",
		Permissions: []string{"admin.access"},
	}
	results = ql.FuzzySearch("dashboard", reqAdmin)
	require.Len(t, results, 2)
}

func TestFuzzySearch_EmptyQueryReturnsEmpty(t *testing.T) {
	ql := NewQuickLinks(nil, nil)
	ql.Add(NewQuickLink("nav.home", "/home"))

	req := SearchRequest{TenantID: uuid.New(), UserID: "1"}
	require.Empty(t, ql.FuzzySearch("", req))
	require.Empty(t, ql.FuzzySearch("   ", req))
}

func TestFuzzySearch_ResultsCappedAtEight(t *testing.T) {
	ql := NewQuickLinks(nil, nil)
	// Add 12 public links that all match "page"
	for i := 0; i < 12; i++ {
		ql.Add(
			NewQuickLinkBuilder("nav.page"+string(rune('a'+i)), "/page/"+string(rune('a'+i))).
				Public().
				WithKeywords("page").
				Build(),
		)
	}

	req := SearchRequest{TenantID: uuid.New(), UserID: "1"}
	results := ql.FuzzySearch("page", req)
	require.Len(t, results, 8, "results should be capped at 8")
}

func TestFuzzySearch_DocumentShape(t *testing.T) {
	tenantID := uuid.New()
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLinkBuilder("nav.users", "/users").
			Public().
			WithKeywords("staff", "people").
			Build(),
	)

	req := SearchRequest{TenantID: tenantID, UserID: "1", Language: "en"}
	results := ql.FuzzySearch("users", req)
	require.Len(t, results, 1)

	doc := results[0].Document
	require.Equal(t, "core.quick_links", doc.Provider)
	require.Equal(t, "quick_link", doc.EntityType)
	require.Equal(t, ResultDomainNavigate, doc.Domain)
	require.Equal(t, "/users", doc.URL)
	require.Equal(t, tenantID, doc.TenantID)
	require.Equal(t, "en", doc.Language)
	require.Contains(t, doc.Metadata, "tr_key")
	require.Contains(t, doc.Metadata, "source")
	require.Empty(t, results[0].WhyMatched)
}

func TestQuickLinks_RestrictedNoConfigFiltersAll(t *testing.T) {
	tenantID := uuid.New()
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLinkBuilder("restricted.no.config", "/restricted").Build(),
	)

	docs, err := CollectDocuments(context.Background(), ql, ProviderScope{
		TenantID: tenantID,
		Language: "en",
	})
	require.NoError(t, err)
	require.Len(t, docs, 1)

	require.Equal(t, VisibilityRestricted, docs[0].Access.Visibility)
	require.Empty(t, docs[0].Access.AllowedPermissions)
	require.Empty(t, docs[0].Access.AllowedRoles)
	require.Empty(t, docs[0].Access.AllowedUsers)

	adminPrincipal := Principal{UserID: "1", Roles: []string{"admin"}, Permissions: []string{"all"}}
	regularPrincipal := Principal{UserID: "2", Roles: []string{"user"}, Permissions: []string{"read"}}
	emptyPrincipal := Principal{UserID: "3"}

	require.False(t, canReadPolicy(docs[0].Access, adminPrincipal),
		"restricted link with no allowed config should be inaccessible even to admin")
	require.False(t, canReadPolicy(docs[0].Access, regularPrincipal),
		"restricted link with no allowed config should be inaccessible to regular user")
	require.False(t, canReadPolicy(docs[0].Access, emptyPrincipal),
		"restricted link with no allowed config should be inaccessible to user with no roles/permissions")
}

// ---------------------------------------------------------------------------
// scoreSingle tests
// ---------------------------------------------------------------------------

func TestScoreSingle_WordPrefixBeatsContains(t *testing.T) {
	// "sett" is a prefix of word "settings" → should get word-prefix score (0.95)
	// not the lower contains score (0.8)
	score := scoreSingle("sett", "settings page")
	require.InDelta(t, fuzzyScoreWordPrefix, score, 0.001,
		"word-level prefix should rank higher than substring contains")
}

func TestScoreSingle_NoFalseFullPhrasePrefix(t *testing.T) {
	// "ha" should get word-prefix score (0.95) via word "hamkorlik",
	// NOT the old 1.0 from full-phrase HasPrefix (which is removed).
	score := scoreSingle("ha", "hamkorlik dasturi")
	require.InDelta(t, fuzzyScoreWordPrefix, score, 0.001,
		"short prefix should get word-prefix score")
}

func TestScoreSingle_SubstringNotPrefix(t *testing.T) {
	// "ett" appears inside "settings" but is not a word prefix
	score := scoreSingle("ett", "settings")
	require.InDelta(t, fuzzyScoreContains, score, 0.001,
		"mid-word substring should get contains score")
}

func TestScoreSingle_NoMatch(t *testing.T) {
	score := scoreSingle("xyz", "settings")
	require.InDelta(t, 0.0, score, 0.001, "completely unrelated words should score 0")
}

func TestScoreSingle_ExactWordMatch(t *testing.T) {
	// "settings" is a full word match → word-prefix with full word
	score := scoreSingle("settings", "settings")
	require.InDelta(t, fuzzyScoreWordPrefix, score, 0.001)
}

// ---------------------------------------------------------------------------
// bestFuzzyScore multi-word coverage tests
// ---------------------------------------------------------------------------

func activeFragments(texts ...string) []scoredFragment {
	frags := make([]scoredFragment, len(texts))
	for i, t := range texts {
		frags[i] = scoredFragment{text: t, activeLanguage: true}
	}
	return frags
}

func TestBestFuzzyScore_MultiWordCoverage(t *testing.T) {
	t.Run("single word unchanged", func(t *testing.T) {
		score := bestFuzzyScore(
			[]string{"settings"},
			activeFragments("settings page"),
		)
		require.InDelta(t, fuzzyScoreWordPrefix, score, 0.001)
	})

	t.Run("multi-word full coverage ranks higher than partial", func(t *testing.T) {
		full := bestFuzzyScore(
			[]string{"john", "settings"},
			activeFragments("john", "settings"),
		)
		partial := bestFuzzyScore(
			[]string{"john", "settings"},
			activeFragments("settings"),
		)
		require.Greater(t, full, partial,
			"matching all query words should score higher than matching only one")
	})

	t.Run("partial match averages in zero for unmatched word", func(t *testing.T) {
		score := bestFuzzyScore(
			[]string{"john", "settings"},
			activeFragments("settings"),
		)
		expected := (0.0 + fuzzyScoreWordPrefix) / 2.0
		require.InDelta(t, expected, score, 0.001)
	})

	t.Run("empty query returns zero", func(t *testing.T) {
		score := bestFuzzyScore([]string{}, activeFragments("settings"))
		require.InDelta(t, 0.0, score, 0.001)
	})

	t.Run("cross-language fragment scores discounted", func(t *testing.T) {
		active := bestFuzzyScore(
			[]string{"settings"},
			[]scoredFragment{{text: "settings", activeLanguage: true}},
		)
		cross := bestFuzzyScore(
			[]string{"settings"},
			[]scoredFragment{{text: "settings", activeLanguage: false}},
		)
		require.Greater(t, active, cross,
			"active language match should score higher than cross-language match")
		require.InDelta(t, active*crossLanguageDiscount, cross, 0.001)
	})
}

// ---------------------------------------------------------------------------
// FuzzySearch integration test for short query
// ---------------------------------------------------------------------------

func TestFuzzySearch_ShortQueryNoFalseTopRank(t *testing.T) {
	ql := NewQuickLinks(nil, nil)
	ql.Add(
		NewQuickLinkBuilder("nav.hamkorlik", "/hamkorlik").
			Public().
			WithKeywords("hamkorlik dasturi").
			Build(),
		NewQuickLinkBuilder("nav.haggle", "/haggle").
			Public().
			WithKeywords("haggle").
			Build(),
	)

	req := SearchRequest{TenantID: uuid.New(), UserID: "1"}
	results := ql.FuzzySearch("ha", req)
	require.NotEmpty(t, results)

	// Both should match with word-prefix score (0.95), not the old 1.0
	for _, r := range results {
		require.InDelta(t, fuzzyScoreWordPrefix, r.FinalScore, 0.001,
			"short 2-char query should get word-prefix score")
	}
}
