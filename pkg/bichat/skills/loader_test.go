package skills

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCatalog_ValidNestedTree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSkillFile(t, root, "analytics/sql-debug", `---
name: SQL Debugging
description: Recover from SQL errors fast
when_to_use:
  - sql error
  - missing column
tags:
  - sql
  - debugging
---
# SQL Debug
Use schema tools before retrying.
`)
	writeSkillFile(t, root, "analytics/kpi-review", `---
name: KPI Review
description: Audit KPI calculations and business definitions
when_to_use:
  - kpi mismatch
  - metric validation
tags:
  - analytics
  - metrics
---
Validate KPI formulas and grain before reporting.
`)

	catalog, err := LoadCatalog(root)
	require.NoError(t, err)
	require.Len(t, catalog.Skills, 2)

	sqlSkill, ok := catalog.Get("analytics/sql-debug")
	require.True(t, ok)
	assert.Equal(t, "analytics", sqlSkill.ParentSlug)
	assert.Equal(t, "SQL Debugging", sqlSkill.Metadata.Name)

	assert.Equal(t, []string{"analytics/kpi-review", "analytics/sql-debug"}, catalog.Children["analytics"])
}

func TestLoadCatalog_MissingRequiredField(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSkillFile(t, root, "bad/skill", `---
name: Broken
description: Missing tags and when_to_use
---
Body
`)

	_, err := LoadCatalog(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "when_to_use")
}

func TestLoadCatalog_MalformedYAML(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSkillFile(t, root, "broken/yaml", `---
name: Broken
description: Nope
when_to_use:
  - one
tags: [sql
---
Body
`)

	_, err := LoadCatalog(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "front matter")
}

func TestLoadCatalog_EmptyBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSkillFile(t, root, "empty/body", `---
name: Empty Body
description: No instructions
when_to_use:
  - test
tags:
  - test
---

`)

	_, err := LoadCatalog(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "markdown body is required")
}

func TestLoadCatalog_DeterministicSlugGeneration(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSkillFile(t, root, "Finance/Month-End", `---
name: Month End
description: Close month books
when_to_use:
  - month close
tags:
  - finance
---
Do month-end checks.
`)

	catalog, err := LoadCatalog(root)
	require.NoError(t, err)
	require.Len(t, catalog.Skills, 1)
	assert.Equal(t, "finance/month-end", catalog.Skills[0].Slug)
}

func TestLoadCatalogFS_ValidTree(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"skills/finance/month-end/SKILL.md": &fstest.MapFile{Data: []byte(`---
name: Month End
description: Close month books
when_to_use:
  - month close
tags:
  - finance
---
Run close checklist.`)},
	}

	catalog, err := LoadCatalogFS(fsys, "skills")
	require.NoError(t, err)
	require.Len(t, catalog.Skills, 1)
	assert.Equal(t, "finance/month-end", catalog.Skills[0].Slug)
	assert.Equal(t, "skills/finance/month-end/SKILL.md", catalog.Skills[0].Path)
}

func writeSkillFile(t *testing.T, root, relDir, content string) {
	t.Helper()
	dir := filepath.Join(root, relDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, skillFileName), []byte(content), 0o644))
}
