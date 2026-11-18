package host

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// ErrSubjectBlocked is matched by every SubjectLoader refusal: the user is
// disabled, deleted, or may no longer act. Triggers and workflow instances
// turn it into a `blocked` outcome with a diagnostic; it is never a failure of
// the business logic itself.
var ErrSubjectBlocked = errors.New("extruntime: subject cannot act")

// SubjectBlockedError is the typed diagnostic of a refused subject.
type SubjectBlockedError struct {
	TenantID uuid.UUID
	UserID   int64
	Reason   string
}

func (e *SubjectBlockedError) Error() string {
	return fmt.Sprintf("extruntime: user %d of tenant %s cannot act: %s", e.UserID, e.TenantID, e.Reason)
}

func (e *SubjectBlockedError) Is(target error) bool { return target == ErrSubjectBlocked }

// SubjectLoader resolves the *current* rights of a user. Events, schedules
// and workflow runs call it for every delivery or run, so a disabled user or a
// revoked permission blocks new actions while session expiry is irrelevant.
// Implementations return a *SubjectBlockedError (or wrap ErrSubjectBlocked)
// when the user may not act.
type SubjectLoader interface {
	LoadSubject(ctx context.Context, tenantID uuid.UUID, userID int64, kind spi.SubjectKind) (spi.Subject, error)
}

// SubjectLoaderFunc adapts a function to SubjectLoader.
type SubjectLoaderFunc func(ctx context.Context, tenantID uuid.UUID, userID int64, kind spi.SubjectKind) (spi.Subject, error)

func (f SubjectLoaderFunc) LoadSubject(ctx context.Context, tenantID uuid.UUID, userID int64, kind spi.SubjectKind) (spi.Subject, error) {
	return f(ctx, tenantID, userID, kind)
}
