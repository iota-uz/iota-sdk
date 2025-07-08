package services

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ServiceInfo provides basic service metadata
type ServiceInfo interface {
	Name() string
	Description() string
	Port() string
}

// ServiceLifecycle manages service start/stop operations
type ServiceLifecycle interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// ServiceStatus provides service state information
type ServiceStatusProvider interface {
	IsRunning() bool
	Status() ServiceStatus
	GetError() error
	GetHealthStatus() HealthStatus
}

// ServiceLogger provides logging capabilities
type ServiceLogger interface {
	Logs() []byte
	ClearLogs()
}

// Service combines all service capabilities
type Service interface {
	ServiceInfo
	ServiceLifecycle
	ServiceStatusProvider
	ServiceLogger
}

type ServiceStatus int

const (
	StatusStopped ServiceStatus = iota
	StatusQueued
	StatusStarting
	StatusRunning
	StatusStopping
	StatusError
)

func (s ServiceStatus) String() string {
	switch s {
	case StatusStopped:
		return "Stopped"
	case StatusQueued:
		return "Queued"
	case StatusStarting:
		return "Starting"
	case StatusRunning:
		return "Running"
	case StatusStopping:
		return "Stopping"
	case StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

type BaseService struct {
	name          string
	description   string
	port          string
	cmd           *exec.Cmd
	status        ServiceStatus
	mu            sync.RWMutex
	lastError     error
	cancelFunc    context.CancelFunc
	logBuffer     *CircularLogBuffer
	startTime     *time.Time
	pid           int
	healthMonitor *HealthMonitor
	healthStatus  HealthStatus
}

func NewBaseService(name, description, port string) *BaseService {
	return &BaseService{
		name:         name,
		description:  description,
		port:         port,
		status:       StatusStopped,
		healthStatus: HealthUnknown,
		logBuffer:    NewCircularLogBuffer(),
	}
}

func (s *BaseService) SetHealthMonitor(monitor *HealthMonitor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.healthMonitor = monitor
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
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we think we're running, double-check the process is alive
	if (s.status == StatusRunning || s.status == StatusStarting) && s.cmd != nil && s.cmd.Process != nil {
		// Try to send signal 0 to check if process is alive
		if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			// Process is dead
			s.status = StatusStopped
			s.lastError = fmt.Errorf("process terminated unexpectedly: %w", err)
			return s.status
		}
	}

	return s.status
}

func (s *BaseService) GetError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

func (s *BaseService) GetPID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pid
}

func (s *BaseService) GetStartTime() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.startTime == nil {
		return nil
	}
	t := *s.startTime
	return &t
}

func (s *BaseService) GetResourceUsage() (*ResourceUsage, error) {
	s.mu.RLock()
	pid := s.pid
	s.mu.RUnlock()

	if pid <= 0 {
		return &ResourceUsage{}, nil
	}

	return GetTotalResourceUsage(pid)
}

func (s *BaseService) GetHealthStatus() HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.healthMonitor != nil {
		return s.healthMonitor.GetStatus()
	}

	// If no health monitor, return healthy if running, unknown otherwise
	if s.status == StatusRunning {
		return HealthHealthy
	}
	return HealthUnknown
}

func (s *BaseService) SetStatus(status ServiceStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *BaseService) Logs() []byte {
	return s.logBuffer.Bytes()
}

