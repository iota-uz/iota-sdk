package composables

import "github.com/iota-uz/iota-sdk/pkg/serrors"

var (
	ErrInvalidPassword error = serrors.NewUnauthenticated("invalid password")
	ErrNotFound        error = serrors.NewNotFound("not found")
	ErrUnauthorized    error = serrors.NewUnauthenticated("unauthorized")
	ErrForbidden       error = serrors.NewPermissionDenied("forbidden")
	ErrInternal        error = serrors.NewInternal("internal error")
)
