package document_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
)

type stubNetError struct{ timeout bool }

func (s stubNetError) Error() string { return "dial failed" }
func (s stubNetError) Timeout() bool { return s.timeout }
func (s stubNetError) Temporary() bool {
	return false
}

func TestClassifyFailure(t *testing.T) {
	t.Parallel()

	var netTimeout net.Error = stubNetError{timeout: true}
	var netOther net.Error = stubNetError{}

	for name, testCase := range map[string]struct {
		err      error
		expected document.FailureReason
	}{
		"no error":             {err: nil, expected: ""},
		"wrapped deadline":     {err: fmt.Errorf("analytics.repository.trends: %w", context.DeadlineExceeded), expected: document.FailureTimeout},
		"wrapped cancellation": {err: fmt.Errorf("analytics.repository.trends: %w", context.Canceled), expected: document.FailureCanceled},
		"net timeout":          {err: fmt.Errorf("datasource: %w", netTimeout), expected: document.FailureTimeout},
		"net failure":          {err: fmt.Errorf("datasource: %w", netOther), expected: document.FailureUnknown},
		"statement timeout":    {err: errors.New("ERROR: canceling statement due to statement timeout (SQLSTATE 57014)"), expected: document.FailureTimeout},
		"i/o timeout":          {err: errors.New("dial tcp 10.0.0.1:5432: i/o timeout"), expected: document.FailureTimeout},
		"unrelated failure":    {err: errors.New("column \"premium\" does not exist"), expected: document.FailureUnknown},
		// The word "timeout" appearing in an error is not itself a deadline —
		// a setup mistake that happens to name the word must not be told to
		// the reader as "pick a shorter period", which would send them
		// chasing a symptom that was never there.
		"unrelated config error naming timeout": {err: errors.New("invalid timeout configuration: must be positive"), expected: document.FailureUnknown},
		"missing dataset":      {err: fmt.Errorf("dataset %q not found", "trends"), expected: document.FailureUnknown},
		// A chain is free to lose its sentinel on the way up: the EAI trends
		// panel fails through a memoizer that reports what it cached as text,
		// so the deadline arrives spelled out rather than wrapped.
		"stringified deadline": {
			err:      errors.New("lens/runtime.MemoizeJSON: analytics.repository.trends: timeout: context deadline exceeded"),
			expected: document.FailureTimeout,
		},
		"stringified cancellation": {
			err:      errors.New("lens/runtime.MemoizeJSON: analytics.repository.trends: context canceled"),
			expected: document.FailureCanceled,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, testCase.expected, document.ClassifyFailure(testCase.err))
		})
	}
}
