package controllers

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type NotificationController struct{}

func NewNotificationController() application.Controller { return &NotificationController{} }
func (c *NotificationController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("core.notifications", 0, application.Route("", "/notifications", application.Authenticated()))
}
func (c *NotificationController) Register(r *mux.Router) {
	router := r.PathPrefix("/notifications").Subrouter()
	router.Use(middleware.Authorize(), middleware.RedirectNotAuthenticated(), middleware.ProvideUser(), middleware.ProvideDynamicLogo(), middleware.NavItems(), middleware.WithPageContext())
	router.HandleFunc("", di.H(c.Index)).Methods(http.MethodGet)
	router.HandleFunc("/dropdown", di.H(c.Dropdown)).Methods(http.MethodGet)
	router.HandleFunc("/summary", di.H(c.Summary)).Methods(http.MethodGet)
	router.HandleFunc("/read-all", di.H(c.MarkAllRead)).Methods(http.MethodPost)
	router.HandleFunc("/{id}/read", di.H(c.MarkRead)).Methods(http.MethodPost)
}
func (c *NotificationController) Index(r *http.Request, w http.ResponseWriter, service *services.NotificationService) {
	cursor := r.URL.Query().Get("cursor")
	if err := notification.ValidateCursor(cursor); err != nil {
		http.Error(w, "Invalid cursor", http.StatusBadRequest)
		return
	}
	registry := notificationFilterRegistry(r)
	conditions := registry.Decode(legacyUnreadQuery(r.URL.Query()))
	params := notification.FindParams{
		Limit: 21, Cursor: cursor,
		Filters: filterq.RepoFilters(conditions, registry.Schema(), map[string]filterq.RepoField[notification.Field]{
			notificationFilterUnread:    {Column: notification.FieldReadAt, Expr: func(col string) string { return col + " IS NULL" }},
			notificationFilterLevel:     {Column: notification.FieldLevel},
			notificationFilterCreatedAt: {Column: notification.FieldCreatedAt},
		}, time.Now()),
	}
	items, err := service.List(r.Context(), params)
	if err != nil {
		http.Error(w, "Unable to retrieve notifications", http.StatusInternalServerError)
		return
	}
	count, err := service.UnreadCount(r.Context())
	if err != nil {
		http.Error(w, "Unable to retrieve notifications", http.StatusInternalServerError)
		return
	}
	props := &notifications.Props{
		Notifications: items, UnreadCount: count, Cursor: cursor, HasMore: len(items) > 20,
		Filters: filterbuilder.Props{Registry: registry, Filters: conditions, ID: "notification-filters", ShowResetAlways: true},
	}
	if props.HasMore {
		props.Notifications = items[:20]
		props.NextCursor = notification.CursorFor(items[19])
	}
	// A history restore (cache miss on Back) needs the whole document.
	if htmx.WantsFullPage(r) {
		templ.Handler(notifications.Index(props)).ServeHTTP(w, r)
	} else {
		templ.Handler(notifications.ContentWithChips(props)).ServeHTTP(w, r)
	}
}

// Filter chip field keys of the notification center.
const (
	notificationFilterUnread    = "unread"
	notificationFilterLevel     = "level"
	notificationFilterCreatedAt = "created_at"
)

// notificationFilterRegistry describes the chips of the notification center:
// read state, importance and the day a notification arrived.
func notificationFilterRegistry(r *http.Request) *filterbuilder.Registry {
	pageCtx := composables.UsePageCtx(r.Context())
	return filterbuilder.NewRegistry(
		filterbuilder.FieldDef{Key: notificationFilterUnread, Type: filterq.FieldTypeBool, Label: pageCtx.T("Notifications.Unread")},
		filterbuilder.FieldDef{
			Key: notificationFilterLevel, Type: filterq.FieldTypeReference, Label: pageCtx.T("NotificationSettings.Level"),
			Options: []filterbuilder.Option{
				filterbuilder.Opt(string(notification.LevelInfo), pageCtx.T("NotificationSettings.LevelInfo")),
				filterbuilder.Opt(string(notification.LevelSuccess), pageCtx.T("NotificationSettings.LevelSuccess")),
				filterbuilder.Opt(string(notification.LevelWarning), pageCtx.T("NotificationSettings.LevelWarning")),
				filterbuilder.Opt(string(notification.LevelError), pageCtx.T("NotificationSettings.LevelError")),
			},
		},
		filterbuilder.FieldDef{Key: notificationFilterCreatedAt, Type: filterq.FieldTypeDate, Label: pageCtx.T("CreatedAt")},
	)
}

// legacyUnreadQuery keeps the former "Unread" tab link (?unread=true)
// working: it becomes the "unread is true" chip unless a chip on read state
// is already present.
func legacyUnreadQuery(q url.Values) url.Values {
	if q.Get("unread") != "true" {
		return q
	}
	for _, raw := range q[filterq.ParamName] {
		if c, ok := filterq.DecodeCondition(raw); ok && c.Field == notificationFilterUnread {
			return q
		}
	}
	out := make(url.Values, len(q)+1)
	for k, v := range q {
		out[k] = append([]string(nil), v...)
	}
	out.Add(filterq.ParamName, filterq.EncodeCondition(filterq.Condition{Field: notificationFilterUnread, Op: filterq.OpIs, Values: []string{"true"}}))
	return out
}
func (c *NotificationController) Summary(r *http.Request, w http.ResponseWriter, service *services.NotificationService) {
	count, err := service.UnreadCount(r.Context())
	if err != nil {
		http.Error(w, "Unable to retrieve notifications", http.StatusInternalServerError)
		return
	}
	templ.Handler(layouts.NotificationBellCount(count)).ServeHTTP(w, r)
}
func (c *NotificationController) MarkRead(r *http.Request, w http.ResponseWriter, service *services.NotificationService) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid notification ID", http.StatusBadRequest)
		return
	}
	if err := service.MarkRead(r.Context(), id); err != nil {
		if errors.Is(err, notification.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Unable to mark notification read", http.StatusInternalServerError)
		return
	}
	notificationReadResponse(w, r)
}
func (c *NotificationController) MarkAllRead(r *http.Request, w http.ResponseWriter, service *services.NotificationService) {
	if err := service.MarkAllRead(r.Context()); err != nil {
		http.Error(w, "Unable to mark notifications read", http.StatusInternalServerError)
		return
	}
	notificationReadResponse(w, r)
}
func notificationReadResponse(w http.ResponseWriter, r *http.Request) {
	if htmx.IsHxRequest(r) {
		htmx.SetTrigger(w, "notificationsChanged", "{}")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	shared.Redirect(w, r, "/notifications")
}

func (c *NotificationController) Dropdown(r *http.Request, w http.ResponseWriter, service *services.NotificationService) {
	items, err := service.List(r.Context(), notification.FindParams{Limit: 10})
	if err != nil {
		http.Error(w, "Unable to retrieve notifications", http.StatusInternalServerError)
		return
	}
	count, err := service.UnreadCount(r.Context())
	if err != nil {
		http.Error(w, "Unable to retrieve notifications", http.StatusInternalServerError)
		return
	}
	templ.Handler(layouts.NotificationDropdown(items, count)).ServeHTTP(w, r)
}
