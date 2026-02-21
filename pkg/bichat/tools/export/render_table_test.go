package export

import (
	"context"
	"encoding/json"
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRenderTableExecutor struct {
	result      *bichatsql.QueryResult
	err         error
	executedSQL string
}

func (m *mockRenderTableExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	m.executedSQL = sql
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &bichatsql.QueryResult{}, nil
}

func TestRenderTableTool_Name(t *testing.T) {
	t.Parallel()

	tool := NewRenderTableTool(&mockRenderTableExecutor{})
	assert.Equal(t, "render_table", tool.Name())
}

func TestRenderTableTool_Call_SuccessWithExport(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	executor := &mockRenderTableExecutor{
		result: &bichatsql.QueryResult{
			Columns: []string{"policy_id", "premium_amount"},
			Rows: [][]any{
				{int64(1), 1000.0},
				{int64(2), 2500.0},
			},
			RowCount: 2,
		},
	}

	tool := NewRenderTableTool(
		executor,
		WithRenderTableOutputDir(tmpDir),
		WithRenderTableBaseURL("http://localhost:8080/static/exports"),
	)

	raw, err := tool.Call(context.Background(), `{
		"sql":"SELECT policy_id, premium_amount FROM analytics.policies_with_details",
		"title":"Policy Premiums",
		"headers":["Policy ID", "Premium (UZS)"]
	}`)
	require.NoError(t, err)

	var out renderTableOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &out))

	assert.Equal(t, "Policy Premiums", out.Title)
	assert.Equal(t, "SELECT policy_id, premium_amount FROM analytics.policies_with_details", out.Query)
	assert.Equal(t, []string{"policy_id", "premium_amount"}, out.Columns)
	assert.Equal(t, []string{"Policy ID", "Premium (UZS)"}, out.Headers)
	require.Len(t, out.Rows, 2)
	assert.False(t, out.Truncated)
	require.NotNil(t, out.Export)
	assert.Contains(t, out.Export.Filename, "render_table_")
	assert.Equal(t, 2, out.Export.RowCount)
	assert.Contains(t, out.Export.URL, "http://localhost:8080/static/exports/render_table_")
	assert.NotEmpty(t, out.ExportPrompt)

	_, statErr := os.Stat(filepath.Join(tmpDir, out.Export.Filename))
	assert.NoError(t, statErr)
}

func TestRenderTableTool_CallStructured_PolicyViolation(t *testing.T) {
	t.Parallel()

	tool := NewRenderTableTool(&mockRenderTableExecutor{}).(*RenderTableTool)

	result, err := tool.CallStructured(context.Background(), `{"sql":"UPDATE analytics.policies_with_details SET premium_amount = 1"}`)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, types.CodecToolError, result.CodecID)

	payload, ok := result.Payload.(types.ToolErrorPayload)
	require.True(t, ok)
	assert.Equal(t, string(tools.ErrCodePolicyViolation), payload.Code)
	assert.Contains(t, strings.ToUpper(payload.Message), "SELECT")
}

func TestRenderTableTool_CallStructured_TruncatedByExecutor(t *testing.T) {
	t.Parallel()

	executor := &mockRenderTableExecutor{
		result: &bichatsql.QueryResult{
			Columns: []string{"id"},
			Rows:    [][]any{{int64(1)}},
			// executor indicates truncation via metadata flag
			Truncated: true,
		},
	}
	tool := NewRenderTableTool(executor).(*RenderTableTool)

	result, err := tool.CallStructured(context.Background(), `{"sql":"SELECT 1"}`)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, types.CodecJSON, result.CodecID)

	var out renderTableOutput
	require.NoError(t, json.Unmarshal([]byte(result.Payload.(types.JSONPayload).Output.(string)), &out))
	assert.True(t, out.Truncated)
	assert.Equal(t, "executor_cap", out.TruncatedReason)
}

func TestRenderTableTool_Call_NoOutputDirStillReturnsPrompt(t *testing.T) {
	t.Parallel()

	executor := &mockRenderTableExecutor{
		result: &bichatsql.QueryResult{
			Columns:  []string{"id"},
			Rows:     [][]any{{int64(1)}},
			RowCount: 1,
		},
	}

	tool := NewRenderTableTool(executor)

	raw, err := tool.Call(context.Background(), `{"sql":"SELECT id FROM analytics.clients_with_policies"}`)
	require.NoError(t, err)

	var out renderTableOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	require.Nil(t, out.Export)
	assert.NotEmpty(t, out.ExportPrompt)
}
