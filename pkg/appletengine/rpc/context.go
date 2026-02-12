package rpc

import "context"

type contextKey string

const (
	tenantIDContextKey  contextKey = "appletengine.tenant_id"
	appletIDContextKey  contextKey = "appletengine.applet_id"
	userIDContextKey    contextKey = "appletengine.user_id"
	requestIDContextKey contextKey = "appletengine.request_id"
)

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDContextKey, tenantID)
}

func TenantIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(tenantIDContextKey).(string)
	return value, ok && value != ""
}

func WithAppletID(ctx context.Context, appletID string) context.Context {
	return context.WithValue(ctx, appletIDContextKey, appletID)
}

func AppletIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(appletIDContextKey).(string)
	return value, ok && value != ""
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(userIDContextKey).(string)
	return value, ok && value != ""
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(requestIDContextKey).(string)
	return value, ok && value != ""
}
