package serrors_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestCodeOf_ClassificationPrecedence(t *testing.T) {
	t.Parallel()
	missing := serrors.NewNotFound("private")
	tests := []struct {
		name string
		err  error
		code serrors.Code
	}{
		{"nil", nil, serrors.Internal},
		{"unknown", errors.New("SQL secret"), serrors.Internal},
		{"wrapped", fmt.Errorf("outer: %w", missing), serrors.NotFound},
		{"unclassified operation", serrors.Wrap("repo.List", missing), serrors.NotFound},
		{"explicit internal", serrors.NewInternal("").WithCause(missing), serrors.Internal},
		{"explicit invalid", serrors.NewInvalid("").WithCause(serrors.NewNotFound("")), serrors.Invalid},
		{"join unclassified first", errors.Join(errors.New("unknown"), missing), serrors.NotFound},
		{"join unclassified frame first", errors.Join(serrors.Wrap("first", errors.New("unknown")), missing), serrors.NotFound},
		{"join conflicting", errors.Join(missing, serrors.NewInvalid("")), serrors.NotFound},
		{"join explicit internal first", errors.Join(serrors.NewInternal(""), missing), serrors.Internal},
		{"cancel", fmt.Errorf("cancel: %w", context.Canceled), serrors.Canceled},
		{"deadline", serrors.Wrap("call", context.DeadlineExceeded), serrors.Timeout},
		{"outer cancellation override", serrors.NewUnavailable("").WithCause(context.Canceled), serrors.Unavailable},
		{"join cancellation", errors.Join(errors.New("unknown"), context.Canceled, missing), serrors.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.code, serrors.CodeOf(tt.err))
			require.Equal(t, tt.err != nil, serrors.HasCode(tt.err, tt.code))
			if tt.code != serrors.NotFound {
				require.False(t, serrors.HasCode(tt.err, serrors.NotFound))
			}
		})
	}
}

func TestIdentity_MultiAndForeignWrappers(t *testing.T) {
	t.Parallel()
	first := errors.New("first")
	second := &pgconn.PgError{Code: "23505", ConstraintName: "owned"}
	err := serrors.Wrap("service.Create", serrors.Multi(serrors.NewConflict("").WithCause(first), fmt.Errorf("foreign: %w", second)))
	require.ErrorIs(t, err, first)
	require.ErrorIs(t, err, second)
	var pg *pgconn.PgError
	require.ErrorAs(t, err, &pg)
	require.Same(t, second, pg)
	firstConflict, secondConflict := serrors.NewConflict(""), serrors.NewConflict("")
	require.NotErrorIs(t, firstConflict, secondConflict)
	require.NoError(t, serrors.Wrap("nil", nil))
	require.NoError(t, serrors.Multi(nil, nil))
	require.Equal(t, []serrors.Op{"service.Create"}, serrors.Trace(err))
}

func TestFromDB_PreservesCause(t *testing.T) {
	t.Parallel()
	domain := serrors.NewInvalid("domain failure").WithCause(sql.ErrNoRows)
	require.Equal(t, serrors.Invalid, serrors.CodeOf(serrors.FromDB("repo.Save", domain)))
	require.ErrorIs(t, serrors.FromDB("repo.Save", domain), domain)
	for _, tt := range []struct {
		name string
		err  error
		code serrors.Code
	}{
		{"sql no rows", sql.ErrNoRows, serrors.NotFound},
		{"pgx no rows", pgx.ErrNoRows, serrors.NotFound},
		{"cancel", context.Canceled, serrors.Canceled},
		{"deadline", context.DeadlineExceeded, serrors.Timeout},
		{"unknown unique", &pgconn.PgError{Code: "23505", ConstraintName: "unknown", Detail: "private SQL"}, serrors.Internal},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := serrors.FromDB("repo.Insert", fmt.Errorf("driver: %w", tt.err))
			require.Equal(t, tt.code, serrors.CodeOf(err))
			require.ErrorIs(t, err, tt.err)
			require.Equal(t, serrors.Op("repo.Insert"), serrors.OpOf(err))
			require.NotContains(t, serrors.Public(err, nil).Message, "private SQL")
		})
	}
	require.NoError(t, serrors.FromDB("repo.Insert", nil))
}

