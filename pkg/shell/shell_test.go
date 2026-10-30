package shell_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/shell"
)

func requireCode(t *testing.T, err error, code shell.ErrorCode) {
	t.Helper()
	var serr *shell.Error
	require.ErrorAs(t, err, &serr)
	require.Equal(t, code, serr.Code, serr.Error())
}

func TestNormalize_Defaults(t *testing.T) {
	req, err := shell.Normalize(shell.Request{Script: "ls"})
	require.NoError(t, err)
	require.Equal(t, shell.TempDir, req.Cwd)
	require.Equal(t, shell.DefaultTimeoutMS, req.TimeoutMS)
	require.Equal(t, shell.DefaultMaxOutputBytes, req.MaxOutputBytes)
}

func TestNormalize_Rejects(t *testing.T) {
	files := func(paths ...string) []shell.FileRef {
		refs := make([]shell.FileRef, 0, len(paths))
		for _, p := range paths {
			refs = append(refs, shell.FileRef{FileRef: "ref-" + p, Path: p})
		}
		return refs
	}
	tests := []struct {
		name string
		req  shell.Request
		code shell.ErrorCode
	}{
		{"empty script", shell.Request{Script: "  "}, shell.CodeInvalidInput},
		{"script too large", shell.Request{Script: strings.Repeat("x", shell.MaxScriptBytes+1)}, shell.CodeLimitExceeded},
		{"negative timeout", shell.Request{Script: "ls", TimeoutMS: -1}, shell.CodeInvalidInput},
		{"timeout above max", shell.Request{Script: "ls", TimeoutMS: shell.MaxTimeoutMS + 1}, shell.CodeLimitExceeded},
		{"output above max", shell.Request{Script: "ls", MaxOutputBytes: shell.MaxOutputBytes + 1}, shell.CodeLimitExceeded},
		{"cwd outside sandbox", shell.Request{Script: "ls", Cwd: "/etc"}, shell.CodeInvalidInput},
		{"cwd prefix trick", shell.Request{Script: "ls", Cwd: "/tmpfoo"}, shell.CodeInvalidInput},
		{"cwd traversal", shell.Request{Script: "ls", Cwd: "/tmp/../etc"}, shell.CodeInvalidInput},
		{"missing file_ref", shell.Request{Script: "ls", Files: []shell.FileRef{{Path: "a"}}}, shell.CodeInvalidInput},
		{"traversal", shell.Request{Script: "ls", Files: files("../a")}, shell.CodeInvalidInput},
		{"nested traversal", shell.Request{Script: "ls", Files: files("a/../../b")}, shell.CodeInvalidInput},
		{"absolute path", shell.Request{Script: "ls", Files: files("/input/a")}, shell.CodeInvalidInput},
		{"non-canonical path", shell.Request{Script: "ls", Files: files("./a")}, shell.CodeInvalidInput},
		{"duplicate path", shell.Request{Script: "ls", Files: files("a.csv", "a.csv")}, shell.CodeInvalidInput},
		{"file used as directory", shell.Request{Script: "ls", Files: files("a", "a/b.csv")}, shell.CodeInvalidInput},
		{"path too long", shell.Request{Script: "ls", Files: files(strings.Repeat("a/", shell.MaxInputPathBytes/2) + "b")}, shell.CodeLimitExceeded},
		{"too many files", shell.Request{Script: "ls", Files: files(manyPaths(shell.MaxInputFiles + 1)...)}, shell.CodeLimitExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := shell.Normalize(tt.req)
			requireCode(t, err, tt.code)
		})
	}
}

func TestNormalize_AcceptsLimits(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
	}{
		{"path at max bytes", []string{strings.Repeat("a", shell.MaxInputPathBytes)}},
		{"max files", manyPaths(shell.MaxInputFiles)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := make([]shell.FileRef, 0, len(tt.paths))
			for _, p := range tt.paths {
				refs = append(refs, shell.FileRef{FileRef: "ref-" + p, Path: p})
			}
			_, err := shell.Normalize(shell.Request{Script: "ls", Files: refs})
			require.NoError(t, err)
		})
	}
}

