package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

type HealthStatus string

const (
	HealthStatusHealthy  HealthStatus = "healthy"
	HealthStatusDegraded HealthStatus = "degraded"
	HealthStatusDown     HealthStatus = "down"
)

type HealthResponse struct {
	Status    HealthStatus   `json:"status"`
	Timestamp string         `json:"timestamp"`
	Version   string         `json:"version,omitempty"`
	Uptime    string         `json:"uptime,omitempty"`
	Checks    map[string]any `json:"checks"`
}

type ComponentHealth struct {
	Status       HealthStatus   `json:"status"`
	ResponseTime string         `json:"responseTime,omitempty"`
	Error        string         `json:"error,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

type DatabaseDetails struct {
	ActiveConnections int `json:"activeConnections"`
	IdleConnections   int `json:"idleConnections"`
	MaxConnections    int `json:"maxConnections"`
}

type SystemDetails struct {
	MemoryUsageMB   uint64 `json:"memoryUsageMB"`
	CPUUsagePercent int    `json:"cpuUsagePercent"`
	Goroutines      int    `json:"goroutines"`
}

var startTime = time.Now()

func NewHealthController(app application.Application) application.Controller {
	return &HealthController{
		app: app,
	}
}

type HealthController struct {
	app application.Application
}

func (c *HealthController) Key() string {
	return "/health"
}

func (c *HealthController) Register(r *mux.Router) {
	router := r.Methods(http.MethodGet).Subrouter()
	router.HandleFunc("/health", c.Get)
}

func (c *HealthController) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)
	conf := configuration.Use()

	w.Header().Set("Content-Type", "application/json")

	// Simple mode: quick health check with minimal overhead
	if !conf.HealthDetailed {
		status := "healthy"
		httpStatus := http.StatusOK

		// Quick DB ping with timeout to determine health
		if err := c.quickDBCheck(ctx); err != nil {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
			logger.Warnf("Health check failed: %v", err)
		}
		if err := c.app.Spotlight().Readiness(ctx); err != nil {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
			logger.Warnf("Spotlight health check failed: %v", err)
		}

		w.WriteHeader(httpStatus)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": status}); err != nil {
			logger.Errorf("Failed to write simple health response: %v", err)
		}
		return
	}

	// Detailed mode: comprehensive health checks with metrics
	response := c.performHealthChecks(ctx)

	var status int
	switch response.Status {
	case HealthStatusHealthy:
		status = http.StatusOK
	case HealthStatusDegraded:
		status = http.StatusOK
	case HealthStatusDown:
		status = http.StatusServiceUnavailable
	}

	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Errorf("Failed to write health response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// quickDBCheck performs a fast database ping for simple health check
func (c *HealthController) quickDBCheck(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	db := c.app.DB()
	if db == nil {
		return fmt.Errorf("database connection pool not available")
	}

	var result int
	return db.QueryRow(timeoutCtx, "SELECT 1").Scan(&result)
}

func (c *HealthController) performHealthChecks(ctx context.Context) *HealthResponse {
	checks := make(map[string]any)
	var wg sync.WaitGroup
	var mu sync.Mutex

	overallStatus := HealthStatusHealthy

	wg.Add(3)

	go func() {
		defer wg.Done()
		dbHealth := c.checkDatabase(ctx)
		mu.Lock()
		checks["database"] = dbHealth
		if dbHealth.Status == HealthStatusDown {
			overallStatus = HealthStatusDown
		} else if dbHealth.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
			overallStatus = HealthStatusDegraded
		}
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		systemHealth := c.checkSystem(ctx)
		mu.Lock()
		checks["system"] = systemHealth
		if systemHealth.Status == HealthStatusDown {
			overallStatus = HealthStatusDown
		} else if systemHealth.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
			overallStatus = HealthStatusDegraded
		}
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		spotlightHealth := c.checkSpotlight(ctx)
		mu.Lock()
		checks["spotlight"] = spotlightHealth
		if spotlightHealth.Status == HealthStatusDown {
			overallStatus = HealthStatusDown
		} else if spotlightHealth.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
			overallStatus = HealthStatusDegraded
		}
		mu.Unlock()
	}()

	wg.Wait()

	return &HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   "1.0.0",
		Uptime:    time.Since(startTime).String(),
		Checks:    checks,
	}
}

func (c *HealthController) checkDatabase(ctx context.Context) ComponentHealth {
	start := time.Now()

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	db := c.app.DB()
	if db == nil {
		return ComponentHealth{
			Status:       HealthStatusDown,
			ResponseTime: time.Since(start).String(),
			Error:        "Database connection pool not available",
		}
	}

	var result int
	err := db.QueryRow(timeoutCtx, "SELECT 1").Scan(&result)
	responseTime := time.Since(start)

	if err != nil {
		return ComponentHealth{
			Status:       HealthStatusDown,
			ResponseTime: responseTime.String(),
			Error:        fmt.Sprintf("Database query failed: %v", err),
		}
	}

	stat := db.Stat()
	details := DatabaseDetails{
		ActiveConnections: int(stat.AcquiredConns()),
		IdleConnections:   int(stat.IdleConns()),
		MaxConnections:    int(stat.MaxConns()),
	}

	status := HealthStatusHealthy
	if responseTime > 100*time.Millisecond {
		status = HealthStatusDegraded
	}

	return ComponentHealth{
		Status:       status,
		ResponseTime: responseTime.String(),
		Details:      map[string]any{"connections": details},
	}
}

func (c *HealthController) checkSystem(ctx context.Context) ComponentHealth {
	_ = ctx
	start := time.Now()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	details := SystemDetails{
		MemoryUsageMB:   m.Alloc / 1024 / 1024,
		CPUUsagePercent: 0,
		Goroutines:      runtime.NumGoroutine(),
	}

	status := HealthStatusHealthy
	if details.MemoryUsageMB > 1000 {
		status = HealthStatusDegraded
	}
	if details.Goroutines > 1000 {
		status = HealthStatusDegraded
	}

	return ComponentHealth{
		Status:       status,
		ResponseTime: time.Since(start).String(),
		Details:      map[string]any{"metrics": details},
	}
}

func (c *HealthController) checkSpotlight(ctx context.Context) ComponentHealth {
	start := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := c.app.Spotlight().Readiness(timeoutCtx); err != nil {
		return ComponentHealth{
			Status:       HealthStatusDown,
			ResponseTime: time.Since(start).String(),
			Error:        err.Error(),
		}
	}

	return ComponentHealth{
		Status:       HealthStatusHealthy,
		ResponseTime: time.Since(start).String(),
	}
}
