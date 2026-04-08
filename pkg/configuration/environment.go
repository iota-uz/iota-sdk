// Package configuration provides this package.
package configuration

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/logging"

	"github.com/caarlos0/env/v11"
	"github.com/iota-uz/utils/fs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

	MaxConns              int32         `env:"DB_MAX_CONNS" envDefault:"32"`
	MinConns              int32         `env:"DB_MIN_CONNS" envDefault:"8"`
	MaxConnLifetime       time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"1h"`
	MaxConnLifetimeJitter time.Duration `env:"DB_MAX_CONN_LIFETIME_JITTER" envDefault:"6m"`
	MaxConnIdleTime       time.Duration `env:"DB_MAX_CONN_IDLE_TIME" envDefault:"15m"`
	HealthCheckPeriod     time.Duration `env:"DB_HEALTH_CHECK_PERIOD" envDefault:"1m"`
	ConnectTimeout        time.Duration `env:"DB_CONNECT_TIMEOUT" envDefault:"10s"`
}

func (d *DatabaseOptions) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		d.Host, d.Port, d.User, d.Name, d.Password,
	)
}

// PoolConfig returns a fully configured pgxpool.Config derived from the
// connection string and the pool-tuning fields on DatabaseOptions.
func (d *DatabaseOptions) PoolConfig() (*pgxpool.Config, error) {
	if d.MaxConns <= 0 {
		return nil, fmt.Errorf("DB_MAX_CONNS must be positive, got %d", d.MaxConns)
	}
	if d.MinConns > d.MaxConns {
		return nil, fmt.Errorf("DB_MIN_CONNS (%d) must not exceed DB_MAX_CONNS (%d)", d.MinConns, d.MaxConns)
	}
	if d.ConnectTimeout <= 0 {
		return nil, fmt.Errorf("DB_CONNECT_TIMEOUT must be positive, got %s", d.ConnectTimeout)
	}

	cfg, err := pgxpool.ParseConfig(d.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("DatabaseOptions.PoolConfig: parse connection string: %w", err)
	}

	cfg.MaxConns = d.MaxConns
	cfg.MinConns = d.MinConns
	cfg.MaxConnLifetime = d.MaxConnLifetime
	cfg.MaxConnLifetimeJitter = d.MaxConnLifetimeJitter
	cfg.MaxConnIdleTime = d.MaxConnIdleTime
	cfg.HealthCheckPeriod = d.HealthCheckPeriod
	cfg.ConnConfig.ConnectTimeout = d.ConnectTimeout

	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET idle_in_transaction_session_timeout = '120s'")
		return err
	}

	cfg.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
		return conn.Ping(ctx) == nil
	}

	return cfg, nil
}

type GoogleOptions struct {
	RedirectURL  string `env:"GOOGLE_REDIRECT_URL"`
	ClientID     string `env:"GOOGLE_CLIENT_ID"`
	ClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
}

// IsConfigured returns true if Google OAuth is configured (both ClientID and ClientSecret are set).
// This makes Google OAuth enablement implicit - no explicit flag needed.
func (g *GoogleOptions) IsConfigured() bool {
	return g.ClientID != "" && g.ClientSecret != ""
}

type TwilioOptions struct {
	WebhookURL  string `env:"TWILIO_WEBHOOK_URL"`
	AccountSID  string `env:"TWILIO_ACCOUNT_SID"`
	AuthToken   string `env:"TWILIO_AUTH_TOKEN"`
	PhoneNumber string `env:"TWILIO_PHONE_NUMBER"`
}

type SMTPOptions struct {
	Host     string `env:"SMTP_HOST"`
	Port     int    `env:"SMTP_PORT" envDefault:"587"`
	Username string `env:"SMTP_USERNAME"`
	Password string `env:"SMTP_PASSWORD"`
	From     string `env:"SMTP_FROM"` // Sender email address
}

type OTPDeliveryOptions struct {
	EnableEmail bool `env:"OTP_ENABLE_EMAIL" envDefault:"false"`
	EnableSMS   bool `env:"OTP_ENABLE_SMS" envDefault:"false"`
}

type LokiOptions struct {
	URL     string `env:"LOKI_URL"`
	AppName string `env:"LOKI_APP_NAME" envDefault:"sdk"`
	LogPath string `env:"LOG_PATH" envDefault:"./logs/app.log"`
}

type OpenTelemetryOptions struct {
	TempoURL    string `env:"OTEL_TEMPO_URL"`
	ServiceName string `env:"OTEL_SERVICE_NAME"`
}

// IsConfigured returns true when both TempoURL and ServiceName are set.
// This makes OpenTelemetry enablement implicit — no explicit flag needed.
func (o *OpenTelemetryOptions) IsConfigured() bool {
	return o.TempoURL != "" && o.ServiceName != ""
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
	NotifyURL      string `env:"OCTO_NOTIFY_URL"`
}

type StripeOptions struct {
	SecretKey     string `env:"STRIPE_SECRET_KEY"`
	SigningSecret string `env:"STRIPE_SIGNING_SECRET"`
}

type OIDCOptions struct {
	IssuerURL            string        `env:"OIDC_ISSUER_URL"`
	CryptoKey            string        `env:"OIDC_CRYPTO_KEY"` // 32-byte base64
	AccessTokenLifetime  time.Duration `env:"OIDC_ACCESS_TOKEN_LIFETIME" envDefault:"1h"`
	RefreshTokenLifetime time.Duration `env:"OIDC_REFRESH_TOKEN_LIFETIME" envDefault:"720h"`
	IDTokenLifetime      time.Duration `env:"OIDC_ID_TOKEN_LIFETIME" envDefault:"1h"`
}

// IsConfigured returns true when required OIDC settings are present.
// OIDC activation is implicit to keep configuration consistent with other modules (e.g. Google OAuth).
func (o *OIDCOptions) IsConfigured() bool {
	return o.IssuerURL != "" && o.CryptoKey != ""
}

type RateLimitOptions struct {
	Enabled   bool   `env:"RATE_LIMIT_ENABLED" envDefault:"true"`
	GlobalRPS int    `env:"RATE_LIMIT_GLOBAL_RPS" envDefault:"1000"`
	Storage   string `env:"RATE_LIMIT_STORAGE" envDefault:"memory"` // memory or redis
	RedisURL  string `env:"RATE_LIMIT_REDIS_URL"`
}

// TwoFactorAuthOptions contains configuration for two-factor authentication and OTP
type TwoFactorAuthOptions struct {
	// Two-Factor Authentication
	Enabled    bool   `env:"ENABLE_2FA" envDefault:"false"`
	TOTPIssuer string `env:"TOTP_ISSUER"` // No default - must be set by app if 2FA is enabled

	// OTP Settings
	OTPCodeLength  int `env:"OTP_CODE_LENGTH" envDefault:"6"`
	OTPTTLSeconds  int `env:"OTP_TTL_SECONDS" envDefault:"300"`
	OTPMaxAttempts int `env:"OTP_MAX_ATTEMPTS" envDefault:"3"`

	// TOTP Secret Encryption
	EncryptionKey string `env:"TOTP_ENCRYPTION_KEY"` // Required for production - used to encrypt TOTP secrets at rest
}

// Validate checks the two-factor auth configuration for errors
func (t *TwoFactorAuthOptions) Validate() error {
	if !t.Enabled {
		return nil // Skip validation if 2FA is disabled
	}

	// TOTPIssuer is required when 2FA is enabled
	if t.TOTPIssuer == "" {
		return fmt.Errorf("TOTP_ISSUER is required when ENABLE_2FA is true")
	}

	// Validate OTP code length (4-10 digits)
	if t.OTPCodeLength < 4 || t.OTPCodeLength > 10 {
		return fmt.Errorf("OTPCodeLength must be between 4 and 10, got %d", t.OTPCodeLength)
	}

	// Validate OTP TTL (60-900 seconds = 1-15 minutes)
	if t.OTPTTLSeconds < 60 || t.OTPTTLSeconds > 900 {
		return fmt.Errorf("OTPTTLSeconds must be between 60 and 900, got %d", t.OTPTTLSeconds)
	}

	// Validate OTP max attempts (1-10)
	if t.OTPMaxAttempts < 1 || t.OTPMaxAttempts > 10 {
		return fmt.Errorf("OTPMaxAttempts must be between 1 and 10, got %d", t.OTPMaxAttempts)
	}

	return nil
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
	SMTP          SMTPOptions
	OTPDelivery   OTPDeliveryOptions
	Loki          LokiOptions
	OpenTelemetry OpenTelemetryOptions
	Click         ClickOptions
	Payme         PaymeOptions
	Octo          OctoOptions
	Stripe        StripeOptions
	OIDC          OIDCOptions
	RateLimit     RateLimitOptions
	TwoFactorAuth TwoFactorAuthOptions

	RedisURL                string        `env:"REDIS_URL" envDefault:"localhost:6379"`
	MeiliURL                string        `env:"MEILI_URL"`
	MeiliAPIKey             string        `env:"MEILI_API_KEY"`
	MigrationsDir           string        `env:"MIGRATIONS_DIR" envDefault:"migrations"`
	ServerPort              int           `env:"PORT" envDefault:"3200"`
	SessionDuration         time.Duration `env:"SESSION_DURATION" envDefault:"720h"`
	GoAppEnvironment        string        `env:"GO_APP_ENV" envDefault:"development"`
	SocketAddress           string        `env:"-"`
	OpenAIKey               string        `env:"OPENAI_KEY"`
	UploadsPath             string        `env:"UPLOADS_PATH" envDefault:"static"`
	Domain                  string        `env:"DOMAIN" envDefault:"localhost"`
	Origin                  string        `env:"ORIGIN" envDefault:"http://localhost:3200"`
	BiChatKnowledgeDir      string        `env:"BICHAT_KNOWLEDGE_DIR"`
	BiChatKBIndexPath       string        `env:"BICHAT_KB_INDEX_PATH"`
	BiChatSchemaMetadataDir string        `env:"BICHAT_SCHEMA_METADATA_DIR"`
	BiChatKnowledgeAutoLoad bool          `env:"BICHAT_KNOWLEDGE_AUTO_LOAD" envDefault:"false"`
	PageSize                int           `env:"PAGE_SIZE" envDefault:"25"`
	MaxPageSize             int           `env:"MAX_PAGE_SIZE" envDefault:"100"`
	MaxUploadSize           int64         `env:"MAX_UPLOAD_SIZE" envDefault:"33554432"`
	MaxUploadMemory         int64         `env:"MAX_UPLOAD_MEMORY" envDefault:"33554432"`
	LogLevel                string        `env:"LOG_LEVEL" envDefault:"error"`
	// SDK will look for this header in the request, if it's not present, it will generate a random uuidv4
	RequestIDHeader string `env:"REQUEST_ID_HEADER" envDefault:"X-Request-ID"`
	// SDK will look for this header in the request, if it's not present, it will use request.RemoteAddr
	RealIPHeader string `env:"REAL_IP_HEADER" envDefault:"X-Real-IP"`
	// Session ID cookie key
	SidCookieKey        string `env:"SID_COOKIE_KEY" envDefault:"sid"`
	OauthStateCookieKey string `env:"OAUTH_STATE_COOKIE_KEY" envDefault:"oauthState"`
	// Allowed origins for CORS and CSRF (full URLs, e.g. "http://localhost:3000").
	// Used by CORS middleware as-is, and by CSRF middleware after normalizing scheme-qualified origins.
	// Origin from config is always trusted for CSRF in addition to this list.
	AllowedOrigins []string `env:"ALLOWED_ORIGINS" envDefault:"http://localhost:3000"`

	TelegramBotToken string `env:"TELEGRAM_BOT_TOKEN"`

	// Test endpoints - only enable in test environment
	EnableTestEndpoints bool `env:"ENABLE_TEST_ENDPOINTS" envDefault:"false"`

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

// IsDev returns true when GoAppEnvironment is not "production".
// GO_APP_ENV defaults to "development" when unset.
func (c *Configuration) IsDev() bool {
	return c.GoAppEnvironment != Production
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

	// Validate two-factor auth configuration
	if err := c.TwoFactorAuth.Validate(); err != nil {
		return fmt.Errorf("two-factor auth configuration error: %w", err)
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

	// Update Domain and Origin dynamically if they weren't explicitly set via environment variables
	// This ensures logs show the correct port when PORT is set via environment
	if os.Getenv("DOMAIN") == "" {
		c.Domain = "localhost"
	}
	if os.Getenv("ORIGIN") == "" {
		// Only include port in Origin for development environment
		// Production and staging should use standard ports (80/443)
		if c.GoAppEnvironment == "development" {
			c.Origin = fmt.Sprintf("%s://%s:%d", c.Scheme(), c.Domain, c.ServerPort)
		} else {
			c.Origin = fmt.Sprintf("%s://%s", c.Scheme(), c.Domain)
		}
	}

	return nil
}

// Unload releases all environment resources.
func (c *Configuration) Unload() {
	if c.logFile != nil {
		if err := c.logFile.Close(); err != nil {
			log.Printf("Failed to close log file: %v", err)
		}
	}
}
