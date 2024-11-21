package commands

import (
	"embed"
	"github.com/iota-agency/iota-sdk/modules"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

func getAllFilenames(fs *embed.FS, dir string) (out []string, err error) {
	if len(dir) == 0 {
		dir = "."
	}

	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		fp := path.Join(dir, entry.Name())
		if entry.IsDir() {
			res, err := getAllFilenames(fs, fp)
			if err != nil {
				return nil, err
			}

			out = append(out, res...)

			continue
		}

		out = append(out, fp)
	}

	return
}

func copyFile(file fs.File, dest, filename string) error {
	defer file.Close()
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	out, err := os.Create(path.Join(dest, filename))
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return err
	}
	return nil
}

// CollectStatic collects all static files from the registry and writes them to the destination directory.
func CollectStatic(registry *modules.ModuleRegistry, dest string) error {
	for _, embedFs := range registry.Assets() {
		filenames, err := getAllFilenames(embedFs, "")
		if err != nil {
			return err
		}
		for _, relPath := range filenames {
			file, err := embedFs.Open(relPath)
			if err != nil {
				return err
			}
			filename := filepath.Base(relPath)
			dest = filepath.Join(dest, filepath.Dir(relPath))
			if err := copyFile(file, dest, filename); err != nil {
				return err
			}
		}
	}
	return nil
}
