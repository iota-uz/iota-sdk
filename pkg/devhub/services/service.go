package services

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// A simple thread-safe buffer
type threadSafeBuffer struct {
	b  bytes.Buffer
	mu sync.Mutex
}

func (b *threadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *threadSafeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Return a copy to avoid race conditions on the slice
	buf := make([]byte, len(b.b.Bytes()))
	copy(buf, b.b.Bytes())
	return buf
}

func (b *threadSafeBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.b.Reset()
}

type Service interface {
	Name() string
	Description() string
	Port() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
	Status() ServiceStatus
	GetError() error
	Logs() []byte
	ClearLogs()
}

type ServiceStatus int

const (
	StatusStopped ServiceStatus = iota
	StatusStarting
	StatusRunning
	StatusStopping
	StatusError
)

type BaseService struct {
	name        string
	description string
	port        string
	cmd         *exec.Cmd
	status      ServiceStatus
	mu          sync.RWMutex
	lastError   error
	cancelFunc  context.CancelFunc
	logBuffer   threadSafeBuffer
}

func NewBaseService(name, description, port string) *BaseService {
	return &BaseService{
		name:        name,
		description: description,
		port:        port,
		status:      StatusStopped,
	}
}

func (s *BaseService) Name() string {
	return s.name
}

func (s *BaseService) Description() string {
	return s.description
}

func (s *BaseService) Port() string {
	return s.port
}

func (s *BaseService) Status() ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// If we think we're running, double-check the process is alive
	if (s.status == StatusRunning || s.status == StatusStarting) && s.cmd != nil && s.cmd.Process != nil {
		// Try to send signal 0 to check if process is alive
		if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			// Process is dead
			return StatusStopped
		}
	}

	return s.status
}

func (s *BaseService) GetError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

func (s *BaseService) Logs() []byte {
	return s.logBuffer.Bytes()
}

func (s *BaseService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if we think we're running
	if s.status != StatusRunning && s.status != StatusStarting {
		return false
	}

	// Double-check if the process is actually still alive
	if s.cmd != nil && s.cmd.Process != nil {
		// Try to send signal 0 to check if process is alive
		if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			// Process is dead
			return false
		}
	}

	return s.status == StatusRunning
}

func (s *BaseService) setStatus(status ServiceStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *BaseService) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = err
	if err != nil {
		s.status = StatusError
	}
}

func (s *BaseService) runCommand(ctx context.Context, command string, args ...string) error {
	s.setStatus(StatusStarting)
	s.setError(nil)
	s.logBuffer.Reset()

	cmdCtx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	// Resolve command path
	cmdPath, err := exec.LookPath(command)
	if err != nil {
		cmdPath = command
	}

	// Log command being executed
	timestamp := time.Now().Format("[15:04:05] ")
	_, _ = s.logBuffer.Write([]byte(fmt.Sprintf("%s[DevHub] Starting: %s %s\n", timestamp, cmdPath, strings.Join(args, " "))))

	s.cmd = exec.CommandContext(cmdCtx, cmdPath, args...)

	// Set working directory
	wd, _ := os.Getwd()
	s.cmd.Dir = wd

	// Create pipes for stdout and stderr
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		s.setError(err)
		return err
	}

	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		s.setError(err)
		return err
	}

	// Start the command
	if err := s.cmd.Start(); err != nil {
		s.setError(err)
		return err
	}

	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			timestamp := time.Now().Format("[15:04:05] ")
			_, _ = s.logBuffer.Write([]byte(timestamp))
			_, _ = s.logBuffer.Write(scanner.Bytes())
			_, _ = s.logBuffer.Write([]byte("\n"))
		}
	}()

	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			timestamp := time.Now().Format("[15:04:05] ")
			_, _ = s.logBuffer.Write([]byte(timestamp))
			_, _ = s.logBuffer.Write(scanner.Bytes())
			_, _ = s.logBuffer.Write([]byte("\n"))
		}
	}()

	// Keep starting status for a moment to show the spinner
	go func() {
		time.Sleep(2 * time.Second)
		// Only set to running if the process is still active
		s.mu.RLock()
		if s.status == StatusStarting && s.cmd != nil && s.cmd.Process != nil {
			s.mu.RUnlock()
			s.setStatus(StatusRunning)
		} else {
			s.mu.RUnlock()
		}
	}()

	go func() {
		defer func() {
			s.setStatus(StatusStopped)
			s.cancelFunc = nil
		}()

		if err := s.cmd.Wait(); err != nil {
			if cmdCtx.Err() == nil {
				s.setError(err)
			}
		}
	}()

	return nil
}

func (s *BaseService) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != StatusRunning {
		return nil
	}

	s.status = StatusStopping

	go func() {
		if s.cmd != nil && s.cmd.Process != nil {
			// Kill the process - on Darwin, kill the process directly
			if runtime.GOOS == "darwin" {
				if err := s.cmd.Process.Kill(); err != nil {
					s.mu.Lock()
					s.lastError = err
					s.status = StatusError
					s.mu.Unlock()
					return
				}
			} else {
				// Kill the entire process group on other platforms
				if err := syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL); err != nil {
					s.mu.Lock()
					s.lastError = err
					s.status = StatusError
					s.mu.Unlock()
					return
				}
			}
		}

		if s.cancelFunc != nil {
			s.cancelFunc()
		}

		time.Sleep(2 * time.Second)

		s.mu.Lock()
		s.status = StatusStopped
		s.mu.Unlock()
	}()

	return nil
}

func (s *BaseService) ClearLogs() {
	s.logBuffer.Reset()
	// Add a timestamp message indicating logs were cleared
	timestamp := time.Now().Format("[15:04:05] ")
	_, _ = s.logBuffer.Write([]byte(fmt.Sprintf("%s[DevHub] Logs cleared\n", timestamp)))
}
