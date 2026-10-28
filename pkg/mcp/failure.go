package mcp

import (
	"encoding/json"
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type Code string

const (
	CodeInvalidInput          Code = "invalid_input"
	CodeNotFound              Code = "not_found"
	CodeForbidden             Code = "forbidden"
	CodeConflict              Code = "conflict"
	CodeApprovalRequired      Code = "approval_required"
	CodePlanExpired           Code = "plan_expired"
	CodeLimitExceeded         Code = "limit_exceeded"
	CodeBusy                  Code = "busy"
	CodeDependencyUnavailable Code = "dependency_unavailable"
)

type Recovery string

const (
	RecoveryFixInput        Recovery = "fix_input"
	RecoveryReauthenticate  Recovery = "reauthenticate"
	RecoveryRequestAccess   Recovery = "request_access"
	RecoveryReadThenPrepare Recovery = "read_then_prepare"
	RecoveryOpenApproval    Recovery = "open_approval"
	RecoveryCheckOperation  Recovery = "check_operation"
	RecoveryRetryLater      Recovery = "retry_later"
	RecoveryNone            Recovery = "none"
)

type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Failure is the only error shape a tool caller ever sees. Handlers return it
// (or an serrors error, which is mapped onto it) to report an expected outcome.
type Failure struct { //nolint:errname // named after the public contract type
	Code         Code     `json:"code"`
	Message      string   `json:"message"`
	Issues       []Issue  `json:"issues"`
	Recovery     Recovery `json:"recovery"`
	RetryAfterMS int64    `json:"retry_after_ms,omitempty"`
	OperationID  string   `json:"operation_id,omitempty"`
}

func Fail(code Code, recovery Recovery, message string, issues ...Issue) *Failure {
	return &Failure{Code: code, Message: message, Issues: issues, Recovery: recovery}
}

func (f *Failure) Error() string { return string(f.Code) + ": " + f.Message }

func (f *Failure) WithRetryAfter(ms int64) *Failure {
	c := *f
	c.RetryAfterMS = ms
	return &c
}

func (f *Failure) WithOperation(id string) *Failure {
	c := *f
	c.OperationID = id
	return &c
}

func (f Failure) MarshalJSON() ([]byte, error) {
	type plain Failure
	if f.Issues == nil {
		f.Issues = []Issue{}
	}
	return json.Marshal(plain(f))
}

// FailureFrom maps an error onto the public contract. Only public messages
// cross the boundary: internal diagnostics may carry SQL or secrets.
func FailureFrom(err error) *Failure {
	var f *Failure
	if errors.As(err, &f) {
		return f
	}
	message := serrors.MessageOf(err).Text
	issues := make([]Issue, 0)
	for _, v := range serrors.FieldsOf(err) {
		issues = append(issues, Issue{Path: v.Field, Code: string(v.Reason), Message: v.Message.Text})
	}
	code, recovery, fallback := CodeDependencyUnavailable, RecoveryRetryLater, "the operation could not be completed"
	switch serrors.CodeOf(err) {
	case serrors.Invalid:
		code, recovery, fallback = CodeInvalidInput, RecoveryFixInput, "invalid input"
	case serrors.NotFound:
		code, recovery, fallback = CodeNotFound, RecoveryFixInput, "not found"
	case serrors.PermissionDenied:
		code, recovery, fallback = CodeForbidden, RecoveryRequestAccess, "access denied"
	case serrors.Unauthenticated:
		code, recovery, fallback = CodeForbidden, RecoveryReauthenticate, "authentication required"
	case serrors.AlreadyExists, serrors.Conflict, serrors.FailedPrecondition:
		code, recovery, fallback = CodeConflict, RecoveryReadThenPrepare, "the data changed or conflicts with the request"
	case serrors.RateLimited:
		code, recovery, fallback = CodeLimitExceeded, RecoveryRetryLater, "rate limit exceeded"
	case serrors.Unavailable, serrors.Timeout, serrors.Canceled, serrors.Unimplemented, serrors.Internal:
	}
	if message == "" {
		message = fallback
	}
	return &Failure{Code: code, Message: message, Issues: issues, Recovery: recovery}
}
