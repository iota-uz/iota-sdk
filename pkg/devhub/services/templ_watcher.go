package services

import (
	"context"
)

type TemplWatcherService struct {
	*BaseService
}

func NewTemplWatcherService() *TemplWatcherService {
	return &TemplWatcherService{
		BaseService: NewBaseService("Templ Watcher", "Template file watcher", ""),
	}
}

func (s *TemplWatcherService) Start(ctx context.Context) error {
	return s.runCommand(ctx, "templ", "generate", "--watch")
}
