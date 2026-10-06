package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/require"
)

type notificationControllerRepo struct {
	notification.Repository
	recipient uint
	id        uuid.UUID
	all       bool
}

func (r *notificationControllerRepo) MarkRead(_ context.Context, recipient uint, id uuid.UUID) error {
	r.recipient = recipient
	r.id = id
	return nil
}
func (r *notificationControllerRepo) MarkAllRead(_ context.Context, recipient uint) error {
	r.recipient = recipient
	r.all = true
	return nil
}

func TestNotificationRead_UsesAuthenticatedRecipient(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "all"}[all], func(t *testing.T) {
			repo := &notificationControllerRepo{}
			service := services.NewNotificationService(repo)
			c := controllers.NewNotificationController().(*controllers.NotificationController)
			id := uuid.New()
			request := httptest.NewRequest(http.MethodPost, "/notifications/"+id.String()+"/read?user_id=999", nil)
			request = mux.SetURLVars(request, map[string]string{"id": id.String()})
			request = request.WithContext(composables.WithUser(request.Context(), user.New("Tester", "", nil, "en", user.WithID(42))))
			request.Header.Set("Hx-Request", "true")
			response := httptest.NewRecorder()
			if all {
				c.MarkAllRead(request, response, service)
			} else {
				c.MarkRead(request, response, service)
			}
			require.Equal(t, http.StatusNoContent, response.Code)
			require.Equal(t, uint(42), repo.recipient)
			require.Contains(t, response.Header().Get("Hx-Trigger"), "notificationsChanged")
			if all {
				require.True(t, repo.all)
			} else {
				require.Equal(t, id, repo.id)
			}
		})
	}
}
func TestNotificationRead_InvalidID(t *testing.T) {
	repo := &notificationControllerRepo{}
	c := controllers.NewNotificationController().(*controllers.NotificationController)
	request := mux.SetURLVars(httptest.NewRequest(http.MethodPost, "/notifications/bad/read", nil), map[string]string{"id": "bad"})
	response := httptest.NewRecorder()
	c.MarkRead(request, response, services.NewNotificationService(repo))
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Zero(t, repo.recipient)
}
