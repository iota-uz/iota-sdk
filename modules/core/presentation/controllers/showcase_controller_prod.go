//go:build !dev

// Package controllers provides this package.
package controllers

import (
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
)

type ShowcaseController struct{}

func NewShowcaseController(_ *dbconfig.Config, _ *httpconfig.Config, _ *appconfig.Config) application.Controller {
	return nil
}
