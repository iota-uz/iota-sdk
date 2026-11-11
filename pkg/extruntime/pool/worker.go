package pool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opSpawn  = serrors.Op("extruntime.pool.spawn")
	opSend   = serrors.Op("extruntime.pool.send")
	opReader = serrors.Op("extruntime.pool.reader")
)

const stderrTailBytes = 4096

// worker is one Node process generation. When the process dies the worker is
// dead for good; the supervisor spawns a fresh worker for the slot.
type worker struct {
	pool *Pool
	slot int
	pid  int

	cmd   *exec.Cmd
	stdin io.WriteCloser
	wmu   sync.Mutex

	exited     chan struct{}
	stderrDone chan struct{}

	// guarded by pool.mu
	used int
	dead bool

	mu     sync.Mutex
	active map[string]*run

	tailMu sync.Mutex
	tail   []byte
}

func (w *worker) send(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return serrors.Wrap(opSend, err)
	}
	w.wmu.Lock()
	defer w.wmu.Unlock()
	if err := writeFrame(w.stdin, payload); err != nil {
		return serrors.Wrap(opSend, err)
	}
	return nil
}

func (w *worker) kill() {
	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
	}
}

func (w *worker) appendTail(line string) {
	w.tailMu.Lock()
	defer w.tailMu.Unlock()
	w.tail = append(w.tail, line...)
	w.tail = append(w.tail, '\n')
	if len(w.tail) > stderrTailBytes {
		w.tail = w.tail[len(w.tail)-stderrTailBytes:]
	}
}

func (w *worker) stderrTail() string {
	w.tailMu.Lock()
	defer w.tailMu.Unlock()
	return strings.TrimSpace(string(w.tail))
}

func (w *worker) register(r *run) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.exited:
		return false
	default:
	}
	w.active[r.id] = r
	return true
}

func (w *worker) unregister(id string) {
	w.mu.Lock()
	delete(w.active, id)
	w.mu.Unlock()
}

func (w *worker) lookup(id string) *run {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.active[id]
}

// spawn starts a node process and completes the hello/ready handshake.
func (p *Pool) spawn(slot int) (*worker, error) {
	args := append(append([]string{}, p.cfg.NodeArgs...), p.cfg.WorkerScript)
	cmd := exec.Command(p.cfg.NodePath, args...)
	cmd.Env = p.workerEnv(slot)
	cmd.Dir = p.cfg.WorkDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, serrors.Wrap(opSpawn, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, serrors.Wrap(opSpawn, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, serrors.Wrap(opSpawn, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, serrors.Wrap(opSpawn, err)
	}

	w := &worker{
		pool:   p,
		slot:   slot,
		pid:    cmd.Process.Pid,
		cmd:    cmd,
		stdin:  stdin,
		exited:     make(chan struct{}),
		stderrDone: make(chan struct{}),
		active:     map[string]*run{},
	}

	go w.drainStderr(stderr)
	reader := bufio.NewReaderSize(stdout, 64*1024)

	type handshake struct {
		pid int
		err error
	}
	hs := make(chan handshake, 1)
	go func() {
		if err := w.send(helloFrame{T: "hello", Protocol: ProtocolVersion}); err != nil {
			hs <- handshake{err: err}
			return
		}
		payload, err := readFrame(reader, p.cfg.MaxFrameBytes)
		if err != nil {
			hs <- handshake{err: serrors.Wrap(opSpawn, err)}
			return
		}
		var head struct {
			T string `json:"t"`
			readyFrame
		}
		if err := json.Unmarshal(payload, &head); err != nil || head.T != "ready" {
			hs <- handshake{err: serrors.WrapContext(opSpawn, ErrProtocol, "first frame is not ready")}
			return
		}
		if head.Protocol != ProtocolVersion {
			hs <- handshake{err: serrors.WrapContext(opSpawn, ErrProtocol, fmt.Sprintf("worker announced %q, host speaks %q", head.Protocol, ProtocolVersion))}
			return
		}
		hs <- handshake{pid: head.PID}
	}()

	timer := time.NewTimer(p.cfg.StartupTimeout)
	defer timer.Stop()
	select {
	case res := <-hs:
		if res.err != nil {
			w.kill()
			_ = cmd.Wait()
			if tail := w.stderrTail(); tail != "" && !errors.Is(res.err, ErrProtocol) {
				return nil, serrors.WrapContext(opSpawn, res.err, "worker stderr: "+tail)
			}
			return nil, res.err
		}
	case <-timer.C:
		w.kill()
		_ = cmd.Wait()
		return nil, serrors.WrapContext(opSpawn, context.DeadlineExceeded, "worker did not become ready; stderr: "+w.stderrTail())
	}

	go w.readLoop(reader)
	return w, nil
}

func (w *worker) drainStderr(r io.Reader) {
	defer close(w.stderrDone)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		w.appendTail(line)
		w.pool.cfg.Logger.Warn("extruntime worker stderr", "worker", w.slot, "pid", w.pid, "line", line)
	}
}

func (w *worker) readLoop(r *bufio.Reader) {
	defer w.onExit()
	for {
		payload, err := readFrame(r, w.pool.cfg.MaxFrameBytes)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				w.pool.cfg.Logger.Warn("extruntime worker read failed", "worker", w.slot, "pid", w.pid, "error", err)
				w.kill()
			}
			return
		}
		var head struct {
			T string `json:"t"`
		}
		if err := json.Unmarshal(payload, &head); err != nil {
			w.pool.cfg.Logger.Warn("extruntime worker sent malformed frame", "worker", w.slot, "error", err)
			w.kill()
			return
		}
		switch head.T {
		case "call":
			var f callFrame
			if json.Unmarshal(payload, &f) != nil {
				w.kill()
				return
			}
			w.handleCall(f)
		case "done":
			var f doneFrame
			if json.Unmarshal(payload, &f) != nil {
				w.kill()
				return
			}
			if r := w.lookup(f.ID); r != nil {
				r.finish(runResult{value: f.Value, ok: f.OK, wire: f.Error})
			}
		case "log":
			var f logFrame
			if json.Unmarshal(payload, &f) == nil {
				w.pool.logFromWorker(w, f)
			}
		default:
			w.pool.cfg.Logger.Warn("extruntime worker sent unknown frame", "worker", w.slot, "t", head.T)
			w.kill()
			return
		}
	}
}

