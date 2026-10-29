// Package protocol is the worker side of the JSON contract shared with
// github.com/iota-uz/iota-sdk/pkg/shell. It is duplicated on purpose: the
// application contract must not depend on this module and vice versa.
package protocol

import (
	"fmt"
	"path"
	"strings"
)

const (
	InputDir  = "/input"
	TempDir   = "/tmp"
	OutputDir = "/output"

	MaxTimeoutMS      = 10000
	MaxOutputBytes    = 128 << 10
	MaxScriptBytes    = 32 << 10
	MaxInputBytes     = 32 << 20
	MaxInputFiles     = 100
	MaxInputPathBytes = 1024
	MaxScratchBytes   = 64 << 20

	// Wire bounds: JSON carries file contents as base64 (4/3) and may escape
	// script and output text up to 6x; the margin covers that and the envelope.
	MaxRequestBytes = MaxInputBytes*4/3 + 1<<20
	MaxResultBytes  = MaxScratchBytes*4/3 + 4<<20
)

type Request struct {
	Script         string      `json:"script"`
	Files          []InputFile `json:"files,omitempty"`
	Cwd            string      `json:"cwd"`
	TimeoutMS      int         `json:"timeout_ms"`
	MaxOutputBytes int         `json:"max_output_bytes"`
}

type InputFile struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

type LimitHit string

const (
	LimitTimeout    LimitHit = "timeout"
	LimitMemory     LimitHit = "memory"
	LimitFilesystem LimitHit = "filesystem"
	LimitOutput     LimitHit = "output"
	LimitExecution  LimitHit = "execution"
)

type Result struct {
	RunID           string       `json:"run_id"`
	ExitCode        *int         `json:"exit_code"`
	Stdout          string       `json:"stdout"`
	Stderr          string       `json:"stderr"`
	StdoutTruncated bool         `json:"stdout_truncated"`
	StderrTruncated bool         `json:"stderr_truncated"`
	LimitHit        *LimitHit    `json:"limit_hit"`
	Outputs         []OutputFile `json:"outputs"`
	DurationMS      int64        `json:"duration_ms"`
}

type OutputFile struct {
	Path     string `json:"path"`
	Content  []byte `json:"content"`
	MimeType string `json:"mime_type"`
}

type ErrorCode string

const (
	CodeInvalidInput          ErrorCode = "invalid_input"
	CodeLimitExceeded         ErrorCode = "limit_exceeded"
	CodeBusy                  ErrorCode = "busy"
	CodeDependencyUnavailable ErrorCode = "dependency_unavailable"
	CodeUnauthorized          ErrorCode = "unauthorized"
)

type Error struct {
	Code         ErrorCode `json:"code"`
	Field        string    `json:"field,omitempty"`
	Message      string    `json:"message"`
	RetryAfterMS int64     `json:"retry_after_ms,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s %s", e.Code, e.Field, e.Message)
}

// Validate rejects a request that must not be started. The application
// validates the same rules; the worker never trusts that it did.
func (r *Request) Validate() *Error {
	if strings.TrimSpace(r.Script) == "" {
		return &Error{Code: CodeInvalidInput, Field: "script", Message: "script is required"}
	}
	if len(r.Script) > MaxScriptBytes {
		return &Error{Code: CodeLimitExceeded, Field: "script", Message: fmt.Sprintf("script exceeds %d bytes", MaxScriptBytes)}
	}
	if r.TimeoutMS <= 0 || r.TimeoutMS > MaxTimeoutMS {
		return &Error{Code: CodeInvalidInput, Field: "timeout_ms", Message: fmt.Sprintf("timeout_ms must be within 1..%d", MaxTimeoutMS)}
	}
	if r.MaxOutputBytes <= 0 || r.MaxOutputBytes > MaxOutputBytes {
		return &Error{Code: CodeInvalidInput, Field: "max_output_bytes", Message: fmt.Sprintf("max_output_bytes must be within 1..%d", MaxOutputBytes)}
	}
	if !insideSandbox(r.Cwd) {
		return &Error{Code: CodeInvalidInput, Field: "cwd", Message: "cwd must be a clean absolute path inside the sandbox"}
	}

	if len(r.Files) > MaxInputFiles {
		return &Error{Code: CodeLimitExceeded, Field: "files", Message: fmt.Sprintf("files exceed %d entries", MaxInputFiles)}
	}
	seen := make(map[string]struct{}, len(r.Files))
	var total int64
	for i, f := range r.Files {
		if len(f.Path) > MaxInputPathBytes {
			return &Error{Code: CodeLimitExceeded, Field: fmt.Sprintf("files[%d].path", i), Message: fmt.Sprintf("path exceeds %d bytes", MaxInputPathBytes)}
		}
		if !validInputPath(f.Path) {
			return &Error{Code: CodeInvalidInput, Field: fmt.Sprintf("files[%d].path", i), Message: fmt.Sprintf("invalid path %q", f.Path)}
		}
		if _, ok := seen[f.Path]; ok {
			return &Error{Code: CodeInvalidInput, Field: fmt.Sprintf("files[%d].path", i), Message: fmt.Sprintf("duplicate path %q", f.Path)}
		}
		seen[f.Path] = struct{}{}
		total += int64(len(f.Content))
	}
	for p := range seen {
		for dir := path.Dir(p); dir != "."; dir = path.Dir(dir) {
			if _, ok := seen[dir]; ok {
				return &Error{Code: CodeInvalidInput, Field: "files", Message: fmt.Sprintf("path %q conflicts with file %q", p, dir)}
			}
		}
	}
	if total > MaxInputBytes {
		return &Error{Code: CodeLimitExceeded, Field: "files", Message: fmt.Sprintf("input files exceed %d bytes", MaxInputBytes)}
	}
	return nil
}

func insideSandbox(cwd string) bool {
	if !path.IsAbs(cwd) || path.Clean(cwd) != cwd || strings.ContainsRune(cwd, 0) {
		return false
	}
	for _, root := range []string{InputDir, TempDir, OutputDir} {
		if cwd == root || strings.HasPrefix(cwd, root+"/") {
			return true
		}
	}
	return false
}

func validInputPath(p string) bool {
	if p == "" || p == "." || path.IsAbs(p) || path.Clean(p) != p || strings.ContainsRune(p, 0) {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}
