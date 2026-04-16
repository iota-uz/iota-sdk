// Package env provides a config.Provider that loads .env files and overlays
// process environment variables.
//
// Key transform (locked — do not change without Wave approval):
// Single underscore → dot, lowercased. Leading/trailing underscores stripped.
//
//	BICHAT_OPENAI_API_KEY → bichat.openai.api_key
//	_LEADING → leading
//	TRAILING_ → trailing
package env

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
	koanfenv "github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

// New returns a Provider that:
//  1. Loads each of the given .env files in order (missing files are silently ignored;
//     malformed files return an error from Load).
//  2. Overlays the live process environment (os.Environ) on top.
//
// Key transform: single underscore → dot, lowercased. See package doc.
func New(files ...string) config.Provider {
	return &envProvider{files: files}
}

type envProvider struct {
	files []string
}

func (p *envProvider) Load(k *koanf.Koanf) error {
	// Collect vars from .env files first (earlier files have lower precedence
	// than later ones, and all file vars have lower precedence than process env).
	fileVars := map[string]string{}
	for _, f := range p.files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			// Silently skip missing files.
			continue
		}
		m, err := godotenv.Read(f)
		if err != nil {
			return err
		}
		for k, v := range m {
			fileVars[k] = v
		}
	}

	// Build a merged environ: file vars first, then process env overrides.
	// We supply this via EnvironFunc so the koanf env provider uses our merged set.
	processEnv := environToMap(os.Environ())
	merged := make(map[string]string, len(fileVars)+len(processEnv))
	for k, v := range fileVars {
		merged[k] = v
	}
	for k, v := range processEnv {
		merged[k] = v
	}

	environ := mapToEnviron(merged)

	provider := koanfenv.Provider(".", koanfenv.Opt{
		TransformFunc: transformKey,
		EnvironFunc:   func() []string { return environ },
	})

	return k.Load(provider, nil)
}