func manyPaths(n int) []string {
	paths := make([]string, n)
	for i := range paths {
		paths[i] = fmt.Sprintf("f%d", i)
	}
	return paths
}

type fakeExecutor struct {
	got    *shell.ExecRequest
	result *shell.ExecResult
}

func (f *fakeExecutor) Execute(_ context.Context, req shell.ExecRequest) (*shell.ExecResult, error) {
	f.got = &req
	return f.result, nil
}

type fakeResolver map[string]string

func (f fakeResolver) Open(_ context.Context, ref string) (io.ReadCloser, error) {
	content, ok := f[ref]
	if !ok {
		return nil, shell.ErrFileNotAccessible
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

type fakePublisher struct {
	published []shell.OutputFile
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, file shell.OutputFile) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.published = append(f.published, file)
	return "file-" + file.Path, nil
}

func TestRunner_Run(t *testing.T) {
	exit := 0
	executor := &fakeExecutor{result: &shell.ExecResult{
		RunID:    "run-1",
		ExitCode: &exit,
		Stdout:   "ok\n",
		Outputs:  []shell.OutputFile{{Path: "/output/r.csv", Content: []byte("a,b\n"), MimeType: "text/csv"}},
	}}
	publisher := &fakePublisher{}
	runner := shell.NewRunner(executor, fakeResolver{"ref-1": "data"}, publisher)

	res, err := runner.Run(context.Background(), shell.Request{
		Script: "cat /input/in.txt",
		Files:  []shell.FileRef{{FileRef: "ref-1", Path: "in.txt"}},
	})
	require.NoError(t, err)
	require.Equal(t, []shell.InputFile{{Path: "in.txt", Content: []byte("data")}}, executor.got.Files)
	require.Equal(t, shell.DefaultTimeoutMS, executor.got.TimeoutMS)
	require.Equal(t, []shell.Artifact{{FileRef: "file-/output/r.csv", Path: "/output/r.csv", SizeBytes: 4, MimeType: "text/csv"}}, res.Artifacts)
	require.Equal(t, "ok\n", res.Stdout)
}

func TestRunner_RejectsBeforeExecution(t *testing.T) {
	big := strings.Repeat("x", shell.MaxInputBytes/2+1)
	tests := []struct {
		name  string
		files []shell.FileRef
		code  shell.ErrorCode
	}{
		{"inaccessible file", []shell.FileRef{{FileRef: "other-tenant", Path: "a"}}, shell.CodeForbidden},
		{"inputs above total limit", []shell.FileRef{{FileRef: "big", Path: "a"}, {FileRef: "big", Path: "b"}}, shell.CodeLimitExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &fakeExecutor{}
			runner := shell.NewRunner(executor, fakeResolver{"big": big}, &fakePublisher{})
			_, err := runner.Run(context.Background(), shell.Request{Script: "ls", Files: tt.files})
			requireCode(t, err, tt.code)
			require.Nil(t, executor.got, "executor must not be called")
		})
	}
}

func TestRunner_Run_InputSizeBoundary(t *testing.T) {
	half := strings.Repeat("x", shell.MaxInputBytes/2)
	tests := []struct {
		name    string
		second  string
		wantErr bool
	}{
		{"exactly MaxInputBytes", half, false},
		{"MaxInputBytes plus one", half + "x", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &fakeExecutor{result: &shell.ExecResult{}}
			runner := shell.NewRunner(executor, fakeResolver{"a": half, "b": tt.second}, &fakePublisher{})
			_, err := runner.Run(context.Background(), shell.Request{
				Script: "ls",
				Files:  []shell.FileRef{{FileRef: "a", Path: "a"}, {FileRef: "b", Path: "b"}},
			})
			if tt.wantErr {
				requireCode(t, err, shell.CodeLimitExceeded)
				require.Nil(t, executor.got, "executor must not be called")
				return
			}
			require.NoError(t, err)
			require.NotNil(t, executor.got)
			require.Len(t, executor.got.Files[1].Content, len(tt.second))
		})
	}
}

