package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/realtime"
	"github.com/sirupsen/logrus"
)

type NotificationInvalidator interface {
	Notify(context.Context, notification.Notification)
	Invalidate(context.Context, uuid.UUID, uint)
}
type NotificationRealtimeService struct {
	app    application.Application
	logger *logrus.Logger
}

func NewNotificationRealtimeService(app application.Application, logger *logrus.Logger) *NotificationRealtimeService {
	return &NotificationRealtimeService{app: app, logger: logger}
}
func (s *NotificationRealtimeService) Notify(ctx context.Context, n notification.Notification) {
	s.publish(ctx, n.TenantID(), n.UserID(), map[string]any{"type": "notification.changed", "notification": map[string]any{"id": n.ID(), "title": n.Title(), "body": n.Body(), "level": n.Level()}})
}
func (s *NotificationRealtimeService) Invalidate(ctx context.Context, tenant uuid.UUID, userID uint) {
	s.publish(ctx, tenant, userID, map[string]string{"type": "notification.changed"})
}
func (s *NotificationRealtimeService) publish(ctx context.Context, tenant uuid.UUID, userID uint, data any) {
	hub := s.app.Websocket()
	if hub == nil {
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		s.logger.WithError(err).Error("failed to encode notification invalidation")
		return
	}
	publishCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = hub.Publish(publishCtx, realtime.Envelope{ID: uuid.New(), TenantID: tenant, Channel: realtime.UserChannel(tenant, userID), Payload: payload, PublishedAt: time.Now().UTC()})
	if err != nil {
		s.logger.WithError(err).WithField("tenant_id", tenant).WithField("user_id", userID).Warn("notification realtime delivery failed; persisted state remains available")
	}
}
