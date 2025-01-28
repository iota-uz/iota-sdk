package composables

import (
	"context"
	"errors"
	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

var (
	ErrNoLogoFound = errors.New("no logo found")
	ErrNoHeadFound = errors.New("no head found")
)

// UseLogo returns the logo component from the context
func UseLogo(ctx context.Context) (templ.Component, error) {
	logo, ok := ctx.Value(constants.LogoKey).(templ.Component)
	if !ok {
		return nil, ErrNoLogoFound
	}
	return logo, nil
}

// MustUseLogo returns the logo component from the context or panics
func MustUseLogo(ctx context.Context) templ.Component {
	logo, err := UseLogo(ctx)
	if err != nil {
		panic(err)
	}
	return logo
}

// UseHead returns the head component from the context
func UseHead(ctx context.Context) (templ.Component, error) {
	head, ok := ctx.Value(constants.HeadKey).(templ.Component)
	if !ok {
		return nil, ErrNoHeadFound
	}
	return head, nil
}

// MustUseHead returns the head component from the context or panics
func MustUseHead(ctx context.Context) templ.Component {
	head, err := UseHead(ctx)
	if err != nil {
		panic(err)
	}
	return head
}
