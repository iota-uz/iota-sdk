package stdconfig

import (
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/bichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/googleoauthconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/pagination"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/session"
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

// Bundle holds pointers to all stdconfig types, populated by RegisterAll.
type Bundle struct {
	App         *appconfig.Config
	Bichat      *bichatconfig.Config
	DB          *dbconfig.Config
	GoogleOAuth *googleoauthconfig.Config
	HTTP        *httpconfig.Config
	Headers     *headers.Config
	Cookies     *cookies.Config
	Session     *session.Config
	Pagination  *pagination.Config
	Meili       *meiliconfig.Config
	OIDC        *oidcconfig.Config
	Payments    *paymentsconfig.Config
	RateLimit   *ratelimitconfig.Config
	Redis       *redisconfig.Config
	SMTP        *smtpconfig.Config
	Telemetry   *telemetryconfig.Config
	Twilio      *twilioconfig.Config
	TwoFactor   *twofactorconfig.Config
	Uploads     *uploadsconfig.Config
}

// RegisterAll registers all stdconfig types into r and returns a Bundle
// with non-nil pointers to each. Any registration error aborts immediately
// and returns (nil, error).
//
// Example:
//
//	src, _ := config.Build(env.New(".env").WithAliases(stdconfig.AllLegacyAliases()...))
//	r := config.NewRegistry(src)
//	b, err := stdconfig.RegisterAll(r)
func RegisterAll(r *config.Registry) (*Bundle, error) {
	b := &Bundle{}
	var err error

	if b.App, err = config.Register[appconfig.Config](r); err != nil {
		return nil, err
	}
	if b.Bichat, err = config.Register[bichatconfig.Config](r); err != nil {
		return nil, err
	}
	if b.DB, err = config.Register[dbconfig.Config](r); err != nil {
		return nil, err
	}
	if b.GoogleOAuth, err = config.Register[googleoauthconfig.Config](r); err != nil {
		return nil, err
	}
	if b.HTTP, err = config.Register[httpconfig.Config](r); err != nil {
		return nil, err
	}
	if b.Headers, err = config.Register[headers.Config](r); err != nil {
		return nil, err
	}
	if b.Cookies, err = config.Register[cookies.Config](r); err != nil {
		return nil, err
	}
	if b.Session, err = config.Register[session.Config](r); err != nil {
		return nil, err
	}
	if b.Pagination, err = config.Register[pagination.Config](r); err != nil {
		return nil, err
	}
	if b.Meili, err = config.Register[meiliconfig.Config](r); err != nil {
		return nil, err
	}
	if b.OIDC, err = config.Register[oidcconfig.Config](r); err != nil {
		return nil, err
	}
	if b.Payments, err = config.Register[paymentsconfig.Config](r); err != nil {
		return nil, err
	}
	if b.RateLimit, err = config.Register[ratelimitconfig.Config](r); err != nil {
		return nil, err
	}
	if b.Redis, err = config.Register[redisconfig.Config](r); err != nil {
		return nil, err
	}
	if b.SMTP, err = config.Register[smtpconfig.Config](r); err != nil {
		return nil, err
	}
	if b.Telemetry, err = config.Register[telemetryconfig.Config](r); err != nil {
		return nil, err
	}
	if b.Twilio, err = config.Register[twilioconfig.Config](r); err != nil {
		return nil, err
	}
	if b.TwoFactor, err = config.Register[twofactorconfig.Config](r); err != nil {
		return nil, err
	}
	if b.Uploads, err = config.Register[uploadsconfig.Config](r); err != nil {
		return nil, err
	}

	return b, nil
}
