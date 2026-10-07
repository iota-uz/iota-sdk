package controllers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/assert"
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

type notificationListRepo struct {
	notification.Repository
	recipient uint
	params    notification.FindParams
	items     []notification.Notification
	count     int64
}

func (r *notificationListRepo) List(_ context.Context, recipient uint, p notification.FindParams) ([]notification.Notification, error) {
	r.recipient = recipient
	r.params = p
	return r.items, nil
}
func (r *notificationListRepo) UnreadCount(_ context.Context, recipient uint) (int64, error) {
	r.recipient = recipient
	return r.count, nil
}
func TestNotificationDropdown_AuthenticatedRecipientAndLimit(t *testing.T) {
	item, err := notification.New(42, "<Title>", "Body")
	require.NoError(t, err)
	repo := &notificationListRepo{count: 2, items: []notification.Notification{item}}
	req := httptest.NewRequest(http.MethodGet, "/notifications/dropdown?user_id=999", nil)
	ctx := composables.WithUser(req.Context(), user.New("Tester", "", nil, "en", user.WithID(42)))
	ctx = composables.WithPageCtx(ctx, settingsPageContext{})
	response := httptest.NewRecorder()
	controllers.NewNotificationController().(*controllers.NotificationController).Dropdown(req.WithContext(ctx), response, services.NewNotificationService(repo))
	require.Equal(t, 42, int(repo.recipient))
	require.Equal(t, 10, repo.params.Limit)
	require.Contains(t, response.Body.String(), "&lt;Title&gt;")
	require.Contains(t, response.Body.String(), "notification-dropdown-item")
}
func TestNotificationHistory_InvalidCursor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/notifications?cursor=bad", nil)
	response := httptest.NewRecorder()
	controllers.NewNotificationController().(*controllers.NotificationController).Index(req, response, nil)
	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestNotificationSummary_LoadsWithAuthenticatedRenderContext(t *testing.T) {
	repo := &notificationListRepo{count: 7}
	middleware := services.WithNotificationSummary(services.NewNotificationService(repo))
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := composables.WithUser(r.Context(), user.New("Tester", "", nil, "en", user.WithID(42)))
		assert.Equal(t, int64(7), services.NotificationUnreadCount(ctx))
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, uint(42), repo.recipient)
}

func TestNotificationHistory_CursorPagination(t *testing.T) {
	var items []notification.Notification
	for i := 0; i < 21; i++ {
		item, err := notification.New(42, "Title", "Body")
		require.NoError(t, err)
		items = append(items, item)
	}
	cursor := notification.CursorFor(items[0])
	repo := &notificationListRepo{items: items}
	req := httptest.NewRequest(http.MethodGet, "/notifications?cursor="+cursor, nil)
	req.Header.Set("Hx-Request", "true")
	ctx := composables.WithUser(req.Context(), user.New("Tester", "", nil, "en", user.WithID(42)))
	ctx = composables.WithPageCtx(ctx, settingsPageContext{})
	response := httptest.NewRecorder()
	controllers.NewNotificationController().(*controllers.NotificationController).Index(req.WithContext(ctx), response, services.NewNotificationService(repo))
	require.Equal(t, 21, repo.params.Limit)
	require.Equal(t, cursor, repo.params.Cursor)
	require.Equal(t, 20, strings.Count(response.Body.String(), `data-testid="notification"`))
	require.Contains(t, response.Body.String(), notification.CursorFor(items[19]))
	require.NotContains(t, response.Body.String(), "every 20s")
}
