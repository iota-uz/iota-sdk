// Package devrunner provides process supervision for dev environments: run multiple
// processes (templ, tailwind, vite, air, etc.) with consistent output prefixing,
// process-group shutdown (SIGTERM then SIGKILL), and keyboard controls (r restart, c clear, q quit).
//
// Process signaling (stop, forceKill) uses syscall.Kill(-pid, ...) and Setpgid and is Unix-only; this package does not compile on Windows.
package devrunner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ANSI color codes for output prefixing.
const (
	ColorReset   = "\033[0m"
	ColorCyan    = "\033[36m"
	ColorMagenta = "\033[35m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorDim     = "\033[2m"
)

// ProcessSpec describes a process to run.
type ProcessSpec struct {
	Name     string            // display name for log prefix (e.g. "air", "vite")
	Command  string            // executable name
	Args     []string          // arguments (nil treated as empty)
	Dir      string            // working directory
	Color    string            // ANSI color for prefix (e.g. devrunner.ColorYellow)
	Critical bool              // if true, exit of this process triggers shutdown
	Env      map[string]string // optional extra env vars (merged over os.Environ())
}

// RunOptions configures Run behavior.
type RunOptions struct {
	// RestartProcessName is the process name to restart when user presses 'r'. Empty disables restart key.
	RestartProcessName string
	// ShutdownTimeout is how long to wait for processes to exit after SIGTERM before sending SIGKILL. Default 3s.
	ShutdownTimeout time.Duration

	// Preflight: run before starting processes. On failure Run returns (1, err).
	// ProjectRoot is used for package.json and PreflightDeps (e.g. app root or first spec's Dir).
	ProjectRoot string
	// PreflightNodeMajor: if > 0, require Node major >= this; if 0 and PreflightPackageJSONPath set, read from package.json engines.node.
	PreflightNodeMajor int
	// PreflightPnpm: if true, require pnpm in PATH.
	PreflightPnpm bool
	// PreflightPackageJSONPath: path relative to ProjectRoot for engines (e.g. "package.json"). If set and PreflightNodeMajor==0, min node is read from here.
	PreflightPackageJSONPath string
	// PreflightDeps: if true and ProjectRoot set, require single version of react/react-dom (pnpm list).
	PreflightDeps bool
}

// Run runs all processes until context is cancelled or a critical process exits.
// It handles SIGINT/SIGTERM, keyboard (r restart, c clear, q quit), and graceful shutdown
// with escalation to SIGKILL after ShutdownTimeout.
// If preflight is enabled in opts and fails, returns (1, err) without starting processes.
// Otherwise returns (0, nil) on graceful quit, (1, nil) if a critical process exited.
func Run(ctx context.Context, cancel context.CancelFunc, specs []ProcessSpec, opts *RunOptions) (int, error) {
	if opts == nil {
		opts = &RunOptions{}
	}
	shutdownTimeout := opts.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 3 * time.Second
	}

	// Preflight
	if opts.PreflightNodeMajor > 0 || opts.PreflightPnpm || opts.PreflightDeps || opts.PreflightPackageJSONPath != "" {
		nodeMajor := opts.PreflightNodeMajor
		if nodeMajor == 0 && opts.ProjectRoot != "" && opts.PreflightPackageJSONPath != "" {
			if m, err := PreflightFromPackageJSON(opts.ProjectRoot); err == nil && m > 0 {
				nodeMajor = m
			}
		}
		if nodeMajor > 0 {
			if err := PreflightNode(ctx, nodeMajor); err != nil {
				return 1, err
			}
			if out, err := exec.CommandContext(ctx, "node", "-v").Output(); err == nil {
				log.Printf("Node %s", strings.TrimSpace(string(out)))
			}
		}
		if opts.PreflightPnpm {
			if err := PreflightPnpm(ctx); err != nil {
				return 1, err
			}
			if out, err := exec.CommandContext(ctx, "pnpm", "-v").Output(); err == nil {
				log.Printf("pnpm %s", strings.TrimSpace(string(out)))
			}
		}
		if opts.PreflightDeps && opts.ProjectRoot != "" {
			if err := PreflightDeps(ctx, opts.ProjectRoot); err != nil {
				return 1, err
			}
		}
	}

	// Hint only after preflight succeeded (so users don't see it when preflight fails).
	if opts.RestartProcessName != "" {
		log.Printf("\n%sr restart %s · c clear · q quit%s\n\n", ColorDim, opts.RestartProcessName, ColorReset)
	}

	maxLen := 0
	for _, s := range specs {
		if len(s.Name) > maxLen {
			maxLen = len(s.Name)
		}
	}

	managed := make([]*managedProcess, 0, len(specs))
	criticalExitCh := make(chan string, len(specs))
	var wg sync.WaitGroup

	for _, spec := range specs {
		mp := &managedProcess{spec: spec, maxLen: maxLen}
		managed = append(managed, mp)

		wg.Add(1)
		if spec.Critical {
			go func(m *managedProcess) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						criticalExitCh <- fmt.Sprintf("%s (panic: %v)", m.spec.Name, r)
					}
				}()
				m.runCritical(ctx, criticalExitCh)
			}(mp)
		} else {
			go func(m *managedProcess) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						outputMu.Lock()
						fmt.Fprintf(os.Stderr, "%s%s[%-*s]%s panic: %v\n", m.spec.Color, ColorDim, m.maxLen, m.spec.Name, ColorReset, r)
						outputMu.Unlock()
					}
				}()
				m.runAuxiliary(ctx)
			}(mp)
		}
	}

	restoreTerm := enableCbreak(ctx)
	defer restoreTerm()
	keyCh := make(chan byte, 8)
	go readKeys(keyCh)

	exitCode := 0
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case name := <-criticalExitCh:
			outputMu.Lock()
			fmt.Fprintf(os.Stderr, "\n%s exited. Shutting down.\n", name)
			outputMu.Unlock()
			exitCode = 1
			cancel()
			break loop
		case key := <-keyCh:
			switch key {
			case 'r':
				if opts.RestartProcessName != "" {
					for _, m := range managed {
						if m.spec.Name == opts.RestartProcessName {
							m.restart()
							break
						}
					}
				}
			case 'c':
				outputMu.Lock()
				log.Print("\033[2J\033[H")
				outputMu.Unlock()
			case 'q':
				cancel()
				break loop
			}
		}
	}

	for _, m := range managed {
		m.stop()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(shutdownTimeout):
		for _, m := range managed {
			m.forceKill()
		}
		<-done
	}

	return exitCode, nil
}

