package testenv

import (
	"context"
	"errors"
)

func lifecycleError(err error, code, operation string, d Descriptor, cleanup error) *Error {
	var typed *Error
	result := &Error{Code: code, Message: err.Error()}
	if errors.As(err, &typed) {
		*result = *typed
		result.Causes = append([]*Error(nil), typed.Causes...)
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		result.Code = "timeout"
	}
	result.Operation, result.EnvironmentID, result.ArtifactDirectory = operation, d.EnvironmentID, d.ArtifactDirectory
	result.cause = errors.Join(err, cleanup)
	if cleanup != nil {
		result.Causes = append(result.Causes, lifecycleError(cleanup, "cleanup_failed", "stop", d, nil))
	}
	return result
}
