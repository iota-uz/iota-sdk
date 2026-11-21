// Package constants provides this package.
package constants

import "github.com/go-playground/validator/v10"

type ContextKey string

const (
	UserKey        ContextKey = "user"
	SessionKey     ContextKey = "session"
	NavItemsKey    ContextKey = "navItems"
	AllNavItemsKey ContextKey = "allNavItems"
	TxKey          ContextKey = "poolTx"
	// AfterCommitKey carries the owner of the transaction in TxKey, which runs
	// the callbacks added to it after the transaction commits.
	AfterCommitKey  ContextKey = "afterCommit"
	PoolKey         ContextKey = "pool"
	ParamsKey       ContextKey = "params"
	LoggerKey       ContextKey = "logger"
	AppKey          ContextKey = "app"
	ContainerKey    ContextKey = "container"
	LogoKey         ContextKey = "logo"
	HeadKey         ContextKey = "head"
	SidebarPropsKey ContextKey = "sidebarProps"
	RequestStart    ContextKey = "requestStart"
	LocationKey     ContextKey = "location"

	PageContext ContextKey = "pageContext"
	TenantIDKey ContextKey = "tenant"
)

var Validate = validator.New(validator.WithRequiredStructEnabled())
