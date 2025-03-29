package configuration

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/logging"

	"github.com/caarlos0/env/v11"
	"github.com/iota-uz/utils/fs"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

const Production = "production"

var singleton = sync.OnceValue(func() *Configuration {
	c := &Configuration{}
	if err := c.load([]string{".env", ".env.local"}); err != nil {
		c.Unload()
		panic(err)
	}
	return c
})

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

type DatabaseOptions struct {
	Opts     string `env:"-"`
	Name     string `env:"DB_NAME" envDefault:"iota_erp"`
	Host     string `env:"DB_HOST" envDefault:"localhost"`
	Port     string `env:"DB_PORT" envDefault:"5432"`
	User     string `env:"DB_USER" envDefault:"postgres"`
	Password string `env:"DB_PASSWORD" envDefault:"postgres"`
}

func (d *DatabaseOptions) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		d.Host, d.Port, d.User, d.Name, d.Password,
	)
}

type GoogleOptions struct {
	RedirectURL  string `env:"GOOGLE_REDIRECT_URL"`
	ClientID     string `env:"GOOGLE_CLIENT_ID"`
	ClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
}

type TwilioOptions struct {
	WebhookURL  string `env:"TWILIO_WEBHOOK_URL"`
	AccountSID  string `env:"TWILIO_ACCOUNT_SID"`
	AuthToken   string `env:"TWILIO_AUTH_TOKEN"`
	PhoneNumber string `env:"TWILIO_PHONE_NUMBER"`
}

type LokiOptions struct {
	URL     string `env:"LOKI_URL"`
	AppName string `env:"LOKI_APP_NAME"`
	LogPath string `env:"LOG_PATH" envDefault:"./logs/app.log"`
}

type Configuration struct {
	Database DatabaseOptions
	Google   GoogleOptions
	Twilio   TwilioOptions
	Loki     LokiOptions

	MigrationsDir    string        `env:"MIGRATIONS_DIR" envDefault:"migrations"`
	ServerPort       int           `env:"PORT" envDefault:"3200"`
	SessionDuration  time.Duration `env:"SESSION_DURATION" envDefault:"720h"`
	GoAppEnvironment string        `env:"GO_APP_ENV" envDefault:"development"`
	SocketAddress    string        `env:"-"`
	OpenAIKey        string        `env:"OPENAI_KEY"`
	UploadsPath      string        `env:"UPLOADS_PATH" envDefault:"static"`
	Domain           string        `env:"DOMAIN" envDefault:"localhost"`
	Origin           string        `env:"ORIGIN" envDefault:"http://localhost:3200"`
	PageSize         int           `env:"PAGE_SIZE" envDefault:"25"`
	MaxPageSize      int           `env:"MAX_PAGE_SIZE" envDefault:"100"`
	LogLevel         string        `env:"LOG_LEVEL" envDefault:"error"`
	// SDK will look for this header in the request, if it's not present, it will generate a random uuidv4
	RequestIDHeader string `env:"REQUEST_ID_HEADER" envDefault:"X-Request-ID"`
	// Session ID cookie key
	SidCookieKey        string `env:"SID_COOKIE_KEY" envDefault:"sid"`
	OauthStateCookieKey string `env:"OAUTH_STATE_COOKIE_KEY" envDefault:"oauthState"`

	TelegramBotToken string `env:"TELEGRAM_BOT_TOKEN"`

	logFile *os.File
	logger  *logrus.Logger
}

func (c *Configuration) Logger() *logrus.Logger {
	return c.logger
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
	case "debug":
		return logrus.DebugLevel
	default:
		return logrus.ErrorLevel
	}
}

func (c *Configuration) Address() string {
	return fmt.Sprintf("%s://%s:%d", c.Scheme(), c.Domain, c.ServerPort)
}

func (c *Configuration) Scheme() string {
	if c.GoAppEnvironment == Production { // assume 'https' on production mode
		return "https"
	}
	return "http"
}

func Use() *Configuration {
	return singleton()
}

func (c *Configuration) load(envFiles []string) error {
	n, err := LoadEnv(envFiles)
	if err != nil {
		return err
	}
	if n == 0 {
		wd, _ := os.Getwd()
		log.Println("No .env files found. Tried:")
		for _, file := range envFiles {
			log.Println(filepath.Join(wd, file))
		}
	}
	if err := env.Parse(c); err != nil {
		return err
	}
	f, logger, err := logging.FileLogger(c.LogrusLogLevel(), c.Loki.LogPath)
	if err != nil {
		return err
	}
	c.logFile = f
	c.logger = logger

	c.Database.Opts = c.Database.ConnectionString()
	if c.GoAppEnvironment == Production {
		c.SocketAddress = fmt.Sprintf(":%d", c.ServerPort)
	} else {
		c.SocketAddress = fmt.Sprintf("localhost:%d", c.ServerPort)
	}
	return nil
}

// unload handles a graceful shutdown.
func (c *Configuration) Unload() {
	if c.logFile != nil {
		c.logFile.Close()
	}
}
