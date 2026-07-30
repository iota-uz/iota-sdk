package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeProviderErrorKnownCodes(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"credit_balance_exhausted":             ProviderBillingBalanceExhaustedError,
		"organization_spend_limit_exceeded":    ProviderBillingOrgSpendLimitError,
		"project_spend_limit_exceeded":         ProviderBillingProjectSpendLimitError,
		"organization_usage_limit_exceeded":    ProviderBillingOrgUsageLimitError,
		"insufficient_quota":                   ProviderBillingQuotaError,
		"rate_limit_exceeded":                  ProviderRateLimitedError,
		"invalid_api_key":                      ProviderAuthInvalidError,
		"organization_membership_required":     ProviderAuthMembershipError,
		"ip_not_authorized":                    ProviderIPNotAuthorizedError,
		"permission_denied":                    ProviderPermissionDeniedError,
		"unsupported_country_region_territory": ProviderRegionUnsupportedError,
		"context_length_exceeded":              ProviderContextLimitError,
		"model_not_found":                      ProviderNotFoundError,
		"conflict":                             ProviderConflictError,
		"unprocessable_entity":                 ProviderUnprocessableError,
		"request_timeout":                      ProviderTimeoutError,
		"api_connection_error":                 ProviderConnectionError,
		"server_error":                         ProviderServerError,
		"overloaded":                           ProviderOverloadedError,
		"slow_down":                            ProviderSlowDownError,
		"response_incomplete":                  ProviderResponseIncompleteError,
		"response_failed":                      ProviderResponseFailedError,
		"invalid_request_error":                ProviderBadRequestError,
	}

	for code, expected := range tests {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, expected, NormalizeProviderError(code, ""))
		})
	}
}

func TestNormalizeProviderErrorSafeFallbacks(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ProviderTimeoutError, NormalizeProviderError("", "request deadline exceeded"))
	assert.Equal(t, ProviderConnectionError, NormalizeProviderError("", "TLS handshake failed"))
	assert.Empty(t, NormalizeProviderError("future_provider_code", "sensitive upstream detail"))
}
