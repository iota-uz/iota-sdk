package periodics

import "time"

// BaseTaskConfig holds common configuration for a periodic task with env tags.
// Embed this struct in your task-specific config and add an envPrefix tag.
//
// Example:
//
//	type MyTaskConfig struct {
//	    periodics.BaseTaskConfig `envPrefix:"PERIODIC_MY_TASK_"`
//	    CustomField string       `env:"CUSTOM_FIELD" envDefault:"value"`
//	}
type BaseTaskConfig struct {
	// Schedule is the cron expression for when to run the task.
	// No envDefault — each module sets its own default via NewDefaultConfig().
	Schedule string `env:"SCHEDULE"`

	// MaxRetries is the maximum number of retry attempts on failure
	MaxRetries int `env:"MAX_RETRIES" envDefault:"3"`

	// RetryDelay is the initial delay between retries (will be exponentially increased)
	RetryDelay time.Duration `env:"RETRY_DELAY" envDefault:"1s"`

	// Timeout is the maximum time the task can run before being cancelled
	Timeout time.Duration `env:"TIMEOUT" envDefault:"5m"`

	// EnableSkipIfRunning skips execution if previous instance is still running
	EnableSkipIfRunning bool `env:"ENABLE_SKIP_IF_RUNNING" envDefault:"true"`

	// Enabled determines if the task should be registered and run
	Enabled bool `env:"ENABLED" envDefault:"true"`

	// RunOnStart determines if the task should run immediately on application startup
	RunOnStart bool `env:"RUN_ON_START" envDefault:"false"`
}

// ToTaskConfig converts BaseTaskConfig to TaskConfig
func (c BaseTaskConfig) ToTaskConfig() TaskConfig {
	skip := c.EnableSkipIfRunning
	return TaskConfig{
		MaxRetries:          IntPtr(c.MaxRetries),
		RetryDelay:          c.RetryDelay,
		Timeout:             c.Timeout,
		EnableSkipIfRunning: &skip,
	}
}
