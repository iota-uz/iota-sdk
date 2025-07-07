package services

import (
	"context"
)

type CSSWatcherService struct {
	*BaseService
}

func NewCSSWatcherService() *CSSWatcherService {
	return &CSSWatcherService{
		BaseService: NewBaseService("CSS Watcher", "CSS file watcher", ""),
	}
}

func (s *CSSWatcherService) Start(ctx context.Context) error {
	return s.runCommand(ctx, "tailwindcss",
		"-c", "tailwind.config.js",
		"-i", "modules/core/presentation/assets/css/main.css",
		"-o", "modules/core/presentation/assets/css/main.min.css",
		"--minify", "--watch")
}
