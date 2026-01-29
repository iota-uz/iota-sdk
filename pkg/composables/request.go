package composables

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var (
	ErrNoLogger = errors.New("logger not found")
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

// UseLogger returns the logger from the context.
// If the logger is not found, the second return value will be false.
func UseLogger(ctx context.Context) *logrus.Entry {
	logger := ctx.Value(constants.LoggerKey)
	if logger == nil {
		panic("logger not found")
	}
	return logger.(*logrus.Entry)
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

// UsePageCtx returns the page context from the context.
// If the page context is not found, function will panic.
func UsePageCtx(ctx context.Context) types.PageContextProvider {
	pageCtx := ctx.Value(constants.PageContext)
	if pageCtx == nil {
		panic("page context not found")
	}
	v, ok := pageCtx.(types.PageContextProvider)
	if !ok {
		panic(fmt.Sprintf("page context is not of type PageContextProvider: %T", pageCtx))
	}
	return v
}

// WithPageCtx returns a new context with the page context.
// Accepts any type implementing PageContextProvider interface for extensibility.
func WithPageCtx(ctx context.Context, pageCtx types.PageContextProvider) context.Context {
	return context.WithValue(ctx, constants.PageContext, pageCtx)
}

func UseFlash(w http.ResponseWriter, r *http.Request, name string) ([]byte, error) {
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
	val, err := base64.URLEncoding.DecodeString(c.Value)
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

// GetLastQueryParam returns the last occurrence of a query parameter.
// This is useful when HTMX includes form data via hx-include="closest form",
// which appends form values to the URL, creating duplicate parameters.
// The last occurrence represents the current form state, while earlier
// occurrences may be stale values from the URL.
//
// Example:
//
//	URL: /loads?driver=uuid-1&sort=load&driver=uuid-2
//	GetLastQueryParam(r, "driver") returns "uuid-2"
func GetLastQueryParam(r *http.Request, key string) string {
	values := r.URL.Query()[key]
	if len(values) > 0 {
		return values[len(values)-1]
	}
	return ""
}

// GetLastQueryParams returns the last occurrence of multiple query parameters.
// This is optimized for retrieving several filter parameters at once.
//
// Example:
//
//	params := GetLastQueryParams(r, "driver", "status", "broker")
//	driverID := params["driver"]
//	status := params["status"]
func GetLastQueryParams(r *http.Request, keys ...string) map[string]string {
	result := make(map[string]string, len(keys))
	query := r.URL.Query()
	for _, key := range keys {
		if values := query[key]; len(values) > 0 {
			result[key] = values[len(values)-1]
		}
	}
	return result
}

// UseRequest returns the HTTP request from the context.
// If the request is not found, the second return value will be false.
func UseRequest(ctx context.Context) (*http.Request, bool) {
	params, ok := UseParams(ctx)
	if !ok {
		return nil, false
	}
	return params.Request, true
}

// GetSessionAudience determines the session audience based on the request origin.
// Returns "website" for requests from website domain (eai.uz), otherwise returns "granite" for admin/internal apps.
// Note: Origin/Host/Referer headers can be spoofed by malicious clients. This should be used alongside
// token-based audience validation in middleware for security.
func GetSessionAudience(r *http.Request) string {
	// Check request origin/host to determine audience
	origin := r.Header.Get("Origin")
	host := r.Host
	referer := r.Header.Get("Referer")

	// Check if this is a website request by looking at origin, host, or referer
	websiteDomains := []string{"eai.uz", "eai-staging.uz"}

	for _, domain := range websiteDomains {
		// Use suffix matching to avoid false positives (e.g., "noteai.uz" matching "eai.uz")
		// Also check for ".domain" pattern to match subdomains
		if strings.HasSuffix(origin, domain) || strings.HasSuffix(host, domain) ||
			strings.Contains(origin, "."+domain) || strings.Contains(host, "."+domain) ||
			strings.Contains(referer, domain) {
			return "website"
		}
	}

	// Default to granite for admin/internal applications
	return "granite"
}
