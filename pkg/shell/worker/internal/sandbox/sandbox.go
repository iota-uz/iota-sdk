// Package sandbox runs one shell request in a fresh gbash runtime backed only
// by an in-memory VFS: /input (read-only inputs), /tmp (scratch) and /output
// (files to publish). Nothing from the host filesystem, process table or
// network is attached.
package sandbox

import (
	"context"
	"errors"
	"io"
	stdfs "io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ewhauser/gbash"
	gbfs "github.com/ewhauser/gbash/fs"
	"github.com/ewhauser/gbash/policy"

	"github.com/iota-uz/iota-sdk/pkg/shell/worker/internal/protocol"
)

// Budgets bound the work of one run, nested executions included.
type Budgets struct {
	Commands          int64
	LoopIterations    int64
	GlobOperations    int64
	SubstitutionDepth int64
	ScratchBytes      int64
}

var DefaultBudgets = Budgets{
	Commands:          10000,
	LoopIterations:    10000,
	GlobOperations:    100000,
	SubstitutionDepth: 50,
	ScratchBytes:      protocol.MaxScratchBytes,
}

// gbash reports its own loop/glob/substitution budget failures only as text.
const gbashBudgetMarker = "increase policy.Limits."

// Run executes req and returns its result without run_id and duration, which
// the supervisor owns.
func Run(ctx context.Context, req *protocol.Request, budgets Budgets) (*protocol.Result, error) {
	registry, err := newRegistry()
	if err != nil {
		return nil, err
	}

	files := make(gbfs.InitialFiles, len(req.Files))
	for _, f := range req.Files {
		files[path.Join(protocol.InputDir, f.Path)] = gbfs.InitialFile{Content: f.Content, Mode: 0o444}
	}
	var quota *quotaFS
	factory := gbfs.FactoryFunc(func(ctx context.Context) (gbfs.FileSystem, error) {
		base, err := gbfs.SeededMemory(files).New(ctx)
		if err != nil {
			return nil, err
		}
		for _, dir := range []string{protocol.InputDir, protocol.OutputDir} {
			if err := base.MkdirAll(ctx, dir, 0o755); err != nil {
				return nil, err
			}
		}
		quota = newQuotaFS(base, budgets.ScratchBytes)
		return quota, nil
	})

	pol := &runPolicy{
		Static: policy.NewStatic(&policy.Config{
			AllowedCommands: registry.Names(),
			ReadRoots:       []string{"/"},
			WriteRoots:      []string{protocol.TempDir, protocol.OutputDir, "/dev/null"},
			SymlinkMode:     policy.SymlinkDeny,
			Limits: policy.Limits{
				MaxCommandCount:      budgets.Commands,
				MaxLoopIterations:    budgets.LoopIterations,
				MaxGlobOperations:    budgets.GlobOperations,
				MaxSubstitutionDepth: budgets.SubstitutionDepth,
				MaxStdoutBytes:       int64(req.MaxOutputBytes),
				MaxStderrBytes:       int64(req.MaxOutputBytes),
				MaxFileBytes:         budgets.ScratchBytes,
			},
		}),
		maxCommands: budgets.Commands,
	}

	runtime, err := gbash.New(
		gbash.WithRegistry(registry),
		gbash.WithPolicy(pol),
		gbash.WithFileSystem(gbash.CustomFileSystem(factory, req.Cwd)),
	)
	if err != nil {
		return nil, err
	}
	session, err := runtime.NewSession(ctx)
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(req.TimeoutMS)*time.Millisecond)
	defer cancel()
	exec, execErr := session.Exec(runCtx, &gbash.ExecutionRequest{Script: req.Script})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	res := &protocol.Result{Outputs: []protocol.OutputFile{}}
	var limit protocol.LimitHit
	switch {
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		limit = protocol.LimitTimeout
	case quota.exceeded.Load():
		limit = protocol.LimitFilesystem
	case pol.exhausted.Load():
		limit = protocol.LimitExecution
	}
	if execErr != nil {
		res.Stderr = execErr.Error()
		if limit == "" {
			limit = protocol.LimitExecution
		}
	} else {
		if limit == "" && strings.Contains(exec.Stderr, gbashBudgetMarker) {
			limit = protocol.LimitExecution
		}
		if limit != protocol.LimitTimeout {
			code := exec.ExitCode
			res.ExitCode = &code
		}
		res.Stdout, res.StdoutTruncated = clip(exec.Stdout, req.MaxOutputBytes)
		res.Stderr, res.StderrTruncated = clip(exec.Stderr, req.MaxOutputBytes-len(res.Stdout))
		res.StdoutTruncated = res.StdoutTruncated || exec.StdoutTruncated
		res.StderrTruncated = res.StderrTruncated || exec.StderrTruncated
		if limit == "" && (res.StdoutTruncated || res.StderrTruncated) {
			limit = protocol.LimitOutput
		}
	}
	if limit != "" {
		res.LimitHit = &limit
	}

	if res.Outputs, err = collectOutputs(ctx, session.FileSystem(), protocol.OutputDir); err != nil {
		return nil, err
	}
	return res, nil
}

// clip cuts s to at most n bytes on a UTF-8 boundary.
func clip(s string, n int) (string, bool) {
	if len(s) <= n {
		return s, false
	}
	n = max(n, 0)
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n], true
}

// collectOutputs returns regular files under dir; symlinks and special files
// are skipped so they cannot expose anything outside /output.
func collectOutputs(ctx context.Context, fsys gbfs.FileSystem, dir string) ([]protocol.OutputFile, error) {
	entries, err := fsys.ReadDir(ctx, dir)
	if err != nil {
		if errors.Is(err, stdfs.ErrNotExist) {
			return []protocol.OutputFile{}, nil
		}
		return nil, err
	}
	outputs := []protocol.OutputFile{}
	for _, entry := range entries {
		name := path.Join(dir, entry.Name())
		info, err := fsys.Lstat(ctx, name)
		if err != nil {
			return nil, err
		}
		switch {
		case info.IsDir():
			nested, err := collectOutputs(ctx, fsys, name)
			if err != nil {
				return nil, err
			}
			outputs = append(outputs, nested...)
		case info.Mode().IsRegular():
			content, err := readFile(ctx, fsys, name)
			if err != nil {
				return nil, err
			}
			outputs = append(outputs, protocol.OutputFile{Path: name, Content: content, MimeType: mimeType(name, content)})
		}
	}
	return outputs, nil
}

func readFile(ctx context.Context, fsys gbfs.FileSystem, name string) ([]byte, error) {
	f, err := fsys.Open(ctx, name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

func mimeType(name string, content []byte) string {
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		return t
	}
	return http.DetectContentType(content)
}
