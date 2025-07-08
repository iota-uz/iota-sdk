package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

// HealthStatus represents the health state of a service
type HealthStatus int

const (
	HealthUnknown HealthStatus = iota
	HealthStarting
	HealthHealthy
	HealthUnhealthy
)

func (h HealthStatus) String() string {
	switch h {
	case HealthUnknown:
		return "Unknown"
	case HealthStarting:
		return "Starting"
	case HealthHealthy:
		return "Healthy"
	case HealthUnhealthy:
		return "Unhealthy"
	default:
		return "Unknown"
	}
}

// HealthChecker interface for different health check implementations
type HealthChecker interface {
	Check(ctx context.Context) error
}

// TCPHealthCheck checks if a TCP port is accepting connections
type TCPHealthCheck struct {
	Host string
	Port string
}

func (t *TCPHealthCheck) Check(ctx context.Context) error {
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(t.Host, t.Port))
	if err != nil {
		return fmt.Errorf("tcp health check failed: %w", err)
	}
	_ = conn.Close()
	return nil
}

// HTTPHealthCheck performs an HTTP request to check service health
type HTTPHealthCheck struct {
	URL            string
	Method         string
	ExpectedStatus int
	Headers        map[string]string
	Timeout        time.Duration
}

func (h *HTTPHealthCheck) Check(ctx context.Context) error {
	method := h.Method
	if method == "" {
		method = "GET"
	}

	expectedStatus := h.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	timeout := h.Timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}

	req, err := http.NewRequestWithContext(ctx, method, h.URL, nil)
	if err != nil {
		return fmt.Errorf("http health check failed to create request: %w", err)
	}

	for key, value := range h.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http health check request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("http health check failed: expected status %d, got %d", expectedStatus, resp.StatusCode)
	}

	return nil
}

// CommandHealthCheck executes a command to check service health
type CommandHealthCheck struct {
	Command []string
	Timeout time.Duration
}

func (c *CommandHealthCheck) Check(ctx context.Context) error {
	if len(c.Command) == 0 {
		return fmt.Errorf("command health check: no command specified")
	}

	timeout := c.Timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.Command[0], c.Command[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command health check failed: %w", err)
	}

	return nil
}

// HealthMonitor manages health checks for a service
type HealthMonitor struct {
	checker      HealthChecker
	interval     time.Duration
	timeout      time.Duration
	retries      int
	startPeriod  time.Duration
	status       HealthStatus
	lastCheck    time.Time
	failureCount int
	startTime    time.Time
	mu           sync.RWMutex
}

func NewHealthMonitor(checker HealthChecker, interval, timeout time.Duration, retries int, startPeriod time.Duration) *HealthMonitor {
	return &HealthMonitor{
		checker:     checker,
		interval:    interval,
		timeout:     timeout,
		retries:     retries,
		startPeriod: startPeriod,
		status:      HealthUnknown,
		startTime:   time.Now(),
	}
}

func (h *HealthMonitor) GetStatus() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

func (h *HealthMonitor) Start(ctx context.Context) {
	// Set initial status to starting
	h.mu.Lock()
	h.status = HealthStarting
	h.mu.Unlock()

	// Wait for start period before beginning health checks
	select {
	case <-time.After(h.startPeriod):
	case <-ctx.Done():
		return
	}

	// Start health check loop
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Do initial check
	h.performCheck(ctx)

	for {
		select {
		case <-ticker.C:
			h.performCheck(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (h *HealthMonitor) performCheck(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	err := h.checker.Check(checkCtx)

	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastCheck = time.Now()

	if err == nil {
		// Check passed
		h.failureCount = 0
		h.status = HealthHealthy
	} else {
		// Check failed
		h.failureCount++
		if h.failureCount >= h.retries {
			h.status = HealthUnhealthy
		}
	}
}

// ParseHealthCheck creates a HealthChecker from configuration
func ParseHealthCheck(config map[string]interface{}) (HealthChecker, error) {
	checkType, ok := config["type"].(string)
	if !ok {
		return nil, fmt.Errorf("health check type not specified")
	}

	switch checkType {
	case "tcp":
		port, ok := config["port"].(string)
		if !ok {
			return nil, fmt.Errorf("tcp health check requires port")
		}
		host := "localhost"
		if h, ok := config["host"].(string); ok {
			host = h
		}
		return &TCPHealthCheck{Host: host, Port: port}, nil

	case "http":
		url, ok := config["url"].(string)
		if !ok {
			return nil, fmt.Errorf("http health check requires url")
		}
		check := &HTTPHealthCheck{URL: url}

		if method, ok := config["method"].(string); ok {
			check.Method = method
		}
		if status, ok := config["expected_status"].(int); ok {
			check.ExpectedStatus = status
		}
		if headers, ok := config["headers"].(map[string]interface{}); ok {
			check.Headers = make(map[string]string)
			for k, v := range headers {
				if str, ok := v.(string); ok {
					check.Headers[k] = str
				}
			}
		}
		return check, nil

	case "command":
		cmd, ok := config["test_command"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("command health check requires test_command")
		}

		var command []string
		for _, c := range cmd {
			if str, ok := c.(string); ok {
				command = append(command, str)
			}
		}
		if len(command) == 0 {
			return nil, fmt.Errorf("command health check requires non-empty test_command")
		}

		return &CommandHealthCheck{Command: command}, nil

	default:
		return nil, fmt.Errorf("unknown health check type: %s", checkType)
	}
}
