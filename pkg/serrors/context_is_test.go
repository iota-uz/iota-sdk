package serrors_test

import (
	"context"
	"database/sql"
	"errors"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
	"testing"
)

type contextMatcherError struct{ sentinel error }

func (e contextMatcherError) Error() string { return "private driver diagnostic" }
func (e contextMatcherError) Is(target error) bool {
	return target == e.sentinel || target == sql.ErrNoRows
}

func TestContextIsOnlyFallbackDoesNotOverrideExplicitClassification(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sentinel error
		code     serrors.Code
	}{
		{"cancellation", context.Canceled, serrors.Canceled},
		{"deadline", context.DeadlineExceeded, serrors.Timeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := contextMatcherError{sentinel: tc.sentinel}
			require.ErrorIs(t, original, tc.sentinel)
			require.ErrorIs(t, original, sql.ErrNoRows)
			require.Equal(t, tc.code, serrors.CodeOf(original))
			for _, mapped := range []error{serrors.FromDB("repo.Find", original), serrors.FromConstraint("repo.Save", original)} {
				require.Equal(t, tc.code, serrors.CodeOf(mapped))
				require.ErrorIs(t, mapped, original)
				require.NotContains(t, serrors.Public(mapped, nil).Message, "private driver")
			}
			require.Equal(t, serrors.Internal, serrors.CodeOf(serrors.NewInternal("outer").WithCause(original)))
			require.Equal(t, serrors.NotFound, serrors.CodeOf(errors.Join(original, serrors.NewNotFound("explicit"))))
			require.Equal(t, serrors.Timeout, serrors.CodeOf(errors.Join(original, context.DeadlineExceeded)))
		})
	}
}
