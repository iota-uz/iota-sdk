package composables

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"golang.org/x/text/language"
)

var (
	ErrNoLocalizer = errors.New("localizer not found")
	ErrNoLogger    = errors.New("logger not found")
)

type Params struct {
	IP            string
	UserAgent     string
	Authenticated bool
	Request       *http.Request
	Writer        http.ResponseWriter
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

// UseWriter returns the response writer from the context.
// If the response writer is not found, the second return value will be false.
func UseWriter(ctx context.Context) (http.ResponseWriter, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return nil, false
	}
	return params.Writer, true
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
func UseLogger(ctx context.Context) (*logrus.Entry, error) {
	logger := ctx.Value(constants.LoggerKey)
	if logger == nil {
		return nil, ErrNoLogger
	}
	return logger.(*logrus.Entry), nil
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

func useLocaleFromUser(ctx context.Context) (language.Tag, error) {
	user, err := UseUser(ctx)
	if err != nil {
		return language.Und, err
	}
	tag, err := language.Parse(string(user.UILanguage()))
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

// UsePageCtx returns the page context from the context.
// If the page context is not found, function will panic.
func UsePageCtx(ctx context.Context) *types.PageContext {
	pageCtx := ctx.Value(constants.PageContext)
	if pageCtx == nil {
		panic("page context not found")
	}
	v, ok := pageCtx.(*types.PageContext)
	if !ok {
		panic(fmt.Sprintf("page context is not of type *types.PageContext: %T", pageCtx))
	}
	return v
}

// WithPageCtx returns a new context with the page context.
func WithPageCtx(ctx context.Context, pageCtx *types.PageContext) context.Context {
	return context.WithValue(ctx, constants.PageContext, pageCtx)
}

func UseFlash(w http.ResponseWriter, r *http.Request, name string) (val []byte, err error) {
	c, err := r.Cookie(name)
	if err != nil {
		switch err {
		case http.ErrNoCookie:
			queryValue := r.URL.Query().Get(name)
			if queryValue != "" {
				return []byte(queryValue), nil
			}
			return nil, nil
		default:
			return nil, err
		}
	}
	val, err = base64.URLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil, err
	}
	dc := &http.Cookie{Name: name, MaxAge: -1, Expires: time.Unix(1, 0)}
	http.SetCookie(w, dc)
	return val, nil
}

func UseFlashMap[K comparable, V any](w http.ResponseWriter, r *http.Request, name string) (map[K]V, error) {
	bytes, err := UseFlash(w, r, name)
	if err != nil {
		return nil, err
	}
	var errorsMap map[K]V
	if len(bytes) == 0 {
		return errorsMap, nil
	}
	return errorsMap, json.Unmarshal(bytes, &errorsMap)
}

func UseQuery[T comparable](v T, r *http.Request) (T, error) {
	return v, shared.Decoder.Decode(v, r.URL.Query())
}

func UseForm[T comparable](v T, r *http.Request) (T, error) {
	if err := r.ParseForm(); err != nil {
		return v, err
	}
	return v, shared.Decoder.Decode(v, r.Form)
}
