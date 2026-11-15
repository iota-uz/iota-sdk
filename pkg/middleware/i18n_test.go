package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func resolveLocale(t *testing.T, supported []string, opts middleware.LocaleOptions, acceptLanguage string) string {
	t.Helper()
	bundle := i18n.NewBundle(language.Russian)
	var (
		got   string
		found bool
	)
	handler := middleware.ProvideLocalizer(bundle, supported, opts)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		locale, ok := intl.UseLocale(r.Context())
		found = ok
		got = locale.String()
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if acceptLanguage != "" {
		req.Header.Set("Accept-Language", acceptLanguage)
	}
	handler.ServeHTTP(httptest.NewRecorder(), req)
	require.True(t, found)
	return got
}

func TestProvideLocalizer_DefaultLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		supported      []string
		opts           middleware.LocaleOptions
		acceptLanguage string
		want           string
	}{
		{name: "no config and no header keeps English", supported: nil, want: "en"},
		{name: "no config and matching header", supported: nil, acceptLanguage: "uz", want: "uz"},
		{name: "configured default replaces English without header", supported: []string{"en", "ru"}, opts: middleware.LocaleOptions{DefaultLanguage: "ru"}, want: "ru"},
		{name: "configured default wins for unmatched header", supported: []string{"en", "ru"}, opts: middleware.LocaleOptions{DefaultLanguage: "ru"}, acceptLanguage: "fr", want: "ru"},
		{name: "matching header still wins over default", supported: []string{"en", "ru"}, opts: middleware.LocaleOptions{DefaultLanguage: "ru"}, acceptLanguage: "en", want: "en"},
		{name: "only ru ignores English header", supported: []string{"ru"}, acceptLanguage: "en-US,en;q=0.9", want: "ru"},
		{name: "only ru without header is ru, not English", supported: []string{"ru"}, want: "ru"},
		{name: "default that is not enabled is ignored", supported: []string{"en", "ru"}, opts: middleware.LocaleOptions{DefaultLanguage: "uz"}, want: "en"},
		{name: "accept language high priority uses default", supported: []string{"en", "ru"}, opts: middleware.LocaleOptions{AcceptLanguageHighPriority: true, DefaultLanguage: "ru"}, want: "ru"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, resolveLocale(t, tt.supported, tt.opts, tt.acceptLanguage))
		})
	}
}