func TestFromConstraint_OnlyOwnedConstraints(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		state string
		code  serrors.Code
	}{
		{"23505", serrors.AlreadyExists}, {"23503", serrors.Conflict}, {"23502", serrors.Invalid}, {"23514", serrors.Invalid},
	} {
		t.Run(tt.state, func(t *testing.T) {
			t.Parallel()
			cause := &pgconn.PgError{Code: tt.state, ConstraintName: "owned", Message: "SQL secret", Detail: "private"}
			rule := serrors.Constraint{SQLState: tt.state, Name: "owned", Message: serrors.Message{ID: "allowed"}}
			err := serrors.FromConstraint("repo.Create", cause, rule)
			require.Equal(t, tt.code, serrors.CodeOf(err))
			require.ErrorIs(t, err, cause)
			require.NotContains(t, serrors.Public(err, nil).Message, "SQL")
			rule.Name = "other"
			require.Equal(t, serrors.Internal, serrors.CodeOf(serrors.FromConstraint("repo.Create", cause, rule)))
			rule.Name = ""
			require.Equal(t, serrors.Internal, serrors.CodeOf(serrors.FromConstraint("repo.Create", cause, rule)))
		})
	}
	require.NoError(t, serrors.FromConstraint("nil", nil))
	missingName := &pgconn.PgError{Code: "23502", TableName: "products", ColumnName: "name", Detail: "private row"}
	rule := serrors.Constraint{SQLState: "23502", Table: "products", Column: "name"}
	err := serrors.FromConstraint("products.Create", missingName, rule)
	require.Equal(t, serrors.Invalid, serrors.CodeOf(err))
	require.ErrorIs(t, err, missingName)
	require.NotContains(t, serrors.Public(err, nil).Message, "private row")
	rule.Table = "other"
	require.Equal(t, serrors.Internal, serrors.CodeOf(serrors.FromConstraint("products.Create", missingName, rule)))
}

func TestImmutableBuilders_NestedMessagesAndSharedSentinel(t *testing.T) {
	t.Parallel()
	bundle := i18n.NewBundle(language.English)
	require.NoError(t, bundle.AddMessages(language.English, &i18n.Message{ID: "field", Other: "Name"}, &i18n.Message{ID: "invalid", Other: "{{.Field}} {{.Count}}"}))
	l := i18n.NewLocalizer(bundle, "en")
	count := int64(2)
	args := map[string]serrors.Value{"Field": serrors.Reference(serrors.Message{ID: "field"}), "Count": serrors.Number(2)}
	message := serrors.Message{ID: "invalid", Args: args, Count: &count}
	fields := []serrors.FieldViolation{{Field: "Name", Message: message}}
	sentinel := serrors.NewInvalid("private").WithPublic(message).WithFields(fields...).WithMeta(args)
	args["Count"] = serrors.Number(99)
	count = 99
	fields[0].Field = "Mutated"
	require.Equal(t, "Name 2", serrors.Public(sentinel, l).Message)
	require.Contains(t, serrors.FieldMap(sentinel, l), "Name")
	returned := serrors.MessageOf(sentinel)
	returned.Args["Count"] = serrors.Number(77)
	*returned.Count = 77
	returnedFields := serrors.FieldsOf(sentinel)
	returnedFields[0].Message.Args["Count"] = serrors.Number(77)
	require.Equal(t, "Name 2", serrors.Public(sentinel, l).Fields[0].Message)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sentinel.WithPublic(serrors.Message{ID: "other"}).WithFields().WithMeta(nil)
		}()
	}
	wg.Wait()
	require.Equal(t, "Name 2", serrors.Public(sentinel, l).Message)
	require.Len(t, serrors.FieldsOf(sentinel), 1)
}

func TestPublic_OuterClassificationBlocksInnerDisclosure(t *testing.T) {
	t.Parallel()
	inner := serrors.NewInvalid("SQL secret").WithReason("allowed").WithPublic(serrors.Message{ID: "missing"}).WithFields(serrors.FieldViolation{Field: "Name"})
	outer := serrors.NewInternal("path secret").WithCause(inner)
	p := serrors.Public(outer, nil)
	require.Empty(t, p.Reason)
	require.Empty(t, p.Fields)
	require.NotContains(t, p.Message, "secret")
	require.Empty(t, serrors.FieldMap(outer, nil))
	wrapped := serrors.WrapContext("outer", inner, "private context")
	require.Equal(t, serrors.Invalid, serrors.CodeOf(wrapped))
	require.Equal(t, "allowed", string(serrors.Public(wrapped, nil).Reason))
	require.Contains(t, serrors.FieldMap(wrapped, nil), "Name")
	require.ErrorIs(t, wrapped, inner)
	require.NoError(t, serrors.WrapContext("nil", nil, "private context"))
}

func ExampleWrap() {
	missing := serrors.NewNotFound("repository lookup failed")
	err := serrors.Wrap("orders.Get", missing)
	fmt.Println(serrors.CodeOf(err), errors.Is(err, missing))
	// Output: not_found true
}

func ExampleFromConstraint() {
	cause := &pgconn.PgError{Code: "23505", ConstraintName: "products_sku_key"}
	err := serrors.FromConstraint("products.Create", cause, serrors.Constraint{
		SQLState: "23505", Name: "products_sku_key", Reason: "duplicate_sku",
		Message: serrors.Message{ID: "Products.Errors.DuplicateSKU"},
	})
	fmt.Println(serrors.CodeOf(err), errors.Is(err, cause))
	// Output: already_exists true
}
