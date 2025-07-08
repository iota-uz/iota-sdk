package devhub

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"gopkg.in/yaml.v3"

	"github.com/iota-uz/iota-sdk/pkg/devhub/services"
)

// ConfigLoader abstracts configuration loading
type ConfigLoader interface {
	Load() (*Config, error)
}

type ServiceManager struct {
	services        []services.Service
	servicesByName  map[string]services.Service
	configs         []ServiceConfig
	dependencyOrder []string
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
}

func NewServiceManager(loader ConfigLoader) (*ServiceManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg, err := loader.Load()
	if err != nil {
		cancel()
		return nil, err
	}

	// Convert services to a slice for dependency resolution
	serviceSlice := make([]ServiceConfig, 0, len(cfg.Services))
	serviceNames := make([]string, 0, len(cfg.Services))
	for name, svc := range cfg.Services {
		svc.Name = name // Set the name from the map key
		serviceSlice = append(serviceSlice, svc)
		serviceNames = append(serviceNames, name)
	}

	// Sort service names for consistent ordering
	sort.Strings(serviceNames)

	// Resolve dependencies
	resolver := NewDependencyResolver(serviceSlice)
	startOrder, err := resolver.GetStartOrder()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}

	serviceList := make([]services.Service, 0, len(cfg.Services))
	servicesByName := make(map[string]services.Service)

	// Create services in sorted order
	for _, name := range serviceNames {
		srvCfg := cfg.Services[name]
		// Parse the run command
		runCmd := srvCfg.Run

		// Check for OS-specific override
		if srvCfg.OS != nil && srvCfg.OS[runtime.GOOS] != "" {
			runCmd = srvCfg.OS[runtime.GOOS]
		}

		// Split command and args
		parts := parseCommand(runCmd)
		if len(parts) == 0 {
			cancel()
			return nil, fmt.Errorf("empty command for service %s", name)
		}

		command := parts[0]
		args := parts[1:]

		// Convert port to string
		portStr := ""
		if srvCfg.Port > 0 {
			portStr = fmt.Sprintf("%d", srvCfg.Port)
		}

		cmdService := services.NewCmdService(
			name,
			srvCfg.Desc,
			portStr,
			command,
			args,
		)

		// Set dependencies
		if len(srvCfg.Needs) > 0 {
			cmdService.SetDependencies(srvCfg.Needs)
		}

		// Setup health check if configured
		if srvCfg.Health != nil {
			healthChecker, err := createHealthChecker(srvCfg)
			if err != nil {
				cancel()
				return nil, fmt.Errorf("failed to create health checker for %s: %w", name, err)
			}

			// Parse durations with defaults
			interval := 5 * time.Second
			if srvCfg.Health.Interval != "" {
				if d, err := time.ParseDuration(srvCfg.Health.Interval); err == nil {
					interval = d
				}
			}

			timeout := 3 * time.Second
			if srvCfg.Health.Timeout != "" {
				if d, err := time.ParseDuration(srvCfg.Health.Timeout); err == nil {
					timeout = d
				}
			}

			startPeriod := 10 * time.Second
			if srvCfg.Health.Wait != "" {
				if d, err := time.ParseDuration(srvCfg.Health.Wait); err == nil {
					startPeriod = d
				}
			}

			retries := 3
			if srvCfg.Health.Retries > 0 {
				retries = srvCfg.Health.Retries
			}

			monitor := services.NewHealthMonitor(healthChecker, interval, timeout, retries, startPeriod)
			cmdService.SetHealthMonitor(monitor)
		}

		serviceList = append(serviceList, cmdService)
		servicesByName[name] = cmdService
	}

	return &ServiceManager{
		services:        serviceList,
		servicesByName:  servicesByName,
		configs:         serviceSlice,
		dependencyOrder: startOrder,
		ctx:             ctx,
		cancel:          cancel,
	}, nil
}

