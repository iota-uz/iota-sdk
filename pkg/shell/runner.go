package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Runner is the application entry point of the shell capability.
type Runner struct {
	executor  Executor
	resolver  FileResolver
	publisher ArtifactPublisher
}

func NewRunner(executor Executor, resolver FileResolver, publisher ArtifactPublisher) *Runner {
	return &Runner{executor: executor, resolver: resolver, publisher: publisher}
}

// Run validates and resolves the request, executes it once and publishes the
// /output files. A failed publication fails the call: no file_ref is returned
// for an artifact that was not stored.
func (r *Runner) Run(ctx context.Context, req Request) (*Result, error) {
	req, err := Normalize(req)
	if err != nil {
		return nil, err
	}
	files, err := r.resolve(ctx, req.Files)
	if err != nil {
		return nil, err
	}

	res, err := r.executor.Execute(ctx, ExecRequest{
		Script:         req.Script,
		Files:          files,
		Cwd:            req.Cwd,
		TimeoutMS:      req.TimeoutMS,
		MaxOutputBytes: req.MaxOutputBytes,
	})
	if err != nil {
		return nil, err
	}

	artifacts := make([]Artifact, 0, len(res.Outputs))
	for _, out := range res.Outputs {
		ref, err := r.publisher.Publish(ctx, out)
		if err != nil {
			return nil, &Error{
				Code:    CodeDependencyUnavailable,
				Field:   "artifacts",
				Message: fmt.Sprintf("publishing %s failed", out.Path),
				Err:     err,
			}
		}
		artifacts = append(artifacts, Artifact{
			FileRef:   ref,
			Path:      out.Path,
			SizeBytes: int64(len(out.Content)),
			MimeType:  out.MimeType,
		})
	}

	return &Result{
		RunID:           res.RunID,
		ExitCode:        res.ExitCode,
		Stdout:          res.Stdout,
		Stderr:          res.Stderr,
		StdoutTruncated: res.StdoutTruncated,
		StderrTruncated: res.StderrTruncated,
		LimitHit:        res.LimitHit,
		Artifacts:       artifacts,
		DurationMS:      res.DurationMS,
	}, nil
}

func (r *Runner) resolve(ctx context.Context, refs []FileRef) ([]InputFile, error) {
	files := make([]InputFile, 0, len(refs))
	remaining := int64(MaxInputBytes)
	for i, ref := range refs {
		content, err := r.read(ctx, ref.FileRef, remaining)
		if err != nil {
			if errors.Is(err, ErrFileNotAccessible) {
				return nil, &Error{Code: CodeForbidden, Field: fmt.Sprintf("files[%d].file_ref", i), Message: "file is not accessible", Err: err}
			}
			return nil, err
		}
		if int64(len(content)) > remaining {
			return nil, newError(CodeLimitExceeded, "files", fmt.Sprintf("input files exceed %d bytes", MaxInputBytes))
		}
		remaining -= int64(len(content))
		files = append(files, InputFile{Path: ref.Path, Content: content})
	}
	return files, nil
}

func (r *Runner) read(ctx context.Context, fileRef string, limit int64) ([]byte, error) {
	rc, err := r.resolver.Open(ctx, fileRef)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	content, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, fmt.Errorf("shell: read %s: %w", fileRef, err)
	}
	return content, nil
}