// NotifyContext returns a context that is cancelled on SIGINT/SIGTERM.
// Call cancel in shutdown to stop listening.
func NotifyContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
}

var outputMu sync.Mutex

type managedProcess struct {
	spec      ProcessSpec
	maxLen    int
	mu        sync.Mutex
	cmd       *exec.Cmd
	restartCh chan struct{}
}

func (m *managedProcess) runCritical(ctx context.Context, exitCh chan<- string) {
	m.restartCh = make(chan struct{}, 1)

	for {
		cmd, err := startProcess(ctx, m.spec, m.maxLen)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start %s: %v\n", m.spec.Name, err)
			exitCh <- m.spec.Name
			return
		}
		m.mu.Lock()
		m.cmd = cmd
		m.mu.Unlock()

		if waitErr := cmd.Wait(); waitErr != nil {
			outputMu.Lock()
			fmt.Fprintf(os.Stderr, "%s%s[%-*s]%s process exit: %v\n", m.spec.Color, ColorDim, m.maxLen, m.spec.Name, ColorReset, waitErr)
			outputMu.Unlock()
		}

		if ctx.Err() != nil {
			return
		}

		select {
		case <-m.restartCh:
			outputMu.Lock()
			fmt.Fprintf(os.Stderr, "%s%s[%-*s]%s restarting...\n", m.spec.Color, ColorDim, m.maxLen, m.spec.Name, ColorReset)
			outputMu.Unlock()
			continue
		default:
			exitCh <- m.spec.Name
			return
		}
	}
}