// parseCommand splits a command string into command and arguments
// Handles basic quoting for arguments with spaces
func parseCommand(cmd string) []string {
	// Simple implementation - can be enhanced for better quote handling
	var parts []string
	var current string
	inQuote := false
	quote := rune(0)

	for _, r := range cmd {
		switch {
		case !inQuote && (r == '"' || r == '\''):
			inQuote = true
			quote = r
		case inQuote && r == quote:
			inQuote = false
			quote = 0
		case !inQuote && r == ' ':
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		default:
			current += string(r)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// FileConfigLoader loads configuration from a file
type FileConfigLoader struct {
	path string
}

func NewFileConfigLoader(path string) *FileConfigLoader {
	return &FileConfigLoader{path: path}
}

func (l *FileConfigLoader) Load() (*Config, error) {
	return loadConfig(l.path)
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// First unmarshal into a map to handle the new format
	var rawConfig map[string]ServiceConfig
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return nil, err
	}

	cfg := &Config{
		Services: rawConfig,
	}

	return cfg, nil
}

func createHealthChecker(srvCfg ServiceConfig) (services.HealthChecker, error) {
	hc := srvCfg.Health

	// TCP health check
	if hc.TCP > 0 {
		return &services.TCPHealthCheck{
			Host: "localhost",
			Port: fmt.Sprintf("%d", hc.TCP),
		}, nil
	}

	// HTTP health check
	if hc.HTTP != "" {
		check := &services.HTTPHealthCheck{
			URL:    hc.HTTP,
			Method: "GET",
		}

		if hc.Timeout != "" {
			if d, err := time.ParseDuration(hc.Timeout); err == nil {
				check.Timeout = d
			}
		}

		return check, nil
	}

	// Command health check
	if hc.Cmd != "" {
		parts := parseCommand(hc.Cmd)
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty health check command")
		}

		check := &services.CommandHealthCheck{
			Command: parts,
		}

		if hc.Timeout != "" {
			if d, err := time.ParseDuration(hc.Timeout); err == nil {
				check.Timeout = d
			}
		}

		return check, nil
	}

	return nil, fmt.Errorf("no valid health check configuration found")
}

func (sm *ServiceManager) GetServices() []ServiceInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Services are already in sorted order from initialization
	infos := make([]ServiceInfo, len(sm.services))
	for i, service := range sm.services {
		status := service.Status()
		errorMsg := ""
		if err := service.GetError(); err != nil {
			errorMsg = err.Error()
		}

		// Get resource usage and other metadata if available
		var cpuPercent, memoryMB float64
		var pid int
		var startTime *time.Time
		var dependsOn []string
		healthStatus := service.GetHealthStatus()

		// Check if it's a CmdService (which embeds BaseService)
		if cmdService, ok := service.(*services.CmdService); ok {
			pid = cmdService.GetPID()
			startTime = cmdService.GetStartTime()
			dependsOn = cmdService.GetDependencies()

			if usage, err := cmdService.GetResourceUsage(); err == nil {
				cpuPercent = usage.CPUPercent
				memoryMB = usage.MemoryMB
			}
		}

		infos[i] = ServiceInfo{
			Name:         service.Name(),
			Description:  service.Description(),
			Status:       status,
			Port:         service.Port(),
			LastUpdate:   time.Now(),
			ErrorMsg:     errorMsg,
			PID:          pid,
			StartTime:    startTime,
			CPUPercent:   cpuPercent,
			MemoryMB:     memoryMB,
			HealthStatus: healthStatus,
			DependsOn:    dependsOn,
		}
	}

	return infos
}

