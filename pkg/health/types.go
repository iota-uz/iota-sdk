package health

import "time"

// Status represents a health evaluation state.
type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
	StatusDisabled Status = "disabled"
	StatusUnknown  Status = "unknown"
)

// Capability describes an optional feature and its runtime health state.
type Capability struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Status  Status `json:"status"`
	Source  string `json:"source"`
	Message string `json:"message,omitempty"`
}

// HealthCheckDetails intentionally remains open-ended because different checks
// emit different diagnostic payloads.
type HealthCheckDetails map[string]any

// HealthCheck captures the outcome of a single detailed check.
type HealthCheck struct {
	Status    Status             `json:"status"`
	LatencyMs int64              `json:"latencyMs,omitempty"`
	Message   string             `json:"message,omitempty"`
	Details   HealthCheckDetails `json:"details,omitempty"`
}

// DetailedHealth is the full internal diagnostics payload.
type DetailedHealth struct {
	Status       Status                 `json:"status"`
	Timestamp    time.Time              `json:"timestamp"`
	Checks       map[string]HealthCheck `json:"checks"`
	Capabilities []Capability           `json:"capabilities"`
}
