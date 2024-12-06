package configuration

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
	"log"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/iota-agency/iota-sdk/pkg/utils/fs"
	"github.com/joho/godotenv"
)

var singleton *Configuration

func LoadEnv(envFiles []string) (int, error) {
	exists := make([]bool, len(envFiles))
	for i, file := range envFiles {
		if fs.FileExists(file) {
			exists[i] = true
		}
	}

	existingFiles := make([]string, 0, len(envFiles))
	for i, file := range envFiles {
		if exists[i] {
			existingFiles = append(existingFiles, file)
		}
	}

	if len(existingFiles) == 0 {
		return 0, nil
	}

	return len(existingFiles), godotenv.Load(existingFiles...)
}

type Configuration struct {
	DBOpts             string        `env:"-"`
	DBName             string        `env:"DB_NAME" envDefault:"iota_erp"`
	DBHost             string        `env:"DB_HOST" envDefault:"localhost"`
	DBPort             string        `env:"DB_PORT" envDefault:"5432"`
	DBUser             string        `env:"DB_USER" envDefault:"postgres"`
	DBPassword         string        `env:"DB_PASSWORD" envDefault:"postgres"`
	GoogleRedirectURL  string        `env:"GOOGLE_REDIRECT_URL"`
	GoogleClientID     string        `env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string        `env:"GOOGLE_CLIENT_SECRET"`
	ServerPort         int           `env:"PORT" envDefault:"3200"`
	SessionDuration    time.Duration `env:"SESSION_DURATION" envDefault:"720h"`
	GoAppEnvironment   string        `env:"GO_APP_ENV" envDefault:"development"`
	SocketAddress      string        `env:"-"`
	OpenAIKey          string        `env:"OPENAI_KEY"`
	UploadsPath        string        `env:"UPLOADS_PATH" envDefault:"static"`
	Domain             string        `env:"DOMAIN" envDefault:"localhost"`
	Origin             string        `env:"ORIGIN" envDefault:"http://localhost:3200"`
	PageSize           int           `env:"PAGE_SIZE" envDefault:"25"`
	MaxPageSize        int           `env:"MAX_PAGE_SIZE" envDefault:"100"`
	LogLevel           string        `env:"LOG_LEVEL" envDefault:"error"`
	// Session ID cookie key
	SidCookieKey string `env:"SID_COOKIE_KEY" envDefault:"sid"`
}

func (c *Configuration) LogrusLogLevel() logrus.Level {
	switch c.LogLevel {
	case "silent":
		return logrus.PanicLevel
	case "error":
		return logrus.ErrorLevel
	case "warn":
		return logrus.WarnLevel
	case "info":
		return logrus.InfoLevel
	default:
		return logrus.ErrorLevel
	}
}

func (c *Configuration) GormLogLevel() logger.LogLevel {
	switch c.LogLevel {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Error
	}
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
	n, err := LoadEnv(envFiles)
	if err != nil {
		return err
	}
	if n == 0 {
		log.Printf("No .env files found. Tried: %s\n", strings.Join(envFiles, ", "))
	}
	if err := env.Parse(c); err != nil {
		return err
	}
	c.DBOpts = fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBName, c.DBPassword,
	)
	if c.GoAppEnvironment == "production" {
		c.SocketAddress = fmt.Sprintf(":%d", c.ServerPort)
	} else {
		c.SocketAddress = fmt.Sprintf("localhost:%d", c.ServerPort)
	}
	return nil
}
