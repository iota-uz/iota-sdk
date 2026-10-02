package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationPreservesCausesAndFlagsAmbiguity(t *testing.T) {
	root := t.TempDir()
	source := `package sample
import "github.com/iota-uz/iota-sdk/pkg/serrors"
func f(op serrors.Op, err error, value string) error {
 _ = serrors.E(op, err)
 _ = serrors.E(op, serrors.Invalid, "bad input", err)
 _ = serrors.E(op, err, "context")
 _ = serrors.E(serrors.Invalid, "invalid")
 _ = serrors.E(op, serrors.PermissionDenied, "denied: " + value)
 return serrors.E(op, err, err)
}`
	path := filepath.Join(root, "sample.go")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("go", "run", ".", "-root", root, "-write").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	if !strings.Contains(string(output), "multiple payload arguments") {
		t.Fatalf("missing review warning: %s", output)
	}
	migrated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`serrors.Wrap(op, err)`, `serrors.New(serrors.Invalid, "bad input").WithOp(op).WithCause(err)`, `serrors.WrapContext(op, err, "context")`, `serrors.New(serrors.Invalid, "invalid").WithOp("")`, `serrors.New(serrors.PermissionDenied, "denied: "+value).WithOp(op)`, `serrors.E(op, err, err)`} {
		if !strings.Contains(string(migrated), fragment) {
			t.Fatalf("missing %q in %s", fragment, migrated)
		}
	}
}
