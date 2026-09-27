package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRejectsEmptyConnectionString(t *testing.T) {
	t.Parallel()

	ds, err := New(Config{})
	require.Nil(t, ds)
	require.Error(t, err)
	require.ErrorContains(t, err, "connection string is required")
}

func TestApplyMaxRowsWrapsQuery(t *testing.T) {
	t.Parallel()

	require.Equal(t, "SELECT 1", applyMaxRows("SELECT 1", 0))
	require.Equal(t, "SELECT * FROM (SELECT 1) AS lens_query LIMIT 25", applyMaxRows("SELECT 1", 25))
	require.Equal(t, "SELECT * FROM (SELECT 1) AS lens_query LIMIT 25", applyMaxRows(" SELECT 1; ", 25))
}

func TestValidateQueryAllowsIdentifiersContainingForbiddenWords(t *testing.T) {
	t.Parallel()

	err := validateQuery("SELECT update_log, delete_requests FROM audit_report")
	require.NoError(t, err)
}

func TestValidateQueryAllowsIntoIdentifierOutsideSelectInto(t *testing.T) {
	t.Parallel()

	err := validateQuery("WITH into AS (SELECT 1 AS value) SELECT value FROM into")
	require.NoError(t, err)
}

func TestValidateQueryRejectsSelectInto(t *testing.T) {
	t.Parallel()

	err := validateQuery("SELECT * INTO backup_table FROM contracts")
	require.Error(t, err)
}

func TestValidateQueryRejectsCommentSplitWriteKeywords(t *testing.T) {
	t.Parallel()

	err := validateQuery("SE/* hidden */LECT 1; DE/* hidden */LETE FROM users")
	require.Error(t, err)
}

func TestValidateParamsRequiresConfiguredParams(t *testing.T) {
	t.Parallel()

	err := validateParams(map[string]any{
		"tenant_id": "tenant-1",
	}, []string{"tenant_id"})
	require.NoError(t, err)

	err = validateParams(map[string]any{}, []string{"tenant_id"})
	require.Error(t, err)
	require.ErrorContains(t, err, "tenant_id")
}
