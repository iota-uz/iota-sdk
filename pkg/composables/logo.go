package composables

import (
	"context"
	"errors"
	"github.com/a-h/templ"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/iota-agency/iota-sdk/pkg/types"
)

var (
	ErrNoHeadFound = errors.New("no head found")
)

// UseLogo returns the logo component from the context
func UseLogo(ctx context.Context) (templ.Component, error) {
	logo, ok := ctx.Value(constants.LogoKey).(templ.Component)
	if !ok {
		return nil, ErrNoHeadFound
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
func UseHead(ctx context.Context) (types.HeadComponent, error) {
	head, ok := ctx.Value(constants.HeadKey).(types.HeadComponent)
	if !ok {
		return nil, ErrNoHeadFound
	}
	return head, nil
}

// MustUseHead returns the head component from the context or panics
func MustUseHead(ctx context.Context) types.HeadComponent {
	head, err := UseHead(ctx)
	if err != nil {
		panic(err)
	}
	return head
}
