package configuration

import (
	"fmt"
	"github.com/iota-agency/iota-erp/sdk/configuration"
	"github.com/iota-agency/iota-erp/sdk/utils/env"
	"time"
)

var singleton *Configuration

type Configuration struct {
	Loaded bool
}

func Use() *Configuration {
	if singleton == nil {
		singleton = &Configuration{}
	}
	return singleton
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

func (c *Configuration) GoogleRedirectURL() string {
	return env.GetEnv("GOOGLE_REDIRECT_URL", "http://localhost:3200/oauth/google/callback")
}

func (c *Configuration) GoogleClientID() string {
	return env.GetEnv("GOOGLE_CLIENT_ID", "")
}

func (c *Configuration) GoogleClientSecret() string {
	return env.GetEnv("GOOGLE_CLIENT_SECRET", "")
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

func (c *Configuration) OpenAIKey() string {
	return env.MustGetEnv("OPENAI_KEY")
}

func (c *Configuration) UploadsPath() string {
	return env.GetEnv("UPLOADS_PATH", "uploads")
}
