package spotlight

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestExpandExactTermsSplitsMultilineValues(t *testing.T) {
	terms := ExpandExactTerms("AA1234567\nBB7654321", "+998 90 123-45-67\n+998 90 123-45-67")

	require.Contains(t, terms, "AA1234567")
	require.Contains(t, terms, "BB7654321")
	require.Contains(t, terms, "+998 90 123-45-67")
	require.Contains(t, terms, "998901234567")
	require.NotContains(t, terms, "AA1234567\nBB7654321")
}

func TestExpandExactTermsDoesNotAddDigitsOnlyVariantForMixedIdentifiers(t *testing.T) {
	terms := ExpandExactTerms("30833WAA")

	require.Contains(t, terms, "30833WAA")
	require.Contains(t, terms, "30833waa")
	require.NotContains(t, terms, "30833")
}

func TestPlanRequest_ClassifiesLookupQueries(t *testing.T) {
	req := planRequest(SearchRequest{
		Query:    "AA 1234567",
		TenantID: uuid.New(),
	})

	require.Equal(t, QueryModeLookup, req.Mode)
	require.Contains(t, req.PreferredDomains, ResultDomainLookup)
	require.Contains(t, req.ExactTerms, "AA1234567")
}

func TestPlanRequest_ClassifiesNavigationQueries(t *testing.T) {
	req := planRequest(SearchRequest{
		Query:    "/crm/clients",
		TenantID: uuid.New(),
	})

	require.Equal(t, QueryModeNavigate, req.Mode)
	require.Equal(t, []ResultDomain{ResultDomainNavigate}, req.PreferredDomains)
}

func TestPlanRequest_PreservesExplicitMode(t *testing.T) {
	req := planRequest(SearchRequest{
		Query:            "help",
		TenantID:         uuid.New(),
		Mode:             QueryModeLookup,
		PreferredDomains: []ResultDomain{ResultDomainAction},
	})

	require.Equal(t, QueryModeLookup, req.Mode)
	require.Equal(t, []ResultDomain{ResultDomainAction}, req.PreferredDomains)
}
