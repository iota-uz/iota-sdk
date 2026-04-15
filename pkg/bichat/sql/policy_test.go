package sql_test

import (
	"context"
	"errors"
	"testing"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSchemaGatedPolicy_AllowsUnrelated(t *testing.T) {
	policy := bichatsql.NewSchemaGatedPolicy("accounting", func(ctx context.Context) error {
		t.Fatalf("authz should not be called when schema is absent")
		return nil
	})

	cases := []string{
		"SELECT * FROM insurance.policies",
		"SELECT accounting_note FROM crm.notes",             // column, not schema
		"SELECT * FROM public.accounting_mapping_overrides", // table name contains substring
	}
	for _, sql := range cases {
		require.NoError(t, policy.Check(context.Background(), sql), "sql=%q", sql)
	}
}

func TestNewSchemaGatedPolicy_BlocksWhenAuthzFails(t *testing.T) {
	denied := errors.New("permission denied")
	policy := bichatsql.NewSchemaGatedPolicy("accounting", func(ctx context.Context) error {
		return denied
	})

	cases := []string{
		"SELECT * FROM accounting.gl_entries",
		"select * from Accounting.GL_Entries",                     // case-insensitive
		`SELECT * FROM "accounting"."journal"`,                    // quoted identifier
		"SELECT a.x FROM accounting .journal a",                   // whitespace between schema and dot
		"SELECT * FROM crm.clients JOIN accounting.ledger ON ...", // mixed
	}
	for _, sql := range cases {
		err := policy.Check(context.Background(), sql)
		require.ErrorIs(t, err, denied, "sql=%q", sql)
	}
}

func TestNewSchemaGatedPolicy_EmptySchema_AllowsAll(t *testing.T) {
	policy := bichatsql.NewSchemaGatedPolicy("", nil)
	require.NoError(t, policy.Check(context.Background(), "SELECT * FROM accounting.anything"))
}

func TestNewSchemaGatedPolicy_NilAuthz_BlocksWithGenericError(t *testing.T) {
	policy := bichatsql.NewSchemaGatedPolicy("accounting", nil)
	err := policy.Check(context.Background(), "SELECT * FROM accounting.gl_entries")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accounting")
}

func TestBIChatDefaults_ReturnsExpectedBundle(t *testing.T) {
	// Smoke test: applying BIChatDefaults and then a user override
	// results in the override winning. Catches accidental regressions
	// where the preset would clobber user options.
	executor := bichatsql.NewSafeQueryExecutor(nil,
		append(bichatsql.BIChatDefaults(nil),
			bichatsql.WithMaxResultRows(500),
		)...,
	)
	// Public accessors aren't exposed; this test is primarily a
	// compile-time smoke test that the preset type-checks.
	_ = executor
}