func (s *BaseService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we think we're running
	if s.status != StatusRunning && s.status != StatusStarting {
		return false
	}

	// Double-check if the process is actually still alive
	if s.cmd != nil && s.cmd.Process != nil {
		// Try to send signal 0 to check if process is alive
		if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			// Process is dead
			s.status = StatusStopped
			s.lastError = fmt.Errorf("process terminated unexpectedly: %w", err)
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
	if _, err := s.logBuffer.Write([]byte(fmt.Sprintf("%s[DevHub] Starting: %s %s\n", timestamp, cmdPath, strings.Join(args, " ")))); err != nil {
		// Log to stderr if buffer write fails
		fmt.Fprintf(os.Stderr, "Failed to write to log buffer: %v\n", err)
	}

	s.cmd = exec.CommandContext(cmdCtx, cmdPath, args...)

	// Set working directory
	wd, _ := os.Getwd()
	s.cmd.Dir = wd

	// Create a new process group so we can kill all child processes
	s.cmd.SysProcAttr = &syscall.SysProcAttr{}
	setSysProcAttr(s.cmd.SysProcAttr)

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

	// Track PID and start time
	s.mu.Lock()
	s.pid = s.cmd.Process.Pid
	now := time.Now()
	s.startTime = &now
	s.mu.Unlock()

	// Read stdout
	go func() {
		defer func() { _ = stdout.Close() }()
		reader := bufio.NewReader(stdout)
		for {
			select {
			case <-cmdCtx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					// EOF is expected when process terminates
					if err == io.EOF {
						return
					}
					// Check if context was cancelled
					if cmdCtx.Err() != nil {
						return
					}
					// Only log unexpected errors
					if !errors.Is(err, os.ErrClosed) && !errors.Is(err, syscall.EPIPE) {
						fmt.Fprintf(os.Stderr, "Error reading stdout: %v\n", err)
					}
					return
				}
				timestamp := time.Now().Format("[15:04:05] ")
				if _, err := s.logBuffer.Write([]byte(fmt.Sprintf("%s%s", timestamp, line))); err != nil {
					// Buffer write failed, stop reading
					return
				}
			}
		}
	}()

	// Read stderr
	go func() {
		defer func() { _ = stderr.Close() }()
		reader := bufio.NewReader(stderr)
		for {
			select {
			case <-cmdCtx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					// EOF is expected when process terminates
					if err == io.EOF {
						return
					}
					// Check if context was cancelled
					if cmdCtx.Err() != nil {
						return
					}
					// Only log unexpected errors
					if !errors.Is(err, os.ErrClosed) && !errors.Is(err, syscall.EPIPE) {
						fmt.Fprintf(os.Stderr, "Error reading stderr: %v\n", err)
					}
					return
				}
				timestamp := time.Now().Format("[15:04:05] ")
				if _, err := s.logBuffer.Write([]byte(fmt.Sprintf("%s%s", timestamp, line))); err != nil {
					// Buffer write failed, stop reading
					return
				}
			}
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

			// Start health monitoring if configured
			s.mu.RLock()
			monitor := s.healthMonitor
			s.mu.RUnlock()

			if monitor != nil {
				go monitor.Start(cmdCtx)
			}
		} else {
			s.mu.RUnlock()
		}
	}()

	// Monitor process exit
	go func() {
		// Wait for the process to exit
		err := s.cmd.Wait()

		// Update status
		s.mu.Lock()
		if s.status == StatusRunning || s.status == StatusStarting {
			s.status = StatusStopped
		}
		s.cancelFunc = nil
		s.pid = 0
		s.startTime = nil

		// Only set error if it wasn't a deliberate stop
		if err != nil && cmdCtx.Err() == nil && s.status != StatusStopping {
			s.lastError = err
			// Log unexpected exit
			timestamp := time.Now().Format("[15:04:05] ")
			_, _ = s.logBuffer.Write([]byte(fmt.Sprintf("%s[DevHub] Process exited with error: %v\n", timestamp, err)))
		}
		s.mu.Unlock()
	}()

	return nil
}

func (s *BaseService) Stop(ctx context.Context) error {
	s.mu.Lock()

	if s.status != StatusRunning && s.status != StatusStarting {
		s.mu.Unlock()
		return nil
	}

	s.status = StatusStopping
	cancelFunc := s.cancelFunc
	pid := s.pid
	s.mu.Unlock()

	// Cancel the context - this will signal the process to stop
	if cancelFunc != nil {
		cancelFunc()
	}

	// Wait for the process to stop with timeout
	stopTimeout := 10 * time.Second
	stopTimer := time.NewTimer(stopTimeout)
	defer stopTimer.Stop()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopTimer.C:
			// Timeout - force kill the process group
			if runtime.GOOS != "windows" && pid > 0 {
				// Kill the entire process group
				if err := killProcessGroup(pid); err != nil {
					// Process might already be gone
					if !errors.Is(err, syscall.ESRCH) {
						return fmt.Errorf("failed to kill process group: %w", err)
					}
				}
			}

			// Give it a moment to die
			time.Sleep(100 * time.Millisecond)

			s.mu.Lock()
			s.status = StatusStopped
			s.pid = 0
			s.startTime = nil
			s.mu.Unlock()
			return nil

		case <-ticker.C:
			// Check if process has stopped
			s.mu.RLock()
			currentStatus := s.status
			s.mu.RUnlock()

			if currentStatus == StatusStopped {
				return nil
			}
		}
	}
}

func (s *BaseService) ClearLogs() {
	s.logBuffer.Reset()
	// Add a timestamp message indicating logs were cleared
	timestamp := time.Now().Format("[15:04:05] ")
	if _, err := s.logBuffer.Write([]byte(fmt.Sprintf("%s[DevHub] Logs cleared\n", timestamp))); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write clear message to log buffer: %v\n", err)
	}
}
