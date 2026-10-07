package services

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
)

type notificationSummaryKey struct{}

func NotificationUnreadCount(ctx context.Context) int64 {
	loader, _ := ctx.Value(notificationSummaryKey{}).(func(context.Context) int64)
	if loader == nil {
		return 0
	}
	return loader(ctx)
}
func WithNotificationSummary(service *NotificationService) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			loader := func(ctx context.Context) int64 { count, _ := service.UnreadCount(ctx); return count }
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), notificationSummaryKey{}, loader)))
		})
	}
}
