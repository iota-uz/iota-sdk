package configuration

import (
	"fmt"
	"github.com/iota-agency/iota-erp/sdk/configuration"
	"github.com/iota-agency/iota-erp/sdk/utils/env"
	"time"
)

var Singleton *Configuration

type Configuration struct {
	Loaded bool
}

func Use() *Configuration {
	if Singleton == nil {
		Singleton = &Configuration{}
	}
	return Singleton
}

func (c *Configuration) Load() error {
	if c.Loaded {
		return nil
	}
	return configuration.LoadEnv()
}

func (c *Configuration) DbOpts() string {
	return fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		env.GetEnv("DB_HOST", "localhost"), env.GetEnv("DB_PORT", "5432"),
		env.GetEnv("DB_USER", "postgres"),
		env.GetEnv("DB_NAME", "iota_erp"), env.GetEnv("DB_PASSWORD", "postgres"))
}

func (c *Configuration) ServerPort() string {
	return env.GetEnv("PORT", "3200")
}

func (c *Configuration) SessionDuration() (time.Duration, error) {
	sessionDuration := env.GetEnv("SESSION_DURATION", "1h")
	return configuration.ParseDuration(sessionDuration)
}

func (c *Configuration) GoAppEnvironment() string {
	return env.GetEnv("GO_APP_ENV", "development")
}

func (c *Configuration) SocketAddress() string {
	if c.GoAppEnvironment() == "production" {
		return fmt.Sprintf(":%s", c.ServerPort())
	}
	return fmt.Sprintf("localhost:%s", c.ServerPort())
}
