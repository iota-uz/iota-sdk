package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderCatalogReference(t *testing.T) {
	t.Parallel()

	catalog := buildRenderTestCatalog(t)
	tests := []struct {
		name     string
		limit    int
		contains []string
	}{
		{
			name:     "IncludesMetadataAndInstructions",
			limit:    10,
			contains: []string{"SKILLS CATALOG:", "load_skill", "@finance/month-end", "@insurance/reserves"},
		},
		{
			name:     "RespectsLimit",
			limit:    1,
			contains: []string{"SKILLS CATALOG:", "additional skill(s) omitted"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := RenderCatalogReference(catalog, tc.limit, 4000)
			for _, want := range tc.contains {
				assert.Contains(t, ref, want)
			}
		})
	}
}

func TestRenderLoadedSkill_FormatsBody(t *testing.T) {
	t.Parallel()

	catalog := buildRenderTestCatalog(t)
	skill, ok := catalog.Get("insurance/reserves")
	require.True(t, ok)

	loaded := RenderLoadedSkill(skill, 4000)
	assert.Contains(t, loaded, "SKILL LOADED: @insurance/reserves")
	assert.Contains(t, loaded, "instructions:")
	assert.Contains(t, loaded, "Reserve formulas")
}

func buildRenderTestCatalog(t *testing.T) *Catalog {
	t.Helper()

	root := t.TempDir()
	writeRenderTestSkill(t, root, "finance/month-end", `---
name: Month End
description: Run month close checklist
when_to_use:
  - month close
tags:
  - finance
---
Close books safely.
`)
	writeRenderTestSkill(t, root, "insurance/reserves", `---
name: Reserves
description: Compute regulatory reserve metrics
when_to_use:
  - reserves
tags:
  - reserves
---
Reserve formulas and SQL templates.
`)

	catalog, err := LoadCatalog(root)
	require.NoError(t, err)
	return catalog
}

func writeRenderTestSkill(t *testing.T, root, slug, content string) {
	t.Helper()

	dir := filepath.Join(root, filepath.FromSlash(slug))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))
}
