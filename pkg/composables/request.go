package composables

import (
	"context"
	"errors"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strconv"

	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	ErrLocalizerNotFound = errors.New("localizer not found")
)

type Params struct {
	IP            string
	UserAgent     string
	Authenticated bool
	Request       *http.Request
	Writer        http.ResponseWriter
	Meta          map[string]interface{}
}

// UseParams returns the request parameters from the context.
// If the parameters are not found, the second return value will be false.
func UseParams(ctx context.Context) (*Params, bool) {
	params, ok := ctx.Value(constants.ParamsKey).(*Params)
	return params, ok
}

// WithParams returns a new context with the request parameters.
func WithParams(ctx context.Context, params *Params) context.Context {
	return context.WithValue(ctx, constants.ParamsKey, params)
}

// UseRequest returns the request from the context.
// If the request is not found, the second return value will be false.
func UseRequest(ctx context.Context) (*http.Request, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return nil, false
	}
	return params.Request, true
}

// UseLogger returns the logger from the context.
// If the logger is not found, the second return value will be false.
func UseLogger(ctx context.Context) (*log.Logger, bool) {
	logger, ok := ctx.Value(constants.LoggerKey).(*log.Logger)
	if !ok {
		return nil, false
	}
	return logger, true
}

// UseMeta returns the metadata from the context.
// If the metadata is not found, the second return value will be false.
func UseMeta(ctx context.Context) (map[string]interface{}, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return nil, false
	}
	return params.Meta, true
}

// WithTx returns a new context with the database transaction.
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, constants.TxKey, tx)
}

// UseTx returns the database transaction from the context.
// If the transaction is not found, the second return value will be false.
func UseTx(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(constants.TxKey).(*gorm.DB)
	if !ok {
		return nil, false
	}
	return tx, true
}

// UseAuthenticated returns whether the user is authenticated and the second return value is true.
// If the user is not authenticated, the second return value is false.
func UseAuthenticated(ctx context.Context) bool {
	params, ok := UseParams(ctx)
	if !ok {
		return false
	}
	return params.Authenticated
}

// UseIP returns the IP address from the context.
// If the IP address is not found, the second return value will be false.
func UseIP(ctx context.Context) (string, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return "", false
	}
	return params.IP, true
}

// UseUserAgent returns the user agent from the context.
// If the user agent is not found, the second return value will be false.
func UseUserAgent(ctx context.Context) (string, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return "", false
	}
	return params.UserAgent, true
}

// UseWriter returns the response writer from the context.
// If the response writer is not found, the second return value will be false.
func UseWriter(ctx context.Context) (http.ResponseWriter, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return nil, false
	}
	return params.Writer, true
}

func useLocaleFromUser(ctx context.Context) (language.Tag, error) {
	user, err := UseUser(ctx)
	if err != nil {
		return language.Und, err
	}
	tag, err := language.Parse(string(user.UILanguage))
	if err != nil {
		return language.Und, err
	}
	return tag, nil
}

// UseLocale returns the locale from the context.
// If the locale is not found, the second return value will be false.
func UseLocale(ctx context.Context, defaultLocale language.Tag) language.Tag {
	tag, err := useLocaleFromUser(ctx)
	if err == nil {
		return tag
	}
	params, ok := UseParams(ctx)
	if !ok {
		return defaultLocale
	}
	headerValue := params.Request.Header.Get("Accept-Language")
	tags, _, err := language.ParseAcceptLanguage(headerValue)
	if err != nil {
		return defaultLocale
	}
	if len(tags) == 0 {
		return defaultLocale
	}
	return tags[0]
}

type PaginationParams struct {
	Limit  int
	Offset int
	Page   int
}

func UsePaginated(r *http.Request) PaginationParams {
	config := configuration.Use()
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit > config.MaxPageSize {
		limit = config.PageSize
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	return PaginationParams{
		Limit:  limit,
		Offset: page * limit,
		Page:   page,
	}
}

func UsePageCtx(r *http.Request, pageData *types.PageData) (*types.PageContext, error) {
	localizer, found := UseLocalizer(r.Context())
	if !found {
		return nil, ErrLocalizerNotFound
	}
	uniTranslator, found := UseUniLocalizer(r.Context())
	if !found {
		return nil, ErrLocalizerNotFound
	}
	locale := UseLocale(r.Context(), language.English)
	navItems, _ := UseNavItems(r)
	return &types.PageContext{
		Pathname:      r.URL.Path,
		Localizer:     localizer,
		Title:         localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: pageData.Title}), //nolint:exhaustruct
		Locale:        locale.String(),
		UniTranslator: uniTranslator,
		NavItems:      navItems,
	}, nil
}
