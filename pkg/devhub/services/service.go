package services

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

type Service interface {
	Name() string
	Description() string
	Port() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
	Status() ServiceStatus
	GetError() error
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
	return s.status
}

func (s *BaseService) GetError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

func (s *BaseService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
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

	cmdCtx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	s.cmd = exec.CommandContext(cmdCtx, command, args...)

	if err := s.cmd.Start(); err != nil {
		s.setError(err)
		return err
	}

	s.setStatus(StatusRunning)

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

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	if s.cmd != nil && s.cmd.Process != nil {
		if err := s.cmd.Process.Kill(); err != nil {
			s.lastError = err
			s.status = StatusError
			return err
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		s.status = StatusStopped
		return nil
	}
}
