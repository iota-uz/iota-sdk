package devhub

import (
	"context"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/iota-uz/iota-sdk/pkg/devhub/services"
)

type ServiceManager struct {
	services []services.Service
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewServiceManager(configPath string) (*ServiceManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg, err := loadConfig(configPath)
	if err != nil {
		cancel()
		return nil, err
	}

	serviceList := make([]services.Service, 0, len(cfg.Services))
	for _, srvCfg := range cfg.Services {
		serviceList = append(serviceList, services.NewCmdService(
			srvCfg.Name,
			srvCfg.Description,
			srvCfg.Port,
			srvCfg.Command,
			srvCfg.Args,
		))
	}

	return &ServiceManager{
		services: serviceList,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
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

func (sm *ServiceManager) Logs(index int) []byte {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if index < 0 || index >= len(sm.services) {
		return nil
	}

	return sm.services[index].Logs()
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

func (sm *ServiceManager) ClearLogs(index int) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if index < 0 || index >= len(sm.services) {
		return
	}

	sm.services[index].ClearLogs()
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
				// Return a batch update for all services
				services := sm.GetServices()
				updates := make([]StatusUpdateMsg, len(services))
				for i, service := range services {
					updates[i] = StatusUpdateMsg{
						ServiceName: service.Name,
						Status:      service.Status,
						ErrorMsg:    service.ErrorMsg,
					}
				}
				return BatchStatusUpdateMsg{Updates: updates}
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
			_ = service.Stop(ctx)
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
