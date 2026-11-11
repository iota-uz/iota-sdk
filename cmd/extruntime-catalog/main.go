// Command extruntime-catalog dumps the SDK's native extension-runtime catalog
// (native entities with key types and fields, native lists, native events and
// every other native contract the SDK registers) as a deterministic JSON
// snapshot. The committed snapshot ships in the @iota-uz/sdk distribution so
// `iota-ext kit generate` renders @iota-kit/core and @iota-kit/crm without a
// server. Applications with natives of their own export theirs with the same
// document (`catalog.NativeSet.Snapshot`, or GET /_ext/manage/catalog/natives).
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

	// Registers core.user.created.v1 in the native event catalog.
	_ "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
)

// DefaultOutput is the committed snapshot, relative to the repository root.
const DefaultOutput = "web/runtime/catalog/sdk-natives.json"

// SDKSnapshot builds the catalog exactly as runtime.New does for an
// application that adds no natives: the default entities and lists plus the
// events Go modules registered at init time.
func SDKSnapshot() (catalog.Snapshot, error) {
	set := catalog.NewNativeSet(data.DefaultNatives())
	for _, r := range events.Default.Resources() {
		if err := set.Register(r); err != nil {
			return catalog.Snapshot{}, err
		}
	}
	for _, l := range data.DefaultNativeLists() {
		if err := set.RegisterList(l); err != nil {
			return catalog.Snapshot{}, err
		}
	}
	return set.Snapshot(), nil
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
		fmt.Fprintf(stdout, "%s is up to date\n", *out)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(*out, doc, 0o644); err != nil {
		return errors.Join(fmt.Errorf("write %s", *out), err)
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	return nil
}
