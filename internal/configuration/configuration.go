package configuration

import (
	"fmt"
	"github.com/iota-agency/iota-erp/sdk/configuration"
	"github.com/iota-agency/iota-erp/sdk/utils/env"
	"time"
)

var singleton *Configuration

type Configuration struct {
	DbOpts             string
	DbHost             string
	DbPort             int
	DbUser             string
	DbName             string
	DbPassword         string
	GoogleRedirectURL  string
	GoogleClientID     string
	GoogleClientSecret string
	ServerPort         string
	SessionDuration    time.Duration
	GoAppEnvironment   string
	SocketAddress      string
	OpenAIKey          string
	UploadsPath        string
	FrontendDomain     string
}

func Use() *Configuration {
	if singleton == nil {
		singleton = &Configuration{}
		if err := singleton.Load(); err != nil {
			panic(err)
		}
	}
	return singleton
}

func (c *Configuration) Load() error {
	if err := configuration.LoadEnv(); err != nil {
		return err
	}

	c.DbOpts = fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		env.GetEnv("DB_HOST", "localhost"), env.GetEnv("DB_PORT", "5432"),
		env.GetEnv("DB_USER", "postgres"),
		env.GetEnv("DB_NAME", "iota_erp"), env.GetEnv("DB_PASSWORD", "postgres"))

	c.GoogleRedirectURL = env.MustGetEnv("GOOGLE_REDIRECT_URL")
	c.GoogleClientID = env.MustGetEnv("GOOGLE_CLIENT_ID")
	c.GoogleClientSecret = env.MustGetEnv("GOOGLE_CLIENT_SECRET")
	c.ServerPort = env.GetEnv("PORT", "3200")
	duration, err := configuration.ParseDuration(env.GetEnv("SESSION_DURATION", "1h"))
	if err != nil {
		return err
	}
	c.SessionDuration = duration
	c.GoAppEnvironment = env.GetEnv("GO_APP_ENV", "development")
	if c.GoAppEnvironment == "production" {
		c.SocketAddress = fmt.Sprintf(":%s", c.ServerPort)
	} else {
		c.SocketAddress = fmt.Sprintf("localhost:%s", c.ServerPort)
	}
	c.OpenAIKey = env.MustGetEnv("OPENAI_KEY")
	c.UploadsPath = env.GetEnv("UPLOADS_PATH", "uploads")
	c.FrontendDomain = env.GetEnv("FRONTEND_DOMAIN", "localhost")
	return nil
}
