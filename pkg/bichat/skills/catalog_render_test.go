package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderCatalogReference_IncludesMetadataAndInstructions(t *testing.T) {
	t.Parallel()

	catalog := buildRenderTestCatalog(t)
	ref := RenderCatalogReference(catalog, 10, 4000)

	assert.Contains(t, ref, "SKILLS CATALOG:")
	assert.Contains(t, ref, "load_skill")
	assert.Contains(t, ref, "@finance/month-end")
	assert.Contains(t, ref, "@insurance/reserves")
}

func TestRenderCatalogReference_RespectsLimit(t *testing.T) {
	t.Parallel()

	catalog := buildRenderTestCatalog(t)
	ref := RenderCatalogReference(catalog, 1, 4000)

	assert.Contains(t, ref, "SKILLS CATALOG:")
	assert.Contains(t, ref, "additional skill(s) omitted")
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
