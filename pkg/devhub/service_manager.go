package devhub

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/iota-uz/iota-sdk/pkg/devhub/services"
)

type ServiceManager struct {
	services []services.Service
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewServiceManager() *ServiceManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &ServiceManager{
		services: []services.Service{
			services.NewDevServerService(),
			services.NewTemplWatcherService(),
			services.NewCSSWatcherService(),
			services.NewPostgresService(),
			services.NewTunnelService(),
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (sm *ServiceManager) GetServices() []ServiceInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	infos := make([]ServiceInfo, len(sm.services))
	for i, service := range sm.services {
		status := convertServiceStatus(service.Status())
		errorMsg := ""
		if err := service.GetError(); err != nil {
			errorMsg = err.Error()
		}

		infos[i] = ServiceInfo{
			Name:        service.Name(),
			Description: service.Description(),
			Status:      status,
			Port:        service.Port(),
			LastUpdate:  time.Now(),
			ErrorMsg:    errorMsg,
		}
	}

	return infos
}

func (sm *ServiceManager) ToggleService(index int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if index < 0 || index >= len(sm.services) {
		return nil
	}

	service := sm.services[index]
	if service.IsRunning() {
		return service.Stop(sm.ctx)
	}

	return service.Start(sm.ctx)
}

func (sm *ServiceManager) StartMonitoring() tea.Cmd {
	return func() tea.Msg {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-sm.ctx.Done():
				return nil
			case <-ticker.C:
				services := sm.GetServices()
				for _, service := range services {
					return StatusUpdateMsg{
						ServiceName: service.Name,
						Status:      service.Status,
						ErrorMsg:    service.ErrorMsg,
					}
				}
			}
		}
	}
}

func (sm *ServiceManager) Shutdown() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.cancel()

	for _, service := range sm.services {
		if service.IsRunning() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			service.Stop(ctx)
			cancel()
		}
	}
}

func convertServiceStatus(status services.ServiceStatus) ServiceStatus {
	switch status {
	case services.StatusStopped:
		return StatusStopped
	case services.StatusStarting:
		return StatusStarting
	case services.StatusRunning:
		return StatusRunning
	case services.StatusStopping:
		return StatusStopping
	case services.StatusError:
		return StatusError
	default:
		return StatusStopped
	}
}
