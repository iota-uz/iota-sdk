package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func VerifyLocks(ctx context.Context, runner Runner, root string, d Dependency) error {
	if !versionPattern.MatchString(d.Version) || !shaPattern.MatchString(d.SHA) {
		return fmt.Errorf("missing production release identity")
	}
	goDir, err := safeDir(root, d.GoDir)
	if err != nil {
		return err
	}
	out, err := runner.Run(ctx, goDir, nil, "env", "GOWORK=off", "go", "list", "-m", "-json", Repository)
	if err != nil {
		return err
	}
	var module struct {
		Version string
		Replace json.RawMessage
	}
	if err = json.Unmarshal(out, &module); err != nil {
		return err
	}
	if module.Version != "v"+d.Version || len(module.Replace) > 0 {
		return fmt.Errorf("Go production graph does not match the verified release")
	}
	if d.WebDir != "" {
		webDir, err := safeDir(root, d.WebDir)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(webDir, "package.json"))
		if err != nil {
			return err
		}
		var pkg struct {
			Dependencies map[string]string `json:"dependencies"`
		}
		if err = json.Unmarshal(data, &pkg); err != nil {
			return err
		}
		if pkg.Dependencies["@iota-uz/sdk"] != d.Version {
			return fmt.Errorf("npm production version must exactly match Go")
		}
		// Validate that the lockfile agrees without executing consumer scripts.
		if _, err = runner.Run(ctx, webDir, nil, "pnpm", "install", "--ignore-workspace", "--ignore-scripts", "--ignore-pnpmfile", "--frozen-lockfile", "--lockfile-only"); err != nil {
			return err
		}
	}
	return nil
}
