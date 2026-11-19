// Package cli runs scenario commands against any testenv.Control, so a
// consumer binary can prepare the same scenario in-process (testenv.Local) or
// over HTTP (testenv.NewClient) with identical input and output.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/iota-uz/iota-sdk/pkg/testenv"
)

const usage = `usage: <command> [flags]

commands:
  scenarios                                    list scenario definitions
  prepare  --scope ID [--input FILE|-] [--manifest FILE]
                                               reserve the scope and prepare the input
  dispose  --scope ID                          dispose the scope (idempotent)
  manifest --scope ID                          print the scope's replay manifest
  replay   --scope ID --manifest FILE          replay a manifest in a new scope`

// Run executes one command. Results are written to stdout as JSON; control
// errors are returned as *testenv.Error.
func Run(ctx context.Context, control testenv.Control, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	command, args := args[0], args[1:]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	scope := flags.String("scope", "", "physical scope ID")
	inputPath := flags.String("input", "-", "scenario input JSON file, - for stdin")
	manifestPath := flags.String("manifest", "", "manifest file")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("%w\n%s", err, usage)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	requireScope := func() error {
		if *scope == "" {
			return fmt.Errorf("--scope is required\n%s", usage)
		}
		return nil
	}
	switch command {
	case "scenarios":
		definitions, err := control.Definitions(ctx)
		if err != nil {
			return err
		}
		return encoder.Encode(definitions)
	case "prepare":
		if err := requireScope(); err != nil {
			return err
		}
		var input testenv.Input
		if err := readJSON(ctx, *inputPath, stdin, &input); err != nil {
			return err
		}
		input.ScopeID = *scope
		if err := control.ReserveScope(ctx, *scope); err != nil {
			return err
		}
		result, err := control.Prepare(ctx, input)
		if err != nil {
			return err
		}
		if *manifestPath != "" {
			manifest, err := control.Manifest(ctx, *scope)
			if err != nil {
				return err
			}
			if err := writeJSON(*manifestPath, manifest); err != nil {
				return err
			}
		}
		return encoder.Encode(result)
	case "dispose":
		if err := requireScope(); err != nil {
			return err
		}
		if err := control.Dispose(ctx, *scope); err != nil {
			return err
		}
		return encoder.Encode(map[string]bool{"disposed": true})
	case "manifest":
		if err := requireScope(); err != nil {
			return err
		}
		manifest, err := control.Manifest(ctx, *scope)
		if err != nil {
			return err
		}
		return encoder.Encode(manifest)
	case "replay":
		if err := requireScope(); err != nil {
			return err
		}
		if *manifestPath == "" {
			return fmt.Errorf("--manifest is required\n%s", usage)
		}
		var manifest testenv.Manifest
		if err := readJSON(ctx, *manifestPath, stdin, &manifest); err != nil {
			return err
		}
		if err := control.ReserveScope(ctx, *scope); err != nil {
			return err
		}
		result, err := control.Replay(ctx, *scope, manifest)
		if err != nil {
			return err
		}
		return encoder.Encode(result)
	}
	return fmt.Errorf("unknown command %q\n%s", command, usage)
}

const inputLimit = 4 << 20

// readJSON reads exactly one JSON document (trailing whitespace allowed) of
// at most inputLimit bytes. It returns when ctx ends, even if the input stays
// open without completing.
func readJSON(ctx context.Context, path string, stdin io.Reader, target any) error {
	reader := stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		reader = file
	}
	type read struct {
		data []byte
		err  error
	}
	done := make(chan read, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(reader, inputLimit+1))
		done <- read{data, err}
	}()
	var data []byte
	select {
	case <-ctx.Done():
		return &testenv.Error{Code: "timeout", Message: "input not read: " + ctx.Err().Error()}
	case got := <-done:
		if got.err != nil {
			return got.err
		}
		data = got.data
	}
	if len(data) > inputLimit {
		return &testenv.Error{Code: "invalid_input", Message: "input exceeds 4 MiB"}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &testenv.Error{Code: "invalid_input", Message: err.Error()}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return &testenv.Error{Code: "invalid_input", Message: "expected exactly one JSON document"}
	}
	return nil
}

func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o600)
}
