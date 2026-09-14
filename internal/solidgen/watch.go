package solidgen

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch reconciles Solid outputs initially and after relevant Go/TSX graph
// changes. Generation errors are reported and the watcher remains alive so a
// developer can fix the source without restarting the dev command.
func Watch(ctx context.Context, cfg Config, report func(Result, error)) error {
	root, err := filepath.Abs(cfg.ModuleRoot)
	if err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	if err := addWatchDirectories(watcher, root); err != nil {
		return err
	}
	run := func() {
		result, runErr := Run(Config{ModuleRoot: root, Catalog: cfg.Catalog})
		report(result, runErr)
	}
	run()

	var debounce <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			report(Result{}, watchErr)
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&fsnotify.Create != 0 {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					_ = addWatchDirectories(watcher, event.Name)
				}
			}
			if relevantGraphFile(event.Name) {
				debounce = time.After(120 * time.Millisecond)
			}
		case <-debounce:
			debounce = nil
			run()
		}
	}
}

func addWatchDirectories(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && skippedDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		return watcher.Add(path)
	})
}

func relevantGraphFile(name string) bool {
	return strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".tsx")
}

func skippedDirectory(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", ".worktrees":
		return true
	default:
		return false
	}
}
