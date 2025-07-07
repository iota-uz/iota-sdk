package services

import (
	"context"
	"os/exec"
)

type PostgresService struct {
	*BaseService
}

func NewPostgresService() *PostgresService {
	return &PostgresService{
		BaseService: NewBaseService("PostgreSQL", "Local PostgreSQL database", "5432"),
	}
}

func (s *PostgresService) Start(ctx context.Context) error {
	return s.runCommand(ctx, "docker", "compose", "-f", "compose.dev.yml", "up", "db")
}

func (s *PostgresService) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != StatusRunning {
		return nil
	}

	s.status = StatusStopping

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", "compose.dev.yml", "down")
	if err := cmd.Run(); err != nil {
		s.lastError = err
		s.status = StatusError
		return err
	}

	s.status = StatusStopped
	return nil
}
