package devhub

// Config represents the new simplified YAML structure
type Config struct {
	Services map[string]ServiceConfig
}

// ServiceConfig represents a service in the new format
type ServiceConfig struct {
	Name   string            `yaml:"-"` // Set from map key
	Desc   string            `yaml:"desc"`
	Port   int               `yaml:"port,omitempty"`
	Run    string            `yaml:"run"`
	Needs  []string          `yaml:"needs,omitempty"`
	Health *HealthConfig     `yaml:"health,omitempty"`
	OS     map[string]string `yaml:"os,omitempty"`
}

// HealthConfig supports multiple health check formats
type HealthConfig struct {
	// Simple TCP port check
	TCP int `yaml:"tcp,omitempty"`

	// HTTP endpoint check
	HTTP string `yaml:"http,omitempty"`

	// Command check
	Cmd string `yaml:"cmd,omitempty"`

	// Common settings
	Wait     string `yaml:"wait,omitempty"`     // Startup grace period
	Interval string `yaml:"interval,omitempty"` // Check interval
	Timeout  string `yaml:"timeout,omitempty"`  // Check timeout
	Retries  int    `yaml:"retries,omitempty"`  // Number of retries
}
