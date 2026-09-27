package configtesting_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config/configtesting"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/bichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/googleoauthconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/meiliconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/oidcconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/paymentsconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/ratelimitconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/redisconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/smtpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/telemetryconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twilioconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twofactorconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
)

func TestPopulated_appconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[appconfig.Config](t, map[string]any{
		"app.environment": "production",
	})
	if cfg.Environment != "production" {
		t.Errorf("Environment: got %q, want %q", cfg.Environment, "production")
	}
	// Other fields get defaults — EnableTestEndpoints should be false (zero).
	if cfg.EnableTestEndpoints {
		t.Error("EnableTestEndpoints should default to false")
	}
}

func TestPopulated_bichatconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[bichatconfig.Config](t, map[string]any{
		"bichat.openai.model": "gpt-4o",
	})
	if cfg.OpenAI.Model != "gpt-4o" {
		t.Errorf("OpenAI.Model: got %q, want %q", cfg.OpenAI.Model, "gpt-4o")
	}
	// Default ViteURL still applied.
	if cfg.Applet.ViteURL != "http://localhost:5173" {
		t.Errorf("Applet.ViteURL default: got %q", cfg.Applet.ViteURL)
	}
}

func TestPopulated_dbconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[dbconfig.Config](t, map[string]any{
		"db.name": "mydb",
	})
	if cfg.Name != "mydb" {
		t.Errorf("Name: got %q, want %q", cfg.Name, "mydb")
	}
	// Host default still applied.
	if cfg.Host != "localhost" {
		t.Errorf("Host default: got %q, want %q", cfg.Host, "localhost")
	}
}

func TestPopulated_googleoauthconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[googleoauthconfig.Config](t, map[string]any{
		"googleoauth.clientid": "my-client-id",
	})
	if cfg.ClientID != "my-client-id" {
		t.Errorf("ClientID: got %q, want %q", cfg.ClientID, "my-client-id")
	}
}

func TestPopulated_httpconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[httpconfig.Config](t, map[string]any{
		"http.port": 9090,
	})
	if cfg.Port != 9090 {
		t.Errorf("Port: got %d, want 9090", cfg.Port)
	}
	// Domain default still applied.
	if cfg.Domain != "localhost" {
		t.Errorf("Domain default: got %q, want %q", cfg.Domain, "localhost")
	}
}

func TestPopulated_meiliconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[meiliconfig.Config](t, map[string]any{
		"meili.url": "http://search.local:7700",
	})
	if cfg.URL != "http://search.local:7700" {
		t.Errorf("URL: got %q, want %q", cfg.URL, "http://search.local:7700")
	}
}

func TestPopulated_oidcconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[oidcconfig.Config](t, map[string]any{
		"oidc.issuerurl": "https://auth.example.com",
	})
	if cfg.IssuerURL != "https://auth.example.com" {
		t.Errorf("IssuerURL: got %q", cfg.IssuerURL)
	}
	// AccessTokenLifetime default.
	if cfg.AccessTokenLifetime != time.Hour {
		t.Errorf("AccessTokenLifetime default: got %v, want %v", cfg.AccessTokenLifetime, time.Hour)
	}
}

func TestPopulated_paymentsconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[paymentsconfig.Config](t, map[string]any{
		"payments.click.merchantid": int64(42),
	})
	if cfg.Click.MerchantID != 42 {
		t.Errorf("Click.MerchantID: got %d, want 42", cfg.Click.MerchantID)
	}
	// Click.URL default.
	if cfg.Click.URL != "https://my.click.uz" {
		t.Errorf("Click.URL default: got %q", cfg.Click.URL)
	}
}

func TestPopulated_ratelimitconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[ratelimitconfig.Config](t, map[string]any{
		"ratelimit.globalrps": 500,
	})
	if cfg.GlobalRPS != 500 {
		t.Errorf("GlobalRPS: got %d, want 500", cfg.GlobalRPS)
	}
	if cfg.Storage != "memory" {
		t.Errorf("Storage default: got %q, want %q", cfg.Storage, "memory")
	}
}

func TestPopulated_redisconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[redisconfig.Config](t, map[string]any{
		"redis.url": "redis://myhost:6380",
	})
	if cfg.URL != "redis://myhost:6380" {
		t.Errorf("URL: got %q", cfg.URL)
	}
}

func TestPopulated_smtpconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[smtpconfig.Config](t, map[string]any{
		"smtp.host": "mail.example.com",
	})
	if cfg.Host != "mail.example.com" {
		t.Errorf("Host: got %q", cfg.Host)
	}
	if cfg.Port != 587 {
		t.Errorf("Port default: got %d, want 587", cfg.Port)
	}
}

func TestPopulated_telemetryconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[telemetryconfig.Config](t, map[string]any{
		"telemetry.loglevel": "debug",
	})
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.Loki.AppName != "sdk" {
		t.Errorf("Loki.AppName default: got %q, want %q", cfg.Loki.AppName, "sdk")
	}
}

func TestPopulated_twilioconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[twilioconfig.Config](t, map[string]any{
		"twilio.accountsid": "AC-test-sid",
	})
	if cfg.AccountSID != "AC-test-sid" {
		t.Errorf("AccountSID: got %q", cfg.AccountSID)
	}
}

func TestPopulated_twofactorconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[twofactorconfig.Config](t, map[string]any{
		"twofactor.totpissuer": "MyApp",
	})
	if cfg.TOTPIssuer != "MyApp" {
		t.Errorf("TOTPIssuer: got %q", cfg.TOTPIssuer)
	}
	if cfg.OTP.CodeLength != 6 {
		t.Errorf("OTP.CodeLength default: got %d, want 6", cfg.OTP.CodeLength)
	}
}

func TestPopulated_uploadsconfig(t *testing.T) {
	t.Parallel()
	cfg := configtesting.Populated[uploadsconfig.Config](t, map[string]any{
		"uploads.path": "/var/uploads",
	})
	if cfg.Path != "/var/uploads" {
		t.Errorf("Path: got %q", cfg.Path)
	}
	if cfg.MaxSize != 33554432 {
		t.Errorf("MaxSize default: got %d, want 33554432", cfg.MaxSize)
	}
}
