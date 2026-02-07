package eval_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/eval"
	"github.com/stretchr/testify/require"
)

func TestLoadSuite_RejectsLegacySchema(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.json")
	require.NoError(t, os.WriteFile(path, []byte(`[
		{"id":"legacy_1","question":"What is revenue?","expected_sql":"SELECT 1"}
	]`), 0o644))

	_, err := eval.LoadSuite(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot unmarshal array")
}

func TestLoadSuite_ValidatesRequiredFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"tests":[
			{"id":"tc1","turns":[{"prompt":"hello"}]}
		]
	}`), 0o644))

	_, err := eval.LoadSuite(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dataset_id is required")
}

func TestLoadSuite_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "suite.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"tests":[
			{
				"id":"tc1",
				"dataset_id":"analytics_baseline_v1",
				"category":"analytics",
				"tags":["eval"],
				"turns":[{"prompt":"What is Q1 income?","oracle_refs":["analytics_baseline_v1.q1_total_income_minor"]}],
				"expect":{"forbidden":false,"redirect_unauth":false,"sse_error":false}
			}
		]
	}`), 0o644))

	suite, err := eval.LoadSuite(path)
	require.NoError(t, err)
	require.Len(t, suite.Tests, 1)
	require.Equal(t, "tc1", suite.Tests[0].ID)
	require.Equal(t, "analytics_baseline_v1", suite.Tests[0].DatasetID)
}
