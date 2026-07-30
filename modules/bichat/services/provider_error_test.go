package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeProviderError_KnownCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code     string
		expected string
	}{
		{"credit_balance_exhausted", ProviderBillingBalanceExhaustedError},
		{"organization_spend_limit_exceeded", ProviderBillingOrgSpendLimitError},
		{"project_spend_limit_exceeded", ProviderBillingProjectSpendLimitError},
		{"organization_usage_limit_exceeded", ProviderBillingOrgUsageLimitError},
		{"insufficient_quota", ProviderBillingQuotaError},
		{"rate_limit_exceeded", ProviderRateLimitedError},
		{"invalid_api_key", ProviderAuthInvalidError},
		{"organization_membership_required", ProviderAuthMembershipError},
		{"ip_not_authorized", ProviderIPNotAuthorizedError},
		{"permission_denied", ProviderPermissionDeniedError},
		{"unsupported_country_region_territory", ProviderRegionUnsupportedError},
		{"context_length_exceeded", ProviderContextLimitError},
		{"model_not_found", ProviderNotFoundError},
		{"conflict", ProviderConflictError},
		{"unprocessable_entity", ProviderUnprocessableError},
		{"request_timeout", ProviderTimeoutError},
		{"api_connection_error", ProviderConnectionError},
		{"server_error", ProviderServerError},
		{"overloaded", ProviderOverloadedError},
		{"slow_down", ProviderSlowDownError},
		{"response_incomplete", ProviderResponseIncompleteError},
		{"response_failed", ProviderResponseFailedError},
		{"invalid_request_error", ProviderBadRequestError},
	}

	require.NotEmpty(t, tests)
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, NormalizeProviderError(tt.code, ""))
		})
	}
}

func TestNormalizeProviderError_SafeFallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		code     string
		raw      string
		expected string
	}{
		{"timeout", "", "request deadline exceeded", ProviderTimeoutError},
		{"connection", "", "TLS handshake failed", ProviderConnectionError},
		{"unknown", "future_provider_code", "sensitive upstream detail", ""},
	}

	require.Len(t, tests, 3)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, NormalizeProviderError(tt.code, tt.raw))
		})
	}
}

func TestParseProviderStreamError_ExtractsCodeAndMessage(t *testing.T) {
	t.Parallel()

	code, message, ok := ParseProviderStreamError(
		`provider failure: {"type":"server_error","code":"permission_denied","message":"Forbidden"}`,
	)

	require.True(t, ok)
	assert.Equal(t, "permission_denied", code)
	assert.Equal(t, "Forbidden", message)
}
