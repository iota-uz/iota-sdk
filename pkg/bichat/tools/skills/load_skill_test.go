package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	bichatskills "github.com/iota-uz/iota-sdk/pkg/bichat/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSkillTool_LoadsBySlug(t *testing.T) {
	t.Parallel()

	catalog := createTestCatalog(t)
	tool := NewLoadSkillTool(catalog)
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "BareSlug",
			input:    `{"skill":"insurance/reserves"}`,
			contains: []string{"SKILL LOADED: @insurance/reserves", "Reserve Calculations", "Use RNP, RZU, and RPNU formulas"},
		},
		{
			name:     "MentionSlug",
			input:    `{"skill":"@insurance/reserves"}`,
			contains: []string{"SKILL LOADED: @insurance/reserves"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tool.Call(context.Background(), tc.input)
			require.NoError(t, err)
			for _, want := range tc.contains {
				assert.Contains(t, out, want)
			}
		})
	}
}

func TestLoadSkillTool_MissingSkillReturnsSuggestions(t *testing.T) {
	t.Parallel()

	catalog := createTestCatalog(t)
	tool := NewLoadSkillTool(catalog)

	out, err := tool.Call(context.Background(), `{"skill":"insurance/reserv"}`)
	require.NoError(t, err)
	assert.Contains(t, out, `Skill not found: "insurance/reserv".`)
	assert.Contains(t, out, "insurance/reserves")
}

func TestLoadSkillTool_MissingRequiredField(t *testing.T) {
	t.Parallel()

	catalog := createTestCatalog(t)
	tool := NewLoadSkillTool(catalog)

	out, err := tool.Call(context.Background(), `{"bad":"value"}`)
	require.NoError(t, err)
	assert.Contains(t, out, "Missing required field `skill`")
}

func TestLoadSkillTool_InvalidArgs(t *testing.T) {
	t.Parallel()

	catalog := createTestCatalog(t)
	tool := NewLoadSkillTool(catalog)

	out, err := tool.Call(context.Background(), `{invalid-json`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid load_skill arguments")
	assert.Empty(t, out)
}

func createTestCatalog(t *testing.T) *bichatskills.Catalog {
	t.Helper()

	root := t.TempDir()
	writeSkill(t, root, "insurance/reserves", `---
name: Reserve Calculations
description: Calculate insurance reserves by regulatory formulas
when_to_use:
  - reserves
  - actuarial
tags:
  - reserves
---
Use RNP, RZU, and RPNU formulas.
`)

	catalog, err := bichatskills.LoadCatalog(root)
	require.NoError(t, err)
	return catalog
}

func writeSkill(t *testing.T, root, slug, content string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(slug))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))
}
