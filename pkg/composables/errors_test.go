package composables

import (
	"context"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAuthErrorsRetainSentinelsAndClassification(t *testing.T) {
	_, err := UseUser(context.Background())
	require.ErrorIs(t, serrors.Wrap("user", err), ErrNoUserFound)
	require.Equal(t, serrors.Unauthenticated, serrors.CodeOf(err))
	denied := serrors.Wrap("policy", ErrForbidden)
	require.ErrorIs(t, denied, ErrForbidden)
	require.Equal(t, serrors.PermissionDenied, serrors.CodeOf(denied))
}
