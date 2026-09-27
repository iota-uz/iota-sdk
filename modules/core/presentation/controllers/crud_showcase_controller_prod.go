//go:build !dev

// Package controllers provides this package.
package controllers

import (
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CrudShowcaseController struct{}

// ShowcaseEntity is a stub type for production builds
type ShowcaseEntity interface{}

func NewCrudShowcaseController(
	_ eventbus.EventBus,
	_ ...CrudOption[ShowcaseEntity],
) application.Controller {
	return nil
}

func InitCrudShowcase(_ *pgxpool.Pool) {}
