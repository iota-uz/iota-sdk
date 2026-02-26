package health

import "time"

type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
	StatusDisabled Status = "disabled"
	StatusUnknown  Status = "unknown"
)

type Capability struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Status  Status `json:"status"`
	Source  string `json:"source"`
	Message string `json:"message,omitempty"`
}

type HealthCheck struct {
	Status    Status         `json:"status"`
	LatencyMs int64          `json:"latencyMs,omitempty"`
	Message   string         `json:"message,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type DetailedHealth struct {
	Status       Status                 `json:"status"`
	Timestamp    time.Time              `json:"timestamp"`
	Checks       map[string]HealthCheck `json:"checks"`
	Capabilities []Capability           `json:"capabilities"`
}
