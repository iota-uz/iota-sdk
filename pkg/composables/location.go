package composables

import (
	"context"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// WithLocation returns a context that carries the application time zone.
func WithLocation(ctx context.Context, loc *time.Location) context.Context {
	return context.WithValue(ctx, constants.LocationKey, loc)
}

// UseLocation returns the application time zone configured for the request.
// The second result is false when the application does not configure one.
func UseLocation(ctx context.Context) (*time.Location, bool) {
	loc, ok := ctx.Value(constants.LocationKey).(*time.Location)
	return loc, ok && loc != nil
}

// FormatDateTime formats t in the application time zone using the request
// locale's conventional layout. The second result is false when the
// application has no time zone configured, so callers can keep rendering the
// timestamp in the viewer's browser zone.
func FormatDateTime(ctx context.Context, t time.Time) (string, bool) {
	loc, ok := UseLocation(ctx)
	if !ok {
		return "", false
	}
	layout := "2006-01-02 15:04"
	if locale, found := intl.UseLocale(ctx); found {
		switch base, _ := locale.Base(); base.String() {
		case "ru", "uz":
			layout = "02.01.2006 15:04"
		}
	}
	return t.In(loc).Format(layout), true
}
