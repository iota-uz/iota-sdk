package sdk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type fileBackup struct {
	path    string
	data    []byte
	mode    os.FileMode
	existed bool
}

func backupFiles(root string, paths []string) ([]fileBackup, error) {
	var backups []fileBackup
	for _, path := range paths {
		if _, err := safeDir(root, filepath.Dir(path)); err != nil {
			return nil, err
		}
		absolute := filepath.Join(root, path)
		info, err := os.Lstat(absolute)
		if os.IsNotExist(err) {
			backups = append(backups, fileBackup{path: absolute})
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("dependency file must be regular: %s", path)
		}
		data, err := os.ReadFile(absolute)
		if err != nil {
			return nil, err
		}
		backups = append(backups, fileBackup{path: absolute, data: data, mode: info.Mode(), existed: true})
	}
	return backups, nil
}

func restoreFiles(backups []fileBackup) error {
	var failures []error
	for _, backup := range backups {
		var err error
		if backup.existed {
			err = os.WriteFile(backup.path, backup.data, backup.mode)
		} else {
			err = os.Remove(backup.path)
			if os.IsNotExist(err) {
				err = nil
			}
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func dependencyFiles(d Dependency) []string {
	paths := []string{DependencyPath, filepath.Join(d.GoDir, "go.mod"), filepath.Join(d.GoDir, "go.sum")}
	if d.WebDir != "" {
		paths = append(paths, filepath.Join(d.WebDir, "package.json"), filepath.Join(d.WebDir, "pnpm-lock.yaml"))
	}
	return paths
}