func (w *worker) handleCall(f callFrame) {
	r := w.lookup(f.Inv)
	if r == nil {
		_ = w.send(replyFrame{T: "reply", Inv: f.Inv, ID: f.ID, Error: Internal(CodeHostCall, "unknown or finished invocation")})
		return
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		reply := replyFrame{T: "reply", Inv: f.Inv, ID: f.ID}
		switch {
		case r.suspended.Load():
			reply.Error = Internal(CodeSuspended, "invocation is suspended; host calls are rejected")
		case r.callCtx.Err() != nil:
			reply.Error = Internal(CodeCanceled, "invocation was canceled")
		default:
			value, err := r.calls.Call(r.callCtx, f.Method, f.Args)
			if err != nil {
				reply.Error = toWire(err)
				if reply.Error.Kind == ErrKindSuspend {
					r.suspended.Store(true)
				}
			} else {
				reply.OK = true
				reply.Value = value
			}
		}
		if err := w.send(reply); err != nil && r.callCtx.Err() == nil {
			w.pool.cfg.Logger.Warn("extruntime reply failed", "worker", w.slot, "invocation", f.Inv, "error", err)
		}
	}()
}

// onExit reaps the process, fails every in-flight run with a typed
// ErrWorkerLost diagnostic and releases the slot for the supervisor.
func (w *worker) onExit() {
	select {
	case <-w.stderrDone:
	case <-time.After(time.Second):
	}
	waitErr := w.cmd.Wait()
	cause := "process exited"
	if waitErr != nil {
		cause = waitErr.Error()
	}
	close(w.exited)
	w.pool.markDead(w)

	w.mu.Lock()
	runs := make([]*run, 0, len(w.active))
	for _, r := range w.active {
		runs = append(runs, r)
	}
	w.mu.Unlock()
	for _, r := range runs {
		r.finish(runResult{lost: &WorkerLostError{
			Worker:     w.slot,
			PID:        w.pid,
			Invocation: r.id,
			Cause:      cause,
			Stderr:     w.stderrTail(),
		}})
	}
}

func (p *Pool) workerEnv(slot int) []string {
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/local/bin:/usr/bin:/bin"
	}
	env := []string{
		"PATH=" + path,
		"NODE_ENV=" + p.cfg.NodeEnv,
		"TZ=" + p.cfg.TZ,
		"IOTA_RUNTIME_PROTOCOL=" + ProtocolVersion,
		"IOTA_RUNTIME_WORKER=" + strconv.Itoa(slot),
	}
	return append(env, p.cfg.ExtraEnv...)
}

func (p *Pool) logFromWorker(w *worker, f logFrame) {
	level := slog.LevelInfo
	switch f.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	p.cfg.Logger.Log(context.Background(), level, f.Msg, "worker", w.slot, "invocation", f.Inv)
}
