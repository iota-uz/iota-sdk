package serrors_test

import (
	"context"
	"database/sql"
	"errors"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFromDBContextClassificationAndPrivateDiagnostic(t *testing.T) {
	explicit := serrors.NewInvalid("private validation").WithPublic(serrors.Message{Text: "approved input"})
	for _, tc := range []struct {
		name string
		err  error
		code serrors.Code
	}{
		{"no rows", sql.ErrNoRows, serrors.NotFound},
		{"canceled", errors.Join(context.Canceled, sql.ErrNoRows), serrors.Canceled},
		{"timeout", context.DeadlineExceeded, serrors.Timeout},
		{"explicit", explicit, serrors.Invalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mapped := serrors.FromDBContext("repo.Find", tc.err, "private SQL diagnostic")
			require.Equal(t, tc.code, serrors.CodeOf(mapped))
			require.ErrorIs(t, mapped, tc.err)
			require.Equal(t, []serrors.Op{"repo.Find"}, serrors.Trace(mapped))
			require.Contains(t, mapped.Error(), "private SQL diagnostic")
			require.NotContains(t, serrors.Public(mapped, nil).Message, "private SQL diagnostic")
			if tc.err == explicit {
				require.Equal(t, "approved input", serrors.Public(mapped, nil).Message)
			}
		})
	}
	require.NoError(t, serrors.FromDBContext("repo.Find", nil, "private SQL diagnostic"))
}
