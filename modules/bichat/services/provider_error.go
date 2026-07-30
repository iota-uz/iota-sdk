package services

import "strings"

const (
	ProviderBillingBalanceExhaustedError  = "provider_billing_balance_exhausted"
	ProviderBillingOrgSpendLimitError     = "provider_billing_org_spend_limit"
	ProviderBillingProjectSpendLimitError = "provider_billing_project_spend_limit"
	ProviderBillingOrgUsageLimitError     = "provider_billing_org_usage_limit"
	ProviderBillingQuotaError             = "provider_billing_quota"
	ProviderRateLimitedError              = "provider_rate_limited"
	ProviderAuthInvalidError              = "provider_auth_invalid"
	ProviderAuthMembershipError           = "provider_auth_membership_required"
	ProviderIPNotAuthorizedError          = "provider_ip_not_authorized"
	ProviderPermissionDeniedError         = "provider_permission_denied"
	ProviderRegionUnsupportedError        = "provider_region_unsupported"
	ProviderBadRequestError               = "provider_bad_request"
	ProviderContextLimitError             = "provider_context_limit"
	ProviderNotFoundError                 = "provider_not_found"
	ProviderConflictError                 = "provider_conflict"
	ProviderUnprocessableError            = "provider_unprocessable"
	ProviderTimeoutError                  = "provider_timeout"
	ProviderConnectionError               = "provider_connection"
	ProviderServerError                   = "provider_server_error"
	ProviderOverloadedError               = "provider_overloaded"
	ProviderSlowDownError                 = "provider_slow_down"
	ProviderResponseIncompleteError       = "provider_response_incomplete"
	ProviderResponseFailedError           = "provider_response_failed"
)

func NormalizeProviderError(code, raw string) string {
	normalizedCode := strings.ToLower(strings.TrimSpace(code))
	switch normalizedCode {
	case "credit_balance_exhausted":
		return ProviderBillingBalanceExhaustedError
	case "organization_spend_limit_exceeded":
		return ProviderBillingOrgSpendLimitError
	case "project_spend_limit_exceeded":
		return ProviderBillingProjectSpendLimitError
	case "organization_usage_limit_exceeded":
		return ProviderBillingOrgUsageLimitError
	case "insufficient_quota":
		return ProviderBillingQuotaError
	case "rate_limit_exceeded", "rate_limit":
		return ProviderRateLimitedError
	case "invalid_api_key", "authentication_error", "auth_error":
		return ProviderAuthInvalidError
	case "organization_membership_required":
		return ProviderAuthMembershipError
	case "ip_not_authorized":
		return ProviderIPNotAuthorizedError
	case "permission_denied", "forbidden":
		return ProviderPermissionDeniedError
	case "unsupported_country_region_territory", "region_unsupported":
		return ProviderRegionUnsupportedError
	case "context_length_exceeded", "context_window_exceeded":
		return ProviderContextLimitError
	case "model_not_found", "not_found":
		return ProviderNotFoundError
	case "conflict":
		return ProviderConflictError
	case "unprocessable_entity":
		return ProviderUnprocessableError
	case "request_timeout", "timeout":
		return ProviderTimeoutError
	case "api_connection_error", "connection_error":
		return ProviderConnectionError
	case "server_error", "internal_error":
		return ProviderServerError
	case "overloaded", "server_overloaded":
		return ProviderOverloadedError
	case "slow_down":
		return ProviderSlowDownError
	case "response_incomplete":
		return ProviderResponseIncompleteError
	case "response_failed":
		return ProviderResponseFailedError
	case "invalid_request_error", "invalid_request", "bad_request":
		return ProviderBadRequestError
	}

	normalizedRaw := strings.ToLower(raw)
	for marker, canonical := range map[string]string{
		"credit_balance_exhausted":          ProviderBillingBalanceExhaustedError,
		"organization_spend_limit_exceeded": ProviderBillingOrgSpendLimitError,
		"project_spend_limit_exceeded":      ProviderBillingProjectSpendLimitError,
		"organization_usage_limit_exceeded": ProviderBillingOrgUsageLimitError,
		"insufficient_quota":                ProviderBillingQuotaError,
		"rate_limit_exceeded":               ProviderRateLimitedError,
		"invalid_api_key":                   ProviderAuthInvalidError,
		"context_length_exceeded":           ProviderContextLimitError,
		"response_incomplete":               ProviderResponseIncompleteError,
		"response_failed":                   ProviderResponseFailedError,
	} {
		if strings.Contains(normalizedRaw, marker) {
			return canonical
		}
	}

	switch {
	case strings.Contains(normalizedRaw, "deadline exceeded"),
		strings.Contains(normalizedRaw, "timed out"),
		strings.Contains(normalizedRaw, "timeout"):
		return ProviderTimeoutError
	case strings.Contains(normalizedRaw, "connection refused"),
		strings.Contains(normalizedRaw, "connection reset"),
		strings.Contains(normalizedRaw, "no such host"),
		strings.Contains(normalizedRaw, "tls handshake"):
		return ProviderConnectionError
	default:
		return ""
	}
}
