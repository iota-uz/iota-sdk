package devhub

import (
	"context"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/devhub/services"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// StateManager handles all state updates asynchronously
// It acts as a buffer between the service layer and the UI
type StateManager struct {
	mu             sync.RWMutex
	services       []ServiceInfo
	serviceManager *ServiceManager
	systemStats    SystemStats

	// Channels for async updates
	statusUpdateCh   chan StatusUpdate
	resourceUpdateCh chan ResourceUpdate
	systemUpdateCh   chan SystemStats

	// Update intervals
	statusInterval   time.Duration
	resourceInterval time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

type StatusUpdate struct {
	ServiceName  string
	Status       services.ServiceStatus
	ErrorMsg     string
	HealthStatus services.HealthStatus
	StartTime    *time.Time
}

type ResourceUpdate struct {
	ServiceName string
	CPUPercent  float64
	MemoryMB    float64
	PID         int
}

func NewStateManager(serviceManager *ServiceManager) *StateManager {
	ctx, cancel := context.WithCancel(context.Background())

	sm := &StateManager{
		serviceManager:   serviceManager,
		services:         make([]ServiceInfo, 0),
		statusUpdateCh:   make(chan StatusUpdate, 100),
		resourceUpdateCh: make(chan ResourceUpdate, 100),
		systemUpdateCh:   make(chan SystemStats, 10),
		statusInterval:   2 * time.Second,
		resourceInterval: 3 * time.Second,
		ctx:              ctx,
		cancel:           cancel,
	}

	// Initialize services
	sm.services = serviceManager.GetServices()

	return sm
}

// Start begins the async update loops
func (sm *StateManager) Start() {
	// Status update loop
	go sm.statusUpdateLoop()

	// Resource monitoring loop
	go sm.resourceUpdateLoop()

	// System stats monitoring loop
	go sm.systemStatsLoop()

	// Channel processor
	go sm.processUpdates()

	// Fetch initial resource usage immediately
	go sm.fetchResourceUpdates()

	// Fetch initial system stats
	go sm.fetchSystemStats()
}

func (sm *StateManager) statusUpdateLoop() {
	ticker := time.NewTicker(sm.statusInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			// Get basic status updates without expensive operations
			serviceList := sm.serviceManager.GetServicesBasic()
			for _, svc := range serviceList {
				// Send status update
				sm.statusUpdateCh <- StatusUpdate{
					ServiceName:  svc.Name,
					Status:       svc.Status,
					ErrorMsg:     svc.ErrorMsg,
					HealthStatus: svc.HealthStatus,
					StartTime:    svc.StartTime,
				}

				// Don't send PID-only updates here as it might reset CPU/Memory values
				// PID will be updated with the next resource update
			}
		}
	}
}

func (sm *StateManager) resourceUpdateLoop() {
	ticker := time.NewTicker(sm.resourceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			// Get resource usage asynchronously
			go sm.fetchResourceUpdates()
		}
	}
}

func (sm *StateManager) fetchResourceUpdates() {
	serviceList := sm.serviceManager.GetServicesBasic()

	// Fetch resources in parallel
	var wg sync.WaitGroup
	for _, svc := range serviceList {
		if svc.PID > 0 {
			wg.Add(1)
			go func(s ServiceInfo) {
				defer wg.Done()

				usage, err := services.GetProcessResourceUsage(s.PID)
				if err == nil {
					sm.resourceUpdateCh <- ResourceUpdate{
						ServiceName: s.Name,
						CPUPercent:  usage.CPUPercent,
						MemoryMB:    usage.MemoryMB,
						PID:         s.PID,
					}
				} else {
					// Still send PID update even if resource fetch failed
					sm.resourceUpdateCh <- ResourceUpdate{
						ServiceName: s.Name,
						PID:         s.PID,
					}
				}
			}(svc)
		}
	}

	// Don't wait forever
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All done
	case <-time.After(2 * time.Second):
		// Timeout - some resource fetches took too long
	}
}

func (sm *StateManager) systemStatsLoop() {
	ticker := time.NewTicker(sm.resourceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			go sm.fetchSystemStats()
		}
	}
}

func (sm *StateManager) fetchSystemStats() {
	// Get CPU usage with a small interval for accuracy
	cpuPercents, err := cpu.Percent(time.Second, false)
	cpuPercent := float64(0)
	if err == nil && len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	// Get memory stats
	vmStat, err := mem.VirtualMemory()
	memoryMB := float64(0)
	memoryPercent := float64(0)
	if err == nil && vmStat != nil {
		memoryMB = float64(vmStat.Used) / (1024 * 1024)
		memoryPercent = vmStat.UsedPercent
	}

	// Send update
	select {
	case sm.systemUpdateCh <- SystemStats{
		CPUPercent:    cpuPercent,
		MemoryMB:      memoryMB,
		MemoryPercent: memoryPercent,
	}:
	default:
		// Channel full, skip
	}
}

func (sm *StateManager) processUpdates() {
	for {
		select {
		case <-sm.ctx.Done():
			return

		case update := <-sm.statusUpdateCh:
			sm.mu.Lock()
			for i := range sm.services {
				if sm.services[i].Name == update.ServiceName {
					sm.services[i].Status = update.Status
					sm.services[i].ErrorMsg = update.ErrorMsg
					sm.services[i].HealthStatus = update.HealthStatus
					sm.services[i].StartTime = update.StartTime
					sm.services[i].LastUpdate = time.Now()
					break
				}
			}
			sm.mu.Unlock()

		case update := <-sm.resourceUpdateCh:
			sm.mu.Lock()
			for i := range sm.services {
				if sm.services[i].Name == update.ServiceName {
					// Always update PID if provided
					if update.PID > 0 {
						sm.services[i].PID = update.PID
					}
					// Only update resource usage if we have actual values
					// This prevents overwriting existing values with zeros from partial updates
					if update.CPUPercent > 0 || update.MemoryMB > 0 {
						sm.services[i].CPUPercent = update.CPUPercent
						sm.services[i].MemoryMB = update.MemoryMB
					}
					break
				}
			}
			sm.mu.Unlock()

		case stats := <-sm.systemUpdateCh:
			sm.mu.Lock()
			sm.systemStats = stats
			sm.mu.Unlock()
		}
	}
}

// GetServices returns a snapshot of current service states
// This is called by the UI thread and never blocks
func (sm *StateManager) GetServices() []ServiceInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Return a copy to avoid race conditions
	services := make([]ServiceInfo, len(sm.services))
	copy(services, sm.services)
	return services
}

// RefreshService forces an immediate update for a specific service
func (sm *StateManager) RefreshService(serviceName string) {
	// Non-blocking send to avoid UI freezes
	select {
	case sm.statusUpdateCh <- StatusUpdate{ServiceName: serviceName}:
	default:
		// Channel full, skip this update
	}
}

// GetSystemStats returns the current system stats
func (sm *StateManager) GetSystemStats() SystemStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.systemStats
}

// Stop gracefully shuts down the state manager
func (sm *StateManager) Stop() {
	sm.cancel()
	close(sm.statusUpdateCh)
	close(sm.resourceUpdateCh)
	close(sm.systemUpdateCh)
}
