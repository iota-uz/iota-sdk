package composables

import (
	"context"
	"errors"
	"github.com/iota-agency/iota-sdk/pkg/constants"
)

var (
	ErrNoLogoFound    = errors.New("no logo found")
	ErrNoFaviconFound = errors.New("no favicon found")
)

// UseLogoURL returns the logo URL from the context
func UseLogoURL(ctx context.Context) (string, error) {
	url, ok := ctx.Value(constants.LogoKey).(string)
	if !ok {
		return "", ErrNoLogoFound
	}
	return url, nil
}

// UseFaviconURL returns the favicon URL from the context
func UseFaviconURL(ctx context.Context) (string, error) {
	url, ok := ctx.Value(constants.FaviconKey).(string)
	if !ok {
		return "", ErrNoFaviconFound
	}
	return url, nil
}