// legacyAliases maps legacy env var names to their stdconfig koanf paths so
// existing deployments and CI workflows continue to work without renames.
// New code should set the canonical env name (e.g. HTTP_PORT for http.port);
// legacy names listed here are honored as a transition convenience.
var legacyAliases = map[string]string{
	// httpconfig
	"PORT":                   "http.port",
	"DOMAIN":                 "http.domain",
	"ORIGIN":                 "http.origin",
	"ALLOWED_ORIGINS":        "http.allowedorigins",
	"GO_APP_ENV":             "http.environment",
	"REQUEST_ID_HEADER":      "http.headers.requestid",
	"REAL_IP_HEADER":         "http.headers.realip",
	"SID_COOKIE_KEY":         "http.cookies.sid",
	"OAUTH_STATE_COOKIE_KEY": "http.cookies.oauthstate",
	"SESSION_DURATION":       "http.session.duration",
	"PAGE_SIZE":              "http.pagination.pagesize",
	"MAX_PAGE_SIZE":          "http.pagination.maxpagesize",

	// appconfig
	"ENABLE_TEST_ENDPOINTS": "app.enabletestendpoints",
	"TELEGRAM_BOT_TOKEN":    "app.telegrambottoken",

	// telemetryconfig
	"LOG_LEVEL":         "telemetry.loglevel",
	"LOKI_URL":          "telemetry.loki.url",
	"LOKI_APP_NAME":     "telemetry.loki.appname",
	"LOG_PATH":          "telemetry.loki.logpath",
	"OTEL_TEMPO_URL":    "telemetry.otel.tempourl",
	"OTEL_SERVICE_NAME": "telemetry.otel.servicename",

	// uploadsconfig
	"UPLOADS_PATH":      "uploads.path",
	"MAX_UPLOAD_SIZE":   "uploads.maxsize",
	"MAX_UPLOAD_MEMORY": "uploads.maxmemory",

	// redisconfig
	"REDIS_URL": "redis.url",

	// meiliconfig
	"MEILI_URL":     "meili.url",
	"MEILI_API_KEY": "meili.apikey",

	// dbconfig pool fields (DB_NAME / DB_HOST / DB_PORT / DB_USER / DB_PASSWORD already
	// transform correctly to db.name etc).
	"DB_MAX_CONNS":                "db.pool.maxconns",
	"DB_MIN_CONNS":                "db.pool.minconns",
	"DB_MAX_CONN_LIFETIME":        "db.pool.maxconnlifetime",
	"DB_MAX_CONN_LIFETIME_JITTER": "db.pool.maxconnlifetimejitter",
	"DB_MAX_CONN_IDLE_TIME":       "db.pool.maxconnidletime",
	"DB_HEALTH_CHECK_PERIOD":      "db.pool.healthcheckperiod",
	"DB_CONNECT_TIMEOUT":          "db.pool.connecttimeout",
	"MIGRATIONS_DIR":              "db.migrationsdir",

	// twofactorconfig
	"ENABLE_2FA":          "twofactor.enabled",
	"TOTP_ISSUER":         "twofactor.totpissuer",
	"TOTP_ENCRYPTION_KEY": "twofactor.encryptionkey",
	"OTP_CODE_LENGTH":     "twofactor.otp.codelength",
	"OTP_TTL_SECONDS":     "twofactor.otp.ttlseconds",
	"OTP_MAX_ATTEMPTS":    "twofactor.otp.maxattempts",
	"OTP_ENABLE_EMAIL":    "twofactor.otp.enableemail",
	"OTP_ENABLE_SMS":      "twofactor.otp.enablesms",

	// ratelimitconfig
	"RATE_LIMIT_ENABLED":    "ratelimit.enabled",
	"RATE_LIMIT_GLOBAL_RPS": "ratelimit.globalrps",
	"RATE_LIMIT_STORAGE":    "ratelimit.storage",
	"RATE_LIMIT_REDIS_URL":  "ratelimit.redisurl",

	// oidcconfig
	"OIDC_ISSUER_URL":             "oidc.issuerurl",
	"OIDC_CRYPTO_KEY":             "oidc.cryptokey",
	"OIDC_ACCESS_TOKEN_LIFETIME":  "oidc.accesstokenlifetime",
	"OIDC_REFRESH_TOKEN_LIFETIME": "oidc.refreshtokenlifetime",
	"OIDC_ID_TOKEN_LIFETIME":      "oidc.idtokenlifetime",

	// googleoauthconfig
	"GOOGLE_REDIRECT_URL":  "google.redirecturl",
	"GOOGLE_CLIENT_ID":     "google.clientid",
	"GOOGLE_CLIENT_SECRET": "google.clientsecret",

	// twilioconfig
	"TWILIO_WEBHOOK_URL":  "twilio.webhookurl",
	"TWILIO_ACCOUNT_SID":  "twilio.accountsid",
	"TWILIO_AUTH_TOKEN":   "twilio.authtoken",
	"TWILIO_PHONE_NUMBER": "twilio.phonenumber",

	// smtpconfig
	"SMTP_HOST":     "smtp.host",
	"SMTP_PORT":     "smtp.port",
	"SMTP_USERNAME": "smtp.username",
	"SMTP_PASSWORD": "smtp.password",
	"SMTP_FROM":     "smtp.from",

	// bichatconfig (legacy OPENAI_* and BICHAT_KNOWLEDGE_* variants)
	"OPENAI_API_KEY":              "bichat.openai.apikey",
	"OPENAI_KEY":                  "bichat.openai.apikey", // alternate legacy name
	"OPENAI_MODEL":                "bichat.openai.model",
	"OPENAI_BASE_URL":             "bichat.openai.baseurl",
	"OPENAI_API_RESOLVE_IP":       "bichat.openai.resolveip",
	"LANGFUSE_PUBLIC_KEY":         "bichat.langfuse.publickey",
	"LANGFUSE_SECRET_KEY":         "bichat.langfuse.secretkey",
	"LANGFUSE_BASE_URL":           "bichat.langfuse.baseurl",
	"LANGFUSE_HOST":               "bichat.langfuse.host",
	"BICHAT_KNOWLEDGE_DIR":        "bichat.knowledge.dir",
	"BICHAT_KB_INDEX_PATH":        "bichat.knowledge.kbindexpath",
	"BICHAT_SCHEMA_METADATA_DIR":  "bichat.knowledge.schemametadata",
	"BICHAT_KNOWLEDGE_AUTO_LOAD":  "bichat.knowledge.autoload",
	"IOTA_APPLET_VITE_URL_BICHAT": "bichat.applet.viteurl",
	"IOTA_APPLET_ENTRY_BICHAT":    "bichat.applet.entry",
	"IOTA_APPLET_CLIENT_BICHAT":   "bichat.applet.client",

	// paymentsconfig
	"CLICK_URL":              "payments.click.url",
	"CLICK_MERCHANT_ID":      "payments.click.merchantid",
	"CLICK_MERCHANT_USER_ID": "payments.click.merchantuserid",
	"CLICK_SERVICE_ID":       "payments.click.serviceid",
	"CLICK_SECRET_KEY":       "payments.click.secretkey",
	"PAYME_URL":              "payments.payme.url",
	"PAYME_MERCHANT_ID":      "payments.payme.merchantid",
	"PAYME_USER":             "payments.payme.user",
	"PAYME_SECRET_KEY":       "payments.payme.secretkey",
	"OCTO_SHOP_ID":           "payments.octo.shopid",
	"OCTO_SECRET":            "payments.octo.secret",
	"OCTO_SECRET_HASH":       "payments.octo.secrethash",
	"OCTO_NOTIFY_URL":        "payments.octo.notifyurl",
	"STRIPE_SECRET_KEY":      "payments.stripe.secretkey",
	"STRIPE_SIGNING_SECRET":  "payments.stripe.signingsecret",
}

// transformKey applies the locked single-underscore-to-dot transform with a
// legacy-alias bypass for env vars whose natural transform doesn't match the
// stdconfig koanf paths (multi-word leaf names, bare top-level vars, or
// renamed prefixes).
//
// Transform steps:
//  1. If the key is a known legacy alias, return its mapped path verbatim.
//  2. Otherwise: strip leading/trailing underscores, lowercase, replace each
//     remaining "_" with ".".
func transformKey(k, v string) (string, any) {
	if alias, ok := legacyAliases[k]; ok {
		return alias, v
	}
	k = strings.Trim(k, "_")
	k = strings.ToLower(k)
	k = strings.ReplaceAll(k, "_", ".")
	return k, v
}

func environToMap(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, entry := range environ {
		key, val, _ := strings.Cut(entry, "=")
		m[key] = val
	}
	return m
}

func mapToEnviron(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}
