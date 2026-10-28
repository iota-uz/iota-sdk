package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/shell/worker/internal/protocol"
	"github.com/iota-uz/iota-sdk/pkg/shell/worker/internal/sandbox"
)

const stderrTailBytes = 8 << 10

// outcome is how a child run ended when it produced no result of its own.
type outcome struct {
	limit   protocol.LimitHit
	message string
	detail  string
}

// runChild starts a fresh child process for req and waits for it. The child
// gets no environment, its own process group and a hard deadline; on deadline
// or client cancellation the whole group is killed and reaped before returning.
func (s *Server) runChild(ctx context.Context, req *protocol.Request) (*protocol.Result, *outcome, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	stdout := &cappedBuffer{limit: protocol.MaxResultBytes}
	stderr := &tailBuffer{limit: stderrTailBytes}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(req.TimeoutMS)*time.Millisecond+s.cfg.KillGrace)
	defer cancel()
	cmd := exec.CommandContext(runCtx, s.cfg.ChildCommand[0], s.cfg.ChildCommand[1:]...) //nolint:gosec // argv is fixed by the supervisor configuration
	cmd.Env = []string{}
	cmd.Dir = "/"
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = childProcAttr()
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second

	waitErr := cmd.Run()
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	if cmd.ProcessState == nil {
		return nil, nil, fmt.Errorf("start child: %w", waitErr)
	}
	if waitErr == nil && !stdout.overflow {
		var res protocol.Result
		if err := json.Unmarshal(stdout.Bytes(), &res); err == nil {
			return &res, nil, nil
		}
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return nil, &outcome{
			limit:   protocol.LimitTimeout,
			message: fmt.Sprintf("run terminated: time limit of %d ms exceeded", req.TimeoutMS),
		}, nil
	}
	if isMemoryFailure(cmd.ProcessState, stderr.String()) {
		return nil, &outcome{limit: protocol.LimitMemory, message: "run terminated: memory limit exceeded", detail: stderr.String()}, nil
	}
	return nil, &outcome{limit: protocol.LimitExecution, message: "run terminated: shell process failed", detail: describeExit(waitErr, stdout, stderr)}, nil
}

func isMemoryFailure(state *os.ProcessState, stderr string) bool {
	if strings.Contains(stderr, "out of memory") {
		return true
	}
	if state == nil {
		return false
	}
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}
	// SIGKILL that the supervisor did not send comes from the kernel OOM killer.
	return (status.Exited() && status.ExitStatus() == sandbox.MemoryExitCode) ||
		(status.Signaled() && status.Signal() == syscall.SIGKILL)
}

func describeExit(waitErr error, stdout *cappedBuffer, stderr *tailBuffer) string {
	var exitErr *exec.ExitError
	switch {
	case stdout.overflow:
		return "child result exceeded the size limit"
	case errors.As(waitErr, &exitErr):
		return fmt.Sprintf("%s: %s", exitErr, stderr.String())
	case waitErr != nil:
		return waitErr.Error()
	default:
		return "child returned an unreadable result"
	}
}

// cappedBuffer keeps at most limit bytes and keeps draining the pipe beyond
// that, so an oversized result cannot block the child.
type cappedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.overflow || b.Len()+len(p) > b.limit {
		b.overflow = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

// tailBuffer keeps the last limit bytes of the child's diagnostics.
type tailBuffer struct {
	buf   []byte
	limit int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.limit {
		b.buf = b.buf[len(b.buf)-b.limit:]
	}
	return len(p), nil
}

func (b *tailBuffer) String() string { return string(b.buf) }
