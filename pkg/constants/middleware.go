package constants

import "github.com/go-playground/validator/v10"

type ContextKey string

const (
	UserKey        ContextKey = "user"
	SessionKey     ContextKey = "session"
	NavItemsKey    ContextKey = "navItems"
	AllNavItemsKey ContextKey = "allNavItems"
	DBKey          ContextKey = "db"
	TxKey          ContextKey = "tx"
	ParamsKey      ContextKey = "params"
	LoggerKey      ContextKey = "logger"
	AppKey         ContextKey = "app"
	LogoKey        ContextKey = "logo"
	HeadKey        ContextKey = "head"
	TabsKey        ContextKey = "tabs"
	RequestStart   ContextKey = "requestStart"
	LocalizerKey   ContextKey = "localizer"
)

var Validate = validator.New(validator.WithRequiredStructEnabled())
