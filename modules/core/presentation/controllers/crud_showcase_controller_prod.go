//go:build !dev

package controllers

import "github.com/iota-uz/iota-sdk/pkg/application"

type CrudShowcaseController struct{}

// ShowcaseEntity is a stub type for production builds
type ShowcaseEntity interface{}

func NewCrudShowcaseController(
	_ application.Application,
	_ ...CrudOption[ShowcaseEntity],
) application.Controller {
	return nil
}

func InitCrudShowcase(_ application.Application) {}
