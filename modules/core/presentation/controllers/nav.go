package controllers

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

func firstPermissionSet(values [][]permission.Permission) []permission.Permission {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func navRouteOptions(perms []permission.Permission) []application.RouteOption {
	if len(perms) == 0 {
		return nil
	}
	return []application.RouteOption{application.RequireAll(perms...)}
}

func navAuthPolicy(perms []permission.Permission) *application.AuthPolicy {
	if len(perms) == 0 {
		return nil
	}
	return &application.AuthPolicy{
		Permissions: perms,
		Logic:       application.PermissionLogicAll,
	}
}

func spotlightOnlySurface() map[application.Surface]application.SurfaceOptions {
	return map[application.Surface]application.SurfaceOptions{
		application.SurfaceSpotlight: {},
	}
}
