package controllers

import (
	"strings"

	"github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

type ControllerOptions struct {
	BasePath                string
	RequireAccessPermission permission.Permission
	ReadAllPermission       permission.Permission
}

type ControllerOption func(*ControllerOptions)

func WithBasePath(path string) ControllerOption {
	return func(o *ControllerOptions) { o.BasePath = path }
}

func WithRequireAccessPermission(p permission.Permission) ControllerOption {
	return func(o *ControllerOptions) { o.RequireAccessPermission = p }
}

func WithReadAllPermission(p permission.Permission) ControllerOption {
	return func(o *ControllerOptions) { o.ReadAllPermission = p }
}

func defaultControllerOptions() ControllerOptions {
	return ControllerOptions{
		BasePath:          "/bi-chat",
		ReadAllPermission: permissions.BiChatReadAll,
	}
}

func applyControllerOptions(opts ...ControllerOption) ControllerOptions {
	o := defaultControllerOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	o.BasePath = normalizeBasePath(o.BasePath)
	if o.ReadAllPermission == nil {
		o.ReadAllPermission = permissions.BiChatReadAll
	}
	return o
}

func normalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/bi-chat"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if p != "/" {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}
