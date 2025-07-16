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
	AppName string `env:"LOKI_APP_NAME" envDefault:"sdk"`
	LogPath string `env:"LOG_PATH" envDefault:"./logs/app.log"`
}

type OpenTelemetryOptions struct {
	Enabled     bool   `env:"OTEL_ENABLED" envDefault:"false"`
	TempoURL    string `env:"OTEL_TEMPO_URL" envDefault:"localhost:4318"`
	ServiceName string `env:"OTEL_SERVICE_NAME" envDefault:"sdk"`
}

type ClickOptions struct {
	URL            string `env:"CLICK_URL" envDefault:"https://my.click.uz"`
	MerchantID     int64  `env:"CLICK_MERCHANT_ID"`
	MerchantUserID int64  `env:"CLICK_MERCHANT_USER_ID"`
	ServiceID      int64  `env:"CLICK_SERVICE_ID"`
	SecretKey      string `env:"CLICK_SECRET_KEY"`
}

type PaymeOptions struct {
	URL        string `env:"PAYME_URL" envDefault:"https://checkout.test.paycom.uz"`
	MerchantID string `env:"PAYME_MERCHANT_ID"`
	User       string `env:"PAYME_USER" envDefault:"Paycom"`
	SecretKey  string `env:"PAYME_SECRET_KEY"`
}

type OctoOptions struct {
	OctoShopID     int32  `env:"OCTO_SHOP_ID"`
	OctoSecret     string `env:"OCTO_SECRET"`
	OctoSecretHash string `env:"OCTO_SECRET_HASH"`
	NotifyUrl      string `env:"OCTO_NOTIFY_URL"`
}

type StripeOptions struct {
	SecretKey     string `env:"STRIPE_SECRET_KEY"`
	SigningSecret string `env:"STRIPE_SIGNING_SECRET"`
}

type RateLimitOptions struct {
	Enabled   bool   `env:"RATE_LIMIT_ENABLED" envDefault:"true"`
	GlobalRPS int    `env:"RATE_LIMIT_GLOBAL_RPS" envDefault:"1000"`
	Storage   string `env:"RATE_LIMIT_STORAGE" envDefault:"memory"` // memory or redis
	RedisURL  string `env:"RATE_LIMIT_REDIS_URL"`
}

// Validate checks the rate limit configuration for errors
func (r *RateLimitOptions) Validate() error {
	if r.GlobalRPS < 0 {
		return fmt.Errorf("rate limit GlobalRPS must be non-negative, got %d", r.GlobalRPS)
	}
	if r.GlobalRPS > 1000000 {
		return fmt.Errorf("rate limit GlobalRPS too high, maximum is 1,000,000, got %d", r.GlobalRPS)
	}
	if r.Storage != "memory" && r.Storage != "redis" {
		return fmt.Errorf("rate limit Storage must be 'memory' or 'redis', got '%s'", r.Storage)
	}
	if r.Storage == "redis" && r.RedisURL == "" {
		return fmt.Errorf("rate limit RedisURL is required when Storage is 'redis'")
	}
	return nil
}

type Configuration struct {
	Database      DatabaseOptions
	Google        GoogleOptions
	Twilio        TwilioOptions
	Loki          LokiOptions
	OpenTelemetry OpenTelemetryOptions
	Click         ClickOptions
	Payme         PaymeOptions
	Octo          OctoOptions
	Stripe        StripeOptions
	RateLimit     RateLimitOptions

	MigrationsDir    string        `env:"MIGRATIONS_DIR" envDefault:"migrations"`
	ServerPort       int           `env:"PORT" envDefault:"3200"`
	SessionDuration  time.Duration `env:"SESSION_DURATION" envDefault:"720h"`
	GoAppEnvironment string        `env:"GO_APP_ENV" envDefault:"development"`
	SocketAddress    string        `env:"-"`
	OpenAIKey        string        `env:"OPENAI_KEY"`
	UploadsPath      string        `env:"UPLOADS_PATH" envDefault:"static"`
	Domain           string        `env:"DOMAIN" envDefault:"localhost:3200"`
	Origin           string        `env:"ORIGIN" envDefault:"http://localhost:3200"`
	PageSize         int           `env:"PAGE_SIZE" envDefault:"25"`
	MaxPageSize      int           `env:"MAX_PAGE_SIZE" envDefault:"100"`
	MaxUploadSize    int64         `env:"MAX_UPLOAD_SIZE" envDefault:"33554432"`
	MaxUploadMemory  int64         `env:"MAX_UPLOAD_MEMORY" envDefault:"33554432"`
	LogLevel         string        `env:"LOG_LEVEL" envDefault:"error"`
	// SDK will look for this header in the request, if it's not present, it will generate a random uuidv4
	RequestIDHeader string `env:"REQUEST_ID_HEADER" envDefault:"X-Request-ID"`
	// SDK will look for this header in the request, if it's not present, it will use request.RemoteAddr
	RealIPHeader string `env:"REAL_IP_HEADER" envDefault:"X-Real-IP"`
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

	// Validate rate limiting configuration
	if err := c.RateLimit.Validate(); err != nil {
		return fmt.Errorf("rate limit configuration error: %w", err)
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
		if err := c.logFile.Close(); err != nil {
			log.Printf("Failed to close log file: %v", err)
		}
	}
}
