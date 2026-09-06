package sdk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func CheckChanges(ctx context.Context, runner Runner, root, base string) error {
	if strings.HasPrefix(base, "-") {
		return fmt.Errorf("invalid base ref")
	}
	out, err := runner.Run(ctx, root, nil, "git", "diff", "--name-status", "--no-renames", "-z", base+"...HEAD", "--", ".changes")
	if err != nil {
		return err
	}
	count := 0
	records := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	for index := 0; index+1 < len(records); index += 2 {
		status, path := records[index], records[index+1]
		if !strings.HasSuffix(path, ".json") {
			continue
		}
		if status != "A" {
			return fmt.Errorf("change declarations are append-only: %s", path)
		}
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return err
		}
		change, err := Decode[Change](data)
		if err != nil {
			return err
		}
		if err = change.Validate(); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("add .changes/<name>.json with bump and summary; use none with a reason when no release is needed")
	}
	return nil
}
