// Command extruntime-catalog dumps the SDK's native extension-runtime catalog
// (native entities with key types and fields, native lists, native events and
// every other native contract the SDK registers) as a deterministic JSON
// snapshot. The committed snapshot ships in the @iota-uz/sdk distribution so
// `iota-ext kit generate` renders @iota-kit/core and @iota-kit/crm without a
// server. Applications with natives of their own export theirs with the same
// document (catalog.Natives(set).Snapshot() over their composed native.Set, or
// GET /_ext/manage/catalog/natives).
//
// Usage:
//
//	go run ./cmd/extruntime-catalog                 # write web/runtime/catalog/sdk-natives.json
//	go run ./cmd/extruntime-catalog -out file.json  # write elsewhere ("-" is stdout)
//	go run ./cmd/extruntime-catalog -check          # exit 1 when the committed file is stale
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// DefaultOutput is the committed snapshot, relative to the repository root.
const DefaultOutput = "web/runtime/catalog/sdk-natives.json"

// SDKSnapshot is the native catalog of the SDK modules: the native.Set
// composed from modules.Natives(), the same registrations an application
// passes to the extension runtime before adding its own.
func SDKSnapshot() (catalog.Snapshot, error) {
	set, err := native.Compose(modules.Natives()...)
	if err != nil {
		return catalog.Snapshot{}, err
	}
	return catalog.Natives(set).Snapshot(), nil
}

// Render is the canonical document of the SDK snapshot.
func Render() ([]byte, error) {
	snap, err := SDKSnapshot()
	if err != nil {
		return nil, err
	}
	return snap.JSON()
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "extruntime-catalog:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("extruntime-catalog", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", DefaultOutput, `output file ("-" for stdout)`)
	check := fs.Bool("check", false, "do not write; fail when the file differs from the current catalog")
	if err := fs.Parse(args); err != nil {
		return err
	}
	doc, err := Render()
	if err != nil {
		return err
	}
	if *out == "-" {
		_, err := stdout.Write(doc)
		return err
	}
	if *check {
		have, err := os.ReadFile(*out)
		if err != nil {
			return fmt.Errorf("read %s: %w (run `go run ./cmd/extruntime-catalog` and commit the result)", *out, err)
		}
		if !bytes.Equal(have, doc) {
			return fmt.Errorf("%s is stale (run `go run ./cmd/extruntime-catalog` and commit the result)", *out)
		}
		_, err = fmt.Fprintf(stdout, "%s is up to date\n", *out)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(*out, doc, 0o644); err != nil {
		return errors.Join(fmt.Errorf("write %s", *out), err)
	}
	_, err = fmt.Fprintf(stdout, "wrote %s\n", *out)
	return err
}
