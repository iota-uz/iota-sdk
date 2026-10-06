package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/di"
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
	router.HandleFunc("/summary", di.H(c.Summary)).Methods(http.MethodGet)
	router.HandleFunc("/read-all", di.H(c.MarkAllRead)).Methods(http.MethodPost)
	router.HandleFunc("/{id}/read", di.H(c.MarkRead)).Methods(http.MethodPost)
}
func notificationPage(r *http.Request) int {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 || page > 100000 {
		return 1
	}
	return page
}
func (c *NotificationController) Index(r *http.Request, w http.ResponseWriter, service *services.NotificationService) {
	page := notificationPage(r)
	unread := r.URL.Query().Get("unread") == "true"
	items, err := service.List(r.Context(), notification.FindParams{Limit: 21, Offset: (page - 1) * 20, UnreadOnly: unread})
	if err != nil {
		http.Error(w, "Unable to retrieve notifications", http.StatusInternalServerError)
		return
	}
	count, err := service.UnreadCount(r.Context())
	if err != nil {
		http.Error(w, "Unable to retrieve notifications", http.StatusInternalServerError)
		return
	}
	props := &notifications.Props{Notifications: items, UnreadCount: count, Page: page, UnreadOnly: unread, HasMore: len(items) > 20}
	if props.HasMore {
		props.Notifications = items[:20]
	}
	if htmx.IsHxRequest(r) {
		templ.Handler(notifications.Content(props)).ServeHTTP(w, r)
	} else {
		templ.Handler(notifications.Index(props)).ServeHTTP(w, r)
	}
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
