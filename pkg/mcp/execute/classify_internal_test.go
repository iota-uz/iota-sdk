package execute

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/stretchr/testify/assert"
)

func TestClassify_TerminalReasonIsAuthoritative(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		err    *pykernel.ExecError
		done   *pykernel.DonePayload
		status string
		code   string
	}{
		{"error lost to backpressure", nil, &pykernel.DonePayload{Reason: pykernel.ReasonError,
			Error: &pykernel.ExecError{Type: "ValueError", Message: "boom"}}, StatusFailed, ErrorPython},
		{"error lost without details", nil, &pykernel.DonePayload{Reason: pykernel.ReasonError}, StatusFailed, ErrorPython},
		{"swallowed timeout", nil, &pykernel.DonePayload{Reason: pykernel.ReasonTimeout}, StatusTimedOut, ErrorTimeout},
		{"quota", &pykernel.ExecError{Type: "OSError"}, &pykernel.DonePayload{Reason: pykernel.ReasonQuotaExceeded}, StatusFailed, ErrorLimitExceeded},
		{"kernel died", nil, &pykernel.DonePayload{Reason: pykernel.ReasonKernelExited}, StatusFailed, ErrorRuntimeLost},
		{"completed", nil, &pykernel.DonePayload{Reason: pykernel.ReasonCompleted}, StatusSucceeded, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, runErr := classify(ctx, tc.err, tc.done, false)
			assert.Equal(t, tc.status, status)
			if tc.code == "" {
				assert.Nil(t, runErr)
			} else if assert.NotNil(t, runErr) {
				assert.Equal(t, tc.code, runErr.Code)
			}
		})
	}
	_, runErr := classify(ctx, nil, &pykernel.DonePayload{Reason: pykernel.ReasonError,
		Error: &pykernel.ExecError{Type: "ValueError", Message: "boom"}}, false)
	assert.Contains(t, runErr.Message, "boom", "structured details survive backpressure")
}
