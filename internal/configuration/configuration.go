package configuration

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/iota-agency/iota-erp/sdk/configuration"
	"github.com/iota-agency/iota-erp/sdk/utils/env"
)

var singleton *Configuration

type Configuration struct {
	DBOpts             string
	DBName             string
	DBPassword         string
	GoogleRedirectURL  string
	GoogleClientID     string
	GoogleClientSecret string
	ServerPort         int
	SessionDuration    time.Duration
	GoAppEnvironment   string
	SocketAddress      string
	OpenAIKey          string
	UploadsPath        string
	FrontendDomain     string
	PageSize           int
	MaxPageSize        int
	SidCookieKey       string // Session ID cookie key
}

func Use() *Configuration {
	if singleton == nil {
		singleton = &Configuration{} //nolint:exhaustruct
		if err := singleton.load([]string{".env", ".env.local"}); err != nil {
			panic(err)
		}
	}
	return singleton
}

func (c *Configuration) load(envFiles []string) error {
	n, err := configuration.LoadEnv(envFiles)
	if err != nil {
		return err
	}
	if n == 0 {
		log.Printf("No .env files found. Tried: %s\n", strings.Join(envFiles, ", "))
	}

	c.DBOpts = fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		env.GetEnv("DB_HOST", "localhost"),
		env.GetEnv("DB_PORT", "5432"),
		env.GetEnv("DB_USER", "postgres"),
		env.GetEnv("DB_NAME", "iota_erp"),
		env.GetEnv("DB_PASSWORD", "postgres"),
	)

	c.GoogleRedirectURL = env.MustGetEnv("GOOGLE_REDIRECT_URL")
	c.GoogleClientID = env.MustGetEnv("GOOGLE_CLIENT_ID")
	c.GoogleClientSecret = env.MustGetEnv("GOOGLE_CLIENT_SECRET")
	c.ServerPort = env.MustGetEnvInt("PORT", 3200)
	c.PageSize = env.MustGetEnvInt("PAGE_SIZE", 25)
	c.MaxPageSize = env.MustGetEnvInt("MAX_PAGE_SIZE", 100)
	duration, err := configuration.ParseDuration(env.GetEnv("SESSION_DURATION", "1h"))
	if err != nil {
		return err
	}
	c.SessionDuration = duration
	c.GoAppEnvironment = env.GetEnv("GO_APP_ENV", "development")
	if c.GoAppEnvironment == "production" {
		c.SocketAddress = fmt.Sprintf(":%d", c.ServerPort)
	} else {
		c.SocketAddress = fmt.Sprintf("localhost:%d", c.ServerPort)
	}
	c.OpenAIKey = env.MustGetEnv("OPENAI_KEY")
	c.UploadsPath = env.GetEnv("UPLOADS_PATH", "uploads")
	c.FrontendDomain = env.GetEnv("FRONTEND_DOMAIN", "localhost")
	c.SidCookieKey = env.GetEnv("SID_COOKIE_KEY", "sid")
	return nil
}
