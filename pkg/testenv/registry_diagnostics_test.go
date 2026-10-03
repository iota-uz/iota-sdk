package testenv

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// Falsely green if compensation is never invoked, or only the secondary error is observed.
func TestPrepareRetainsPrimaryAndCompensationFailures(t *testing.T) {
	primary := errors.New("partial scenario")
	typed := &Error{Code: "missing_capability", Message: "scenario prerequisite", cause: primary}
	for _, tc := range []struct {
		name    string
		primary error
		code    string
	}{
		{"ordinary", primary, "execution_failed"}, {"typed", typed, "missing_capability"}, {"deadline", context.DeadlineExceeded, "timeout"},
	} {
		for _, fails := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "/clean", true: "/cleanup-fails"}[fails], func(t *testing.T) {
				r := NewEnvironmentRegistry(nil, true, "env")
				cleanup := errors.New("compensation failure")
				calls := 0
				require.NoError(t, r.Register(testDefinition(), func(context.Context, Input) (Result, error) { return Result{}, tc.primary }, func(context.Context, string) error {
					calls++
					if fails {
						return cleanup
					}
					return nil
				}))
				require.NoError(t, r.AllowScope("scope"))
				_, err := r.Prepare(t.Context(), testInput("scope"))
				var diagnostic *Error
				require.ErrorAs(t, err, &diagnostic)
				require.Equal(t, tc.code, diagnostic.Code)
				require.ErrorIs(t, err, tc.primary)
				require.Equal(t, "prepare", diagnostic.Operation)
				require.Equal(t, "env", diagnostic.EnvironmentID)
				require.Equal(t, 1, calls)
				if fails {
					require.ErrorIs(t, err, cleanup)
					require.Len(t, diagnostic.Causes, 1)
					require.Equal(t, "cleanup_failed", diagnostic.Causes[0].Code)
					require.Equal(t, "dispose", diagnostic.Causes[0].Operation)
				} else {
					require.Empty(t, diagnostic.Causes)
				}
			})
		}
	}
}

// Falsely green if a direct registry call replaces the credential-protected mounted HTTP boundary.
func TestPrepareHTTPRetainsPrimaryStatusAndSecondaryCleanup(t *testing.T) {
	r := NewEnvironmentRegistry(nil, true, "env")
	require.NoError(t, r.Register(testDefinition(), func(context.Context, Input) (Result, error) {
		return Result{}, &Error{Code: "missing_capability", Message: "required backend"}
	}, func(context.Context, string) error { return errors.New("compensation failure") }))
	require.NoError(t, r.AllowScope("scope"))
	handler, err := NewHandler(r, testToken)
	require.NoError(t, err)
	response := request(t, handler, "POST", "/__test__/scenarios/prepare", testInput("scope"), testToken)
	require.Equal(t, 412, response.Code)
	var diagnostic Error
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &diagnostic))
	require.Equal(t, "missing_capability", diagnostic.Code)
	require.Len(t, diagnostic.Causes, 1)
	require.Equal(t, "cleanup_failed", diagnostic.Causes[0].Code)
	require.Equal(t, "dispose", diagnostic.Causes[0].Operation)
}