// GetServicesBasic returns service info without expensive resource monitoring
func (sm *ServiceManager) GetServicesBasic() []ServiceInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	infos := make([]ServiceInfo, len(sm.services))
	for i, service := range sm.services {
		status := service.Status()
		errorMsg := ""
		if err := service.GetError(); err != nil {
			errorMsg = err.Error()
		}

		var pid int
		var startTime *time.Time
		var dependsOn []string
		healthStatus := service.GetHealthStatus()

		// Get basic info without resource usage
		if cmdService, ok := service.(*services.CmdService); ok {
			pid = cmdService.GetPID()
			startTime = cmdService.GetStartTime()
			dependsOn = cmdService.GetDependencies()
		}

		infos[i] = ServiceInfo{
			Name:         service.Name(),
			Description:  service.Description(),
			Status:       status,
			Port:         service.Port(),
			LastUpdate:   time.Now(),
			ErrorMsg:     errorMsg,
			PID:          pid,
			StartTime:    startTime,
			HealthStatus: healthStatus,
			DependsOn:    dependsOn,
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
	sm.mu.RLock()
	if index < 0 || index >= len(sm.services) {
		sm.mu.RUnlock()
		return nil
	}

	service := sm.services[index]
	name := service.Name()
	isRunning := service.IsRunning()
	sm.mu.RUnlock()

	if isRunning {
		// Stop the service asynchronously
		go sm.stopServiceAsync(name)
	} else {
		// Start the service asynchronously
		go sm.startServiceAsync(name)
	}
	return nil
}

// StartService starts a specific service by name
func (sm *ServiceManager) StartService(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.startServiceWithDependencies(name)
}

// StopService stops a specific service by name
func (sm *ServiceManager) StopService(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.stopServiceWithDependents(name)
}

// RestartService restarts a specific service
func (sm *ServiceManager) RestartService(index int) error {
	sm.mu.RLock()
	if index < 0 || index >= len(sm.services) {
		sm.mu.RUnlock()
		return nil
	}

	service := sm.services[index]
	name := service.Name()
	isRunning := service.IsRunning()
	sm.mu.RUnlock()

	// Restart asynchronously
	go sm.restartServiceAsync(name, isRunning)
	return nil
}

func (sm *ServiceManager) restartServiceAsync(name string, wasRunning bool) {
	// Stop the service if it was running
	if wasRunning {
		sm.mu.RLock()
		service, exists := sm.servicesByName[name]
		sm.mu.RUnlock()

		if exists && service.IsRunning() {
			if err := service.Stop(sm.ctx); err != nil {
				// The error will be handled by the service itself
				return
			}

			// Wait a moment for the service to fully stop
			time.Sleep(1 * time.Second)
		}
	}

	// Start the service - errors will be handled by the service itself
	_ = sm.startServiceWithDependenciesAsync(name)
}

func (sm *ServiceManager) startServiceAsync(name string) {
	// Start the service - errors will be handled by the service itself
	_ = sm.startServiceWithDependenciesAsync(name)
}

func (sm *ServiceManager) stopServiceAsync(name string) {
	// Stop the service - errors will be handled by the service itself
	_ = sm.stopServiceWithDependentsAsync(name)
}

func (sm *ServiceManager) startServiceWithDependencies(name string) error {
	service, exists := sm.servicesByName[name]
	if !exists {
		return fmt.Errorf("service not found: %s", name)
	}

	// If already running, nothing to do
	if service.IsRunning() {
		return nil
	}

	// Start dependencies first
	if cmdService, ok := service.(*services.CmdService); ok {
		deps := cmdService.GetDependencies()
		// Set service as queued if it has dependencies
		if len(deps) > 0 {
			cmdService.SetStatus(services.StatusQueued)
		}

		for _, depName := range deps {
			depService, exists := sm.servicesByName[depName]
			if !exists {
				return fmt.Errorf("dependency not found: %s", depName)
			}

			if !depService.IsRunning() {
				if err := sm.startServiceWithDependencies(depName); err != nil {
					return fmt.Errorf("failed to start dependency %s: %w", depName, err)
				}

				// Wait for dependency to become healthy
				if err := sm.waitForHealthy(depName, 30*time.Second); err != nil {
					return fmt.Errorf("dependency %s failed health check: %w", depName, err)
				}
			}
		}
	}

	// Start the service
	return service.Start(sm.ctx)
}

func (sm *ServiceManager) stopServiceWithDependents(name string) error {
	service, exists := sm.servicesByName[name]
	if !exists {
		return fmt.Errorf("service not found: %s", name)
	}

	// Find and stop dependent services first
	resolver := NewDependencyResolver(sm.configs)
	dependents := resolver.GetDependents(name)

	for _, depName := range dependents {
		if depService, exists := sm.servicesByName[depName]; exists && depService.IsRunning() {
			if err := sm.stopServiceWithDependents(depName); err != nil {
				return fmt.Errorf("failed to stop dependent service %s: %w", depName, err)
			}
		}
	}

	// Stop the service itself
	return service.Stop(sm.ctx)
}

func (sm *ServiceManager) waitForHealthy(name string, timeout time.Duration) error {
	service, exists := sm.servicesByName[name]
	if !exists {
		return fmt.Errorf("service not found: %s", name)
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for service to become healthy")
		}

		health := service.GetHealthStatus()
		if health == services.HealthHealthy {
			return nil
		}

		select {
		case <-ticker.C:
			continue
		case <-sm.ctx.Done():
			return sm.ctx.Err()
		}
	}
}

