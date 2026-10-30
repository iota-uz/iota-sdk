package shell

import (
	"fmt"
	"path"
	"strings"
)

// Normalize applies contract defaults and rejects requests that must not run.
func Normalize(req Request) (Request, error) {
	if strings.TrimSpace(req.Script) == "" {
		return req, newError(CodeInvalidInput, "script", "script is required")
	}
	if len(req.Script) > MaxScriptBytes {
		return req, newError(CodeLimitExceeded, "script", fmt.Sprintf("script exceeds %d bytes", MaxScriptBytes))
	}

	switch {
	case req.TimeoutMS == 0:
		req.TimeoutMS = DefaultTimeoutMS
	case req.TimeoutMS < 0:
		return req, newError(CodeInvalidInput, "timeout_ms", "timeout_ms must be a positive integer")
	case req.TimeoutMS > MaxTimeoutMS:
		return req, newError(CodeLimitExceeded, "timeout_ms", fmt.Sprintf("timeout_ms exceeds %d", MaxTimeoutMS))
	}

	switch {
	case req.MaxOutputBytes == 0:
		req.MaxOutputBytes = DefaultMaxOutputBytes
	case req.MaxOutputBytes < 0:
		return req, newError(CodeInvalidInput, "max_output_bytes", "max_output_bytes must be a positive integer")
	case req.MaxOutputBytes > MaxOutputBytes:
		return req, newError(CodeLimitExceeded, "max_output_bytes", fmt.Sprintf("max_output_bytes exceeds %d", MaxOutputBytes))
	}

	if req.Cwd == "" {
		req.Cwd = TempDir
	}
	if err := ValidateCwd(req.Cwd); err != nil {
		return req, err
	}

	paths := make([]string, 0, len(req.Files))
	for i, f := range req.Files {
		field := fmt.Sprintf("files[%d]", i)
		if strings.TrimSpace(f.FileRef) == "" {
			return req, newError(CodeInvalidInput, field+".file_ref", "file_ref is required")
		}
		paths = append(paths, f.Path)
	}
	if err := ValidateInputPaths(paths); err != nil {
		return req, err
	}
	return req, nil
}

// ValidateCwd accepts only directories inside the sandbox roots.
func ValidateCwd(cwd string) error {
	if !path.IsAbs(cwd) || path.Clean(cwd) != cwd || strings.ContainsRune(cwd, 0) {
		return newError(CodeInvalidInput, "cwd", "cwd must be a clean absolute path")
	}
	for _, root := range []string{InputDir, TempDir, OutputDir} {
		if cwd == root || strings.HasPrefix(cwd, root+"/") {
			return nil
		}
	}
	return newError(CodeInvalidInput, "cwd", fmt.Sprintf("cwd must be inside %s, %s or %s", InputDir, TempDir, OutputDir))
}

// ValidateInputPaths rejects traversal, non-canonical, duplicate and
// file-versus-directory conflicting paths relative to /input.
func ValidateInputPaths(paths []string) error {
	if len(paths) > MaxInputFiles {
		return newError(CodeLimitExceeded, "files", fmt.Sprintf("files exceed %d entries", MaxInputFiles))
	}
	seen := make(map[string]struct{}, len(paths))
	for i, p := range paths {
		field := fmt.Sprintf("files[%d].path", i)
		if len(p) > MaxInputPathBytes {
			return newError(CodeLimitExceeded, field, fmt.Sprintf("path exceeds %d bytes", MaxInputPathBytes))
		}
		if err := validateInputPath(p); err != nil {
			return newError(CodeInvalidInput, field, err.Error())
		}
		if _, ok := seen[p]; ok {
			return newError(CodeInvalidInput, field, fmt.Sprintf("duplicate path %q", p))
		}
		seen[p] = struct{}{}
	}
	for p := range seen {
		for dir := path.Dir(p); dir != "."; dir = path.Dir(dir) {
			if _, ok := seen[dir]; ok {
				return newError(CodeInvalidInput, "files", fmt.Sprintf("path %q conflicts with file %q", p, dir))
			}
		}
	}
	return nil
}

func validateInputPath(p string) error {
	switch {
	case p == "":
		return fmt.Errorf("path is required")
	case strings.ContainsRune(p, 0):
		return fmt.Errorf("path contains a NUL byte")
	case path.IsAbs(p):
		return fmt.Errorf("path must be relative to %s", InputDir)
	case path.Clean(p) != p || p == ".":
		return fmt.Errorf("path %q is not canonical", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return fmt.Errorf("path %q escapes %s", p, InputDir)
		}
	}
	return nil
}
