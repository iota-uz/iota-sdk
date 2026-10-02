package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationOnlyClassifiesAssignedDriverErrors(t *testing.T) {
	// A matching method name or a nearby SQL call must not classify a domain error.
	root := t.TempDir()
	source := `package sample
import (
 "context"
 "database/sql"
 "github.com/iota-uz/iota-sdk/pkg/repo"
 "github.com/iota-uz/iota-sdk/pkg/serrors"
)
type domain struct{}
func (domain) Get() error { return sql.ErrNoRows }
func driver(db *sql.DB) error {
 err := db.QueryRow("select 1").Scan(new(int))
 if err != nil { return serrors.Wrap("driver", err) }
 return nil
}
func business(d domain) error {
 err := d.Get()
 if err != nil { return serrors.Wrap("domain", err) }
 return nil
}
func other(db *sql.DB, cause error) error {
 err := db.QueryRow("select 1").Scan(new(int))
 if err != nil { return serrors.Wrap("mapped", cause) }
 return nil
}
func nested(db *sql.DB, d domain) error {
 err := db.QueryRow("select 1").Scan(new(int))
 if err != nil {
  if err := d.Get(); err != nil { return serrors.Wrap("nested", err) }
  return serrors.Wrap("outer", err)
 }
 return nil
}
func contextual(db *sql.DB) error {
 if err := db.QueryRow("select 1").Scan(new(int)); err != nil { return serrors.WrapContext("contextual", err, "private SQL context") }
 return nil
}
func reassigned(db *sql.DB, d domain) error {
 err := db.QueryRow("select 1").Scan(new(int))
 if err != nil {
  err = d.Get()
  return serrors.Wrap("reassigned", err)
 }
 return nil
}
func transaction(tx repo.Tx) error {
 _, err := tx.Exec(context.Background(), "select 1")
 if err != nil { return serrors.Wrap("transaction", err) }
 return nil
}
`
	path := filepath.Join(root, "sample_repository.go")
	require.NoError(t, os.WriteFile(path, []byte(source), 0600))
	checkout, err := filepath.Abs("../..")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.org/sample\n\ngo 1.24.10\nrequire github.com/iota-uz/iota-sdk v0.0.0\nreplace github.com/iota-uz/iota-sdk => "+checkout+"\n"), 0600))
	cmd := exec.Command("go", "run", ".", "-root", root, "-write")
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, `serrors.Wrap("domain", err)`)
	require.Contains(t, text, `serrors.Wrap("mapped", cause)`)
	require.Contains(t, text, `serrors.Wrap("nested", err)`)
	require.Contains(t, text, `serrors.Wrap("reassigned", err)`)
	require.Contains(t, text, `serrors.FromDBContext("contextual", err, "private SQL context")`)
	require.Contains(t, text, `serrors.FromDB("driver", err)`)
	require.Contains(t, text, `serrors.FromDB("outer", err)`)
	require.Contains(t, text, `serrors.FromDB("transaction", err)`)
	require.Equal(t, 3, strings.Count(text, "serrors.FromDB("))
	cmd = exec.Command("go", "run", ".", "-root", root)
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
	require.Contains(t, string(out), "classified 0 concrete driver error boundaries")
}
