package serrors_test

import (
	"context"
	"database/sql"
	"errors"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDatabaseMappingPreservesFirstContextClassification(t *testing.T) {
	for _, sentinel := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			original := errors.Join(sentinel, sql.ErrNoRows)
			mapped := serrors.FromDB("repo.Find", original)
			require.Equal(t, serrors.CodeOf(original), serrors.CodeOf(mapped))
			require.ErrorIs(t, mapped, sentinel)
			require.ErrorIs(t, mapped, sql.ErrNoRows)
			driver := &pgconn.PgError{Code: "23505", ConstraintName: "owned_key"}
			original = errors.Join(sentinel, driver)
			mapped = serrors.FromConstraint("repo.Insert", original, serrors.Constraint{SQLState: "23505", Name: "owned_key", Message: serrors.Message{Text: "owned public conflict"}})
			require.Equal(t, serrors.CodeOf(original), serrors.CodeOf(mapped))
			require.ErrorIs(t, mapped, sentinel)
			require.ErrorIs(t, mapped, driver)
			require.NotContains(t, serrors.Public(mapped, nil).Message, "owned public conflict")
		})
	}
}
