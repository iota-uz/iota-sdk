//go:build !dev

package controllers

import "github.com/iota-uz/iota-sdk/pkg/application"

type ShowcaseController struct{}

func NewShowcaseController(_ application.Application) application.Controller {
	return nil
}