func (m *managedProcess) runAuxiliary(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		start := time.Now()
		cmd, err := startProcess(ctx, m.spec, m.maxLen)
		if err != nil {
			outputMu.Lock()
			fmt.Fprintf(os.Stderr, "%s%s[%-*s]%s failed to start: %v\n", m.spec.Color, ColorDim, m.maxLen, m.spec.Name, ColorReset, err)
			outputMu.Unlock()
		} else {
			m.mu.Lock()
			m.cmd = cmd
			m.mu.Unlock()

			waitErr := cmd.Wait()

			if ctx.Err() != nil {
				return
			}

			if waitErr == nil {
				outputMu.Lock()
				fmt.Fprintf(os.Stderr, "%s%s[%-*s]%s finished\n", m.spec.Color, ColorDim, m.maxLen, m.spec.Name, ColorReset)
				outputMu.Unlock()
				return
			}

			if time.Since(start) > 10*time.Second {
				backoff = time.Second
			}
		}

		outputMu.Lock()
		fmt.Fprintf(os.Stderr, "%s%s[%-*s]%s crashed, restarting in %s...\n", m.spec.Color, ColorDim, m.maxLen, m.spec.Name, ColorReset, backoff)
		outputMu.Unlock()

		select {
		case <-time.After(backoff):
			backoff = min(backoff*2, maxBackoff)
		case <-ctx.Done():
			return
		}
	}
}

func (m *managedProcess) restart() {
	if m.restartCh != nil {
		select {
		case m.restartCh <- struct{}{}:
		default:
		}
	}
	m.stop()
}

func (m *managedProcess) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		_ = syscall.Kill(-m.cmd.Process.Pid, syscall.SIGTERM)
	}
}

func (m *managedProcess) forceKill() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		_ = syscall.Kill(-m.cmd.Process.Pid, syscall.SIGKILL)
	}
}

func startProcess(ctx context.Context, spec ProcessSpec, padLen int) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = mergeEnv(os.Environ(), spec.Env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	prefix := fmt.Sprintf("[%-*s]", padLen, spec.Name)
	coloredPrefix := spec.Color + prefix + ColorReset

	go logOutput(stdout, coloredPrefix)
	go logOutput(stderr, coloredPrefix)

	return cmd, nil
}

func logOutput(r io.Reader, prefix string) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		outputMu.Lock()
		log.Printf("%s %s\n", prefix, scanner.Text())
		outputMu.Unlock()
	}
}

func enableCbreak(ctx context.Context) func() {
	save := exec.CommandContext(ctx, "stty", "-g")
	save.Stdin = os.Stdin
	state, err := save.Output()
	if err != nil {
		return func() {}
	}

	set := exec.CommandContext(ctx, "stty", "cbreak", "-echo")
	set.Stdin = os.Stdin
	if err := set.Run(); err != nil {
		return func() {}
	}

	return func() {
		restore := exec.CommandContext(context.Background(), "stty", strings.TrimSpace(string(state)))
		restore.Stdin = os.Stdin
		if err := restore.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to restore terminal (stty): %v\n", err)
		}
	}
}

// readKeys reads keypresses in a goroutine; it is not interrupted when context is cancelled, so Run is not reentrant and the goroutine outlives Run until process exit. Fine when Run is invoked once from cmd/dev.
func readKeys(ch chan<- byte) {
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return
		}
		ch <- buf[0]
	}
}

// mergeEnv returns a new environment slice with base entries plus spec overrides; keys in overrides override any existing key in base.
func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	seen := make(map[string]bool)
	out := make([]string, 0, len(base)+len(overrides))
	for _, s := range base {
		i := strings.IndexByte(s, '=')
		if i <= 0 {
			continue
		}
		k := s[:i]
		v := s[i+1:]
		if ov, ok := overrides[k]; ok {
			v = ov
		}
		out = append(out, k+"="+v)
		seen[k] = true
	}
	for k, v := range overrides {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}