func TestRunner_PublishFailureReturnsNoArtifact(t *testing.T) {
	exit := 0
	executor := &fakeExecutor{result: &shell.ExecResult{
		ExitCode: &exit,
		Outputs:  []shell.OutputFile{{Path: "/output/r.csv", Content: []byte("x")}},
	}}
	storeErr := errors.New("storage down")
	res, err := shell.NewRunner(executor, fakeResolver{}, &fakePublisher{err: storeErr}).Run(context.Background(), shell.Request{Script: "ls"})
	require.Nil(t, res)
	requireCode(t, err, shell.CodeDependencyUnavailable)
	require.ErrorIs(t, err, storeErr)
}

func TestHTTPExecutor_Execute(t *testing.T) {
	exit := 0
	tests := []struct {
		name    string
		status  int
		body    any
		wantErr shell.ErrorCode
		check   func(t *testing.T, res *shell.ExecResult, err error)
	}{
		{
			name:   "result",
			status: http.StatusOK,
			body:   shell.ExecResult{RunID: "r", ExitCode: &exit, Stdout: "hi"},
			check: func(t *testing.T, res *shell.ExecResult, err error) {
				t.Helper()
				require.NoError(t, err)
				require.Equal(t, "hi", res.Stdout)
			},
		},
		{
			name:   "busy keeps retry hint",
			status: http.StatusTooManyRequests,
			body:   shell.Error{Code: shell.CodeBusy, Message: "queue is full", RetryAfterMS: 1500},
			check: func(t *testing.T, _ *shell.ExecResult, err error) {
				t.Helper()
				var serr *shell.Error
				require.ErrorAs(t, err, &serr)
				require.Equal(t, shell.CodeBusy, serr.Code)
				require.Equal(t, int64(1500), serr.RetryAfterMS)
			},
		},
		{
			name:   "invalid input passes through",
			status: http.StatusBadRequest,
			body:   shell.Error{Code: shell.CodeInvalidInput, Field: "cwd", Message: "bad"},
			check: func(t *testing.T, _ *shell.ExecResult, err error) {
				t.Helper()
				requireCode(t, err, shell.CodeInvalidInput)
			},
		},
		{
			name:   "rejected token is a dependency failure",
			status: http.StatusUnauthorized,
			body:   map[string]string{"code": "unauthorized"},
			check: func(t *testing.T, _ *shell.ExecResult, err error) {
				t.Helper()
				requireCode(t, err, shell.CodeDependencyUnavailable)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != shell.RunsPath || r.Header.Get("Authorization") != "Bearer secret" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				var req shell.ExecRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Script != "ls" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.WriteHeader(tt.status)
				_ = json.NewEncoder(w).Encode(tt.body)
			}))
			defer srv.Close()

			res, err := shell.NewHTTPExecutor(srv.URL, "secret", nil).Execute(context.Background(), shell.ExecRequest{Script: "ls"})
			tt.check(t, res, err)
		})
	}
}

func TestHTTPExecutor_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	_, err := shell.NewHTTPExecutor(url, "secret", nil).Execute(context.Background(), shell.ExecRequest{Script: "ls"})
	requireCode(t, err, shell.CodeDependencyUnavailable)
}

func TestHTTPExecutor_DefaultDeadline(t *testing.T) {
	restore := shell.SetDefaultExecuteTimeout(50 * time.Millisecond)
	defer restore()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	done := make(chan error, 1)
	go func() {
		_, err := shell.NewHTTPExecutor(srv.URL, "secret", nil).Execute(context.Background(), shell.ExecRequest{Script: "ls"})
		done <- err
	}()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not return after the default deadline")
	}
}

func TestResult_NullFields(t *testing.T) {
	raw, err := json.Marshal(shell.Result{RunID: "r", Artifacts: []shell.Artifact{}})
	require.NoError(t, err)
	require.True(t, bytes.Contains(raw, []byte(`"exit_code":null`)) && bytes.Contains(raw, []byte(`"limit_hit":null`)), string(raw))
}
