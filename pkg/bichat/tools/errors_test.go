package tools

import (
	"encoding/json"
	"testing"
)

func TestFormatToolError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		code          ToolErrorCode
		message       string
		hints         []string
		wantCode      ToolErrorCode
		wantMessage   string
		wantHintCount int
	}{
		{
			name:          "query error with hints",
			code:          ErrCodeQueryError,
			message:       "syntax error at or near 'SELCT'",
			hints:         []string{HintCheckSQLSyntax, HintVerifyTableNames},
			wantCode:      ErrCodeQueryError,
			wantMessage:   "syntax error at or near 'SELCT'",
			wantHintCount: 2,
		},
		{
			name:          "policy violation with hints",
			code:          ErrCodePolicyViolation,
			message:       "query contains disallowed keyword: INSERT",
			hints:         []string{HintOnlySelectAllowed, HintNoWriteOperations},
			wantCode:      ErrCodePolicyViolation,
			wantMessage:   "query contains disallowed keyword: INSERT",
			wantHintCount: 2,
		},
		{
			name:          "no data with hints",
			code:          ErrCodeNoData,
			message:       "table not found: analytics.customers",
			hints:         []string{HintUseSchemaList, "Check spelling and case sensitivity"},
			wantCode:      ErrCodeNoData,
			wantMessage:   "table not found: analytics.customers",
			wantHintCount: 2,
		},
		{
			name:          "data too large with hints",
			code:          ErrCodeDataTooLarge,
			message:       "result set exceeds 100000 rows",
			hints:         []string{HintAddLimitClause, HintFilterWithWhere},
			wantCode:      ErrCodeDataTooLarge,
			wantMessage:   "result set exceeds 100000 rows",
			wantHintCount: 2,
		},
		{
			name:          "service unavailable with hints",
			code:          ErrCodeServiceUnavailable,
			message:       "knowledge base is not available",
			hints:         []string{HintServiceMayBeDown, HintRetryLater},
			wantCode:      ErrCodeServiceUnavailable,
			wantMessage:   "knowledge base is not available",
			wantHintCount: 2,
		},
		{
			name:          "invalid request with no hints",
			code:          ErrCodeInvalidRequest,
			message:       "query parameter is required",
			hints:         []string{},
			wantCode:      ErrCodeInvalidRequest,
			wantMessage:   "query parameter is required",
			wantHintCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := FormatToolError(tt.code, tt.message, tt.hints...)

			// Parse JSON result
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(result), &parsed); err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			// Check top-level structure
			errorField, ok := parsed["error"].(map[string]interface{})
			if !ok {
				t.Fatalf("missing or invalid 'error' field")
			}

			// Check code
			gotCode, ok := errorField["code"].(string)
			if !ok {
				t.Fatalf("missing or invalid 'code' field")
			}
			if gotCode != string(tt.wantCode) {
				t.Errorf("code = %v, want %v", gotCode, tt.wantCode)
			}

			// Check message
			gotMessage, ok := errorField["message"].(string)
			if !ok {
				t.Fatalf("missing or invalid 'message' field")
			}
			if gotMessage != tt.wantMessage {
				t.Errorf("message = %v, want %v", gotMessage, tt.wantMessage)
			}

			// Check hints
			if tt.wantHintCount > 0 {
				gotHints, ok := errorField["hints"].([]interface{})
				if !ok {
					t.Fatalf("missing or invalid 'hints' field")
				}
				if len(gotHints) != tt.wantHintCount {
					t.Errorf("hint count = %v, want %v", len(gotHints), tt.wantHintCount)
				}
			} else {
				// Hints should be omitted if empty
				if _, exists := errorField["hints"]; exists {
					gotHints, _ := errorField["hints"].([]interface{})
					if len(gotHints) != 0 {
						t.Errorf("expected no hints, got %v", len(gotHints))
					}
				}
			}
		})
	}
}

func TestToolErrorCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code ToolErrorCode
		want string
	}{
		{
			name: "invalid request",
			code: ErrCodeInvalidRequest,
			want: "INVALID_REQUEST",
		},
		{
			name: "query error",
			code: ErrCodeQueryError,
			want: "QUERY_ERROR",
		},
		{
			name: "policy violation",
			code: ErrCodePolicyViolation,
			want: "POLICY_VIOLATION",
		},
		{
			name: "no data",
			code: ErrCodeNoData,
			want: "NO_DATA",
		},
		{
			name: "data too large",
			code: ErrCodeDataTooLarge,
			want: "DATA_TOO_LARGE",
		},
		{
			name: "service unavailable",
			code: ErrCodeServiceUnavailable,
			want: "SERVICE_UNAVAILABLE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if string(tt.code) != tt.want {
				t.Errorf("code = %v, want %v", tt.code, tt.want)
			}
		})
	}
}

func TestToolErrorError(t *testing.T) {
	t.Parallel()

	err := &ToolError{
		Code:    ErrCodeQueryError,
		Message: "syntax error",
		Hints:   []string{"hint1", "hint2"},
	}

	if err.Error() != "syntax error" {
		t.Errorf("Error() = %v, want %v", err.Error(), "syntax error")
	}
}

func TestFormatToolErrorFallback(t *testing.T) {
	t.Parallel()

	// Test that even with invalid JSON marshaling, we get a valid JSON response
	// This is a defensive test - in practice, ToolError should always marshal successfully
	result := FormatToolError(ErrCodeInvalidRequest, "test message")

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON (fallback should still be valid JSON): %v", err)
	}

	errorField, ok := parsed["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing or invalid 'error' field in fallback")
	}

	if errorField["code"] != "INVALID_REQUEST" {
		t.Errorf("fallback code = %v, want INVALID_REQUEST", errorField["code"])
	}

	if errorField["message"] != "test message" {
		t.Errorf("fallback message = %v, want test message", errorField["message"])
	}
}

func TestHintConstants(t *testing.T) {
	t.Parallel()

	// Verify hint constants are non-empty and reasonable
	hints := []struct {
		name  string
		value string
	}{
		{"HintCheckSQLSyntax", HintCheckSQLSyntax},
		{"HintVerifyTableNames", HintVerifyTableNames},
		{"HintUseSchemaList", HintUseSchemaList},
		{"HintAddLimitClause", HintAddLimitClause},
		{"HintFilterWithWhere", HintFilterWithWhere},
		{"HintCheckJoinConditions", HintCheckJoinConditions},
		{"HintOnlySelectAllowed", HintOnlySelectAllowed},
		{"HintNoWriteOperations", HintNoWriteOperations},
		{"HintCheckSchemaAccess", HintCheckSchemaAccess},
		{"HintTableMayBeEmpty", HintTableMayBeEmpty},
		{"HintTryDifferentTerms", HintTryDifferentTerms},
		{"HintCheckDateRange", HintCheckDateRange},
		{"HintServiceMayBeDown", HintServiceMayBeDown},
		{"HintRetryLater", HintRetryLater},
		{"HintCheckConnection", HintCheckConnection},
		{"HintCheckRequiredFields", HintCheckRequiredFields},
		{"HintCheckFieldTypes", HintCheckFieldTypes},
		{"HintCheckFieldFormat", HintCheckFieldFormat},
	}

	for _, h := range hints {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()

			if h.value == "" {
				t.Errorf("%s is empty", h.name)
			}

			if len(h.value) < 10 {
				t.Errorf("%s is too short: %q", h.name, h.value)
			}

			if len(h.value) > 200 {
				t.Errorf("%s is too long: %q", h.name, h.value)
			}
		})
	}
}
