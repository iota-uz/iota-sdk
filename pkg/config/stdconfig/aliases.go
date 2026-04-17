// Package stdconfig aggregates all stdconfig sub-packages.
package stdconfig

import (
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

// AllLegacyAliases returns all per-package legacy alias maps as a slice.
// Pass to env.Provider.WithAliases to wire them in one call:
//
//	env.New(".env").WithAliases(stdconfig.AllLegacyAliases()...)
func AllLegacyAliases() []map[string]string {
	return []map[string]string{
		appconfig.LegacyAliases(),
		bichatconfig.LegacyAliases(),
		dbconfig.LegacyAliases(),
		googleoauthconfig.LegacyAliases(),
		httpconfig.LegacyAliases(),
		headers.LegacyAliases(),
		cookies.LegacyAliases(),
		session.LegacyAliases(),
		pagination.LegacyAliases(),
		meiliconfig.LegacyAliases(),
		oidcconfig.LegacyAliases(),
		paymentsconfig.LegacyAliases(),
		ratelimitconfig.LegacyAliases(),
		redisconfig.LegacyAliases(),
		smtpconfig.LegacyAliases(),
		telemetryconfig.LegacyAliases(),
		twilioconfig.LegacyAliases(),
		twofactorconfig.LegacyAliases(),
		uploadsconfig.LegacyAliases(),
	}
}
