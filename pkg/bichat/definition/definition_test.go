package definition

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFiles_NonRecursiveAndSorted(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"root/b.md":      &fstest.MapFile{Data: []byte("b")},
		"root/a.md":      &fstest.MapFile{Data: []byte("a")},
		"root/sub/c.md":  &fstest.MapFile{Data: []byte("c")},
		"root/notes.txt": &fstest.MapFile{Data: []byte("n")},
	}

	files, err := LoadFiles(fsys, LoadFilesOptions{
		Root:      "root",
		Recursive: false,
		Match: func(path string, entry fs.DirEntry) bool {
			return entry.Name() == "a.md" || entry.Name() == "b.md"
		},
	})
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, "root/a.md", files[0].Path)
	assert.Equal(t, "root/b.md", files[1].Path)
}

func TestLoadFiles_Recursive(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"skills/a/SKILL.md": &fstest.MapFile{Data: []byte("a")},
		"skills/b/SKILL.md": &fstest.MapFile{Data: []byte("b")},
	}

	files, err := LoadFiles(fsys, LoadFilesOptions{
		Root:      "skills",
		Recursive: true,
		Match: func(path string, entry fs.DirEntry) bool {
			return entry.Name() == "SKILL.md"
		},
	})
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, "skills/a/SKILL.md", files[0].Path)
	assert.Equal(t, "skills/b/SKILL.md", files[1].Path)
}

func TestParseDocument_StrictKnownFields(t *testing.T) {
	t.Parallel()

	type frontMatter struct {
		Name string `yaml:"name"`
	}

	_, err := ParseDocument[frontMatter]([]byte(`---
name: ok
unknown: nope
---
body`), "doc.md", ParseDocumentOptions{KnownFields: true, RequireBody: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field unknown not found")
}

func TestParseDocument_Valid(t *testing.T) {
	t.Parallel()

	type frontMatter struct {
		Name string `yaml:"name"`
	}

	doc, err := ParseDocument[frontMatter]([]byte(`---
name: ok
---
 body `), "doc.md", ParseDocumentOptions{KnownFields: true, RequireBody: true})
	require.NoError(t, err)
	assert.Equal(t, "ok", doc.FrontMatter.Name)
	assert.Equal(t, "body", doc.Body)
}
