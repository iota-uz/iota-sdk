//go:build !dev

package agentsignin

import (
	"errors"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

func NewController(o Options) (application.Controller, error) {
	if o.Enabled {
		return nil, errors.New("agent sign-in is unavailable without the dev build tag")
	}
	return nil, nil
}
