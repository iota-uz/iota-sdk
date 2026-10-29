package shell

import (
	"context"
	"io"
)

const (
	InputDir  = "/input"
	TempDir   = "/tmp"
	OutputDir = "/output"

	DefaultTimeoutMS      = 5000
	MaxTimeoutMS          = 10000
	DefaultMaxOutputBytes = 32 << 10
	MaxOutputBytes        = 128 << 10
	MaxScriptBytes        = 32 << 10
	MaxInputBytes         = 32 << 20
	MaxInputFiles         = 100
	MaxInputPathBytes     = 1024
)

// Request is the shell tool input.
type Request struct {
	Script         string    `json:"script"`
	Files          []FileRef `json:"files,omitempty"`
	Cwd            string    `json:"cwd,omitempty"`
	TimeoutMS      int       `json:"timeout_ms,omitempty"`
	MaxOutputBytes int       `json:"max_output_bytes,omitempty"`
}

// FileRef places an application file at Path inside /input.
type FileRef struct {
	FileRef string `json:"file_ref"`
	Path    string `json:"path"`
}

type LimitHit string

const (
	LimitTimeout    LimitHit = "timeout"
	LimitMemory     LimitHit = "memory"
	LimitFilesystem LimitHit = "filesystem"
	LimitOutput     LimitHit = "output"
	LimitExecution  LimitHit = "execution"
)

// Result is the shell tool output. ExitCode is nil when the run was terminated
// without a shell exit status.
type Result struct {
	RunID           string     `json:"run_id"`
	ExitCode        *int       `json:"exit_code"`
	Stdout          string     `json:"stdout"`
	Stderr          string     `json:"stderr"`
	StdoutTruncated bool       `json:"stdout_truncated"`
	StderrTruncated bool       `json:"stderr_truncated"`
	LimitHit        *LimitHit  `json:"limit_hit"`
	Artifacts       []Artifact `json:"artifacts"`
	DurationMS      int64      `json:"duration_ms"`
}

type Artifact struct {
	FileRef   string `json:"file_ref"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	MimeType  string `json:"mime_type"`
}

// FileResolver opens an application file for the current caller. It must
// enforce user and tenant access and return ErrFileNotAccessible when the
// reference is unknown or not visible to the caller.
type FileResolver interface {
	Open(ctx context.Context, fileRef string) (io.ReadCloser, error)
}

// ArtifactPublisher stores a file produced in /output as a private
// application file owned by the current caller and returns its file_ref.
type ArtifactPublisher interface {
	Publish(ctx context.Context, file OutputFile) (string, error)
}

// Executor runs a validated request in an isolated worker.
type Executor interface {
	Execute(ctx context.Context, req ExecRequest) (*ExecResult, error)
}

// ExecRequest is the worker run request; Files are already resolved.
type ExecRequest struct {
	Script         string      `json:"script"`
	Files          []InputFile `json:"files,omitempty"`
	Cwd            string      `json:"cwd"`
	TimeoutMS      int         `json:"timeout_ms"`
	MaxOutputBytes int         `json:"max_output_bytes"`
}

// InputFile is a resolved input; Path is relative to /input.
type InputFile struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

// ExecResult is the worker run result; Outputs are the unpublished /output files.
type ExecResult struct {
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

// OutputFile is a file left in /output; Path is absolute inside the sandbox.
type OutputFile struct {
	Path     string `json:"path"`
	Content  []byte `json:"content"`
	MimeType string `json:"mime_type"`
}
