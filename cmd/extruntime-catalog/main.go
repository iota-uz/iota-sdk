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
//	go run ./cmd/extruntime-catalog -openapi api.json  # write the OpenAPI 3.1 document instead
//
// -openapi renders the HTTP interface of the native catalog (the browser RPC
// for the native commands and queries, and the management API) as an
// OpenAPI 3.1 document, offline, for tools that cannot reach a running server
// (a Swagger UI, a code generator). It describes the SDK's natives; a running
// application serves the full document, with its packages, at
// /_ext/openapi.json. Applications with natives of their own render theirs
// with runtime.BuildOpenAPI over reference.Natives(catalog.Natives(set)).
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/openapi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
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

// RenderOpenAPI is the OpenAPI 3.1 document of the SDK's native catalog: what
// an administrator sees of an application that registers only the SDK modules
// and has no package active.
func RenderOpenAPI() ([]byte, error) {
	set, err := native.Compose(modules.Natives()...)
	if err != nil {
		return nil, err
	}
	model, err := reference.New(reference.Natives(catalog.Natives(set))).FullModel(context.Background(), "en")
	if err != nil {
		return nil, err
	}
	doc, err := runtime.BuildOpenAPI(model, "sid")
	if err != nil {
		return nil, err
	}
	if err := openapi.Validate(doc); err != nil {
		return nil, fmt.Errorf("generated OpenAPI document is invalid: %w", err)
	}
	return doc, nil
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
	openAPI := fs.String("openapi", "", `write the OpenAPI 3.1 document of the native catalog to this file ("-" for stdout) instead of the snapshot`)
	if err := fs.Parse(args); err != nil {
		return err
	}
	render := Render
	regenerate := "go run ./cmd/extruntime-catalog"
	if *openAPI != "" {
		render, *out = RenderOpenAPI, *openAPI
		regenerate += " -openapi " + *openAPI
	}
	doc, err := render()
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
			return fmt.Errorf("read %s: %w (run `%s` and commit the result)", *out, err, regenerate)
		}
		if !bytes.Equal(have, doc) {
			return fmt.Errorf("%s is stale (run `%s` and commit the result)", *out, regenerate)
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
