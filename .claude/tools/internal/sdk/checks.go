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
	out, err := runner.Run(ctx, root, nil, "git", "diff", "--name-status", "--no-renames", base+"...HEAD", "--", ".changes")
	if err != nil {
		return err
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) != 2 || !strings.HasSuffix(fields[1], ".json") {
			continue
		}
		if fields[0] != "A" {
			return fmt.Errorf("change declarations are append-only: %s", fields[1])
		}
		data, err := os.ReadFile(filepath.Join(root, fields[1]))
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
