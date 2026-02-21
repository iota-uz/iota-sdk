package definition

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

type SourceFile struct {
	Path    string
	Content []byte
}

type LoadFilesOptions struct {
	Root      string
	Recursive bool
	Match     func(path string, entry fs.DirEntry) bool
}

func LoadFiles(fsys fs.FS, opts LoadFilesOptions) ([]SourceFile, error) {
	if fsys == nil {
		return nil, fmt.Errorf("filesystem is required")
	}

	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}

	paths := make([]string, 0)
	if opts.Recursive {
		err := fs.WalkDir(fsys, root, func(filePath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if opts.Match != nil && !opts.Match(filePath, d) {
				return nil
			}
			paths = append(paths, filePath)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %q: %w", root, err)
		}
	} else {
		entries, err := fs.ReadDir(fsys, root)
		if err != nil {
			return nil, fmt.Errorf("read dir %q: %w", root, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			filePath := entry.Name()
			if root != "." {
				filePath = path.Join(root, entry.Name())
			}
			if opts.Match != nil && !opts.Match(filePath, entry) {
				continue
			}
			paths = append(paths, filePath)
		}
	}

	sort.Strings(paths)
	files := make([]SourceFile, 0, len(paths))
	for _, filePath := range paths {
		content, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return nil, fmt.Errorf("read file %q: %w", filePath, err)
		}
		files = append(files, SourceFile{Path: filePath, Content: content})
	}

	return files, nil
}
