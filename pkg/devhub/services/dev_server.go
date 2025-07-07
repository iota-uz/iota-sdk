package services

import (
	"context"
	"os"
	"path/filepath"
)

type DevServerService struct {
	*BaseService
}

func NewDevServerService() *DevServerService {
	return &DevServerService{
		BaseService: NewBaseService("Dev Server", "Development server", "3200"),
	}
}

func (s *DevServerService) Start(ctx context.Context) error {
	wd, err := os.Getwd()
	if err != nil {
		s.setError(err)
		return err
	}

	serverCmd := filepath.Join(wd, "cmd", "server", "main.go")
	return s.runCommand(ctx, "go", "run", serverCmd)
}
