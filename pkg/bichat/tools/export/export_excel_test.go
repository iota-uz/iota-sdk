package export

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportToExcelTool_CallStructured_EmitsArtifact(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tool := NewExportToExcelTool(
		WithOutputDir(tmpDir),
		WithBaseURL("http://test.com/exports"),
	).(*ExportToExcelTool)

	input := `{
		"data": {
			"columns": ["id", "name"],
			"rows": [[1, "Alice"], [2, "Bob"]],
			"row_count": 2
		},
		"filename": "report.xlsx"
	}`

	result, err := tool.CallStructured(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Artifacts, 1)

	artifact := result.Artifacts[0]
	assert.Equal(t, "export", artifact.Type)
	assert.Equal(t, "report.xlsx", artifact.Name)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", artifact.MimeType)
	assert.Equal(t, "http://test.com/exports/report.xlsx", artifact.URL)
	assert.Equal(t, 2, artifact.Metadata["row_count"])
	assert.Positive(t, artifact.SizeBytes)

	outputJSON, err := tool.Call(context.Background(), input)
	require.NoError(t, err)
	var output excelExportOutput
	require.NoError(t, json.Unmarshal([]byte(outputJSON), &output))
	assert.Equal(t, "report.xlsx", output.Filename)

	_, statErr := os.Stat(filepath.Join(tmpDir, "report.xlsx"))
	assert.NoError(t, statErr)
}

func TestExportToExcelTool_CallStructured_ValidationError(t *testing.T) {
	t.Parallel()

	tool := NewExportToExcelTool(WithOutputDir(t.TempDir())).(*ExportToExcelTool)
	result, err := tool.CallStructured(context.Background(), `{"filename":"report.xlsx"}`)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Artifacts)
	assert.Equal(t, types.CodecToolError, result.CodecID)
}