// Async versions that don't hold locks during long operations
func (sm *ServiceManager) startServiceWithDependenciesAsync(name string) error {
	// Get service references without holding lock
	sm.mu.RLock()
	service, exists := sm.servicesByName[name]
	if !exists {
		sm.mu.RUnlock()
		return fmt.Errorf("service not found: %s", name)
	}

	// Check if already running
	if service.IsRunning() {
		sm.mu.RUnlock()
		return nil
	}

	// Get dependencies
	var dependencies []string
	if cmdService, ok := service.(*services.CmdService); ok {
		dependencies = cmdService.GetDependencies()
	}
	sm.mu.RUnlock()

	// Set service as queued if it has dependencies
	if len(dependencies) > 0 {
		if cmdService, ok := service.(*services.CmdService); ok {
			cmdService.SetStatus(services.StatusQueued)
		}
	}

	// Start dependencies first (without holding lock)
	for _, depName := range dependencies {
		sm.mu.RLock()
		depService, exists := sm.servicesByName[depName]
		if !exists {
			sm.mu.RUnlock()
			return fmt.Errorf("dependency not found: %s", depName)
		}
		isRunning := depService.IsRunning()
		sm.mu.RUnlock()

		if !isRunning {
			if err := sm.startServiceWithDependenciesAsync(depName); err != nil {
				return fmt.Errorf("failed to start dependency %s: %w", depName, err)
			}

			// Wait for dependency to become healthy (without lock)
			if err := sm.waitForHealthy(depName, 30*time.Second); err != nil {
				return fmt.Errorf("dependency %s failed health check: %w", depName, err)
			}
		}
	}

	// Start the service itself
	sm.mu.RLock()
	service, exists = sm.servicesByName[name]
	if !exists {
		sm.mu.RUnlock()
		return fmt.Errorf("service not found: %s", name)
	}
	sm.mu.RUnlock()

	return service.Start(sm.ctx)
}

func (sm *ServiceManager) stopServiceWithDependentsAsync(name string) error {
	// Find dependent services without holding lock
	sm.mu.RLock()
	_, exists := sm.servicesByName[name]
	if !exists {
		sm.mu.RUnlock()
		return fmt.Errorf("service not found: %s", name)
	}
	configs := sm.configs
	sm.mu.RUnlock()

	// Find and stop dependent services first
	resolver := NewDependencyResolver(configs)
	dependents := resolver.GetDependents(name)

	for _, depName := range dependents {
		sm.mu.RLock()
		depService, exists := sm.servicesByName[depName]
		if !exists {
			sm.mu.RUnlock()
			continue
		}
		isRunning := depService.IsRunning()
		sm.mu.RUnlock()

		if isRunning {
			if err := sm.stopServiceWithDependentsAsync(depName); err != nil {
				return fmt.Errorf("failed to stop dependent service %s: %w", depName, err)
			}
		}
	}

	// Stop the service itself
	sm.mu.RLock()
	service, exists := sm.servicesByName[name]
	if !exists {
		sm.mu.RUnlock()
		return fmt.Errorf("service not found: %s", name)
	}
	sm.mu.RUnlock()

	return service.Stop(sm.ctx)
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
