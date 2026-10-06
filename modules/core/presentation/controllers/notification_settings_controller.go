package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	settings "github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/notification_settings"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type NotificationSettingsController struct{}

func NewNotificationSettingsController() application.Controller {
	return &NotificationSettingsController{}
}
func (c *NotificationSettingsController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("core.notification_settings", 0, application.Route("", "/settings/notifications", application.RequireAll(permissions.NotificationRulesRead))).WithNav(application.NavNode{ID: "core.notification_settings", Parent: "core.administration", TitleKey: "NotificationSettings.Title", Path: "/settings/notifications", Order: 65})
}
func (c *NotificationSettingsController) Register(r *mux.Router) {
	router := r.PathPrefix("/settings/notifications").Subrouter()
	router.Use(middleware.Authorize(), middleware.RedirectNotAuthenticated(), middleware.ProvideUser(), middleware.ProvideDynamicLogo(), middleware.NavItems(), middleware.WithPageContext())
	router.HandleFunc("", di.H(c.Index)).Methods(http.MethodGet)
	router.HandleFunc("", di.H(c.Save)).Methods(http.MethodPost)
	router.HandleFunc("/test", di.H(c.Test)).Methods(http.MethodPost)
}
func (c *NotificationSettingsController) render(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService, message string, delivered int) {
	users, err := s.Recipients(r.Context())
	if err != nil {
		http.Error(w, "Unable to load recipients", http.StatusInternalServerError)
		return
	}
	props := &settings.Props{CanManage: composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) == nil, MessageKey: message, Delivered: delivered}
	tenant, err := composables.UseTenantID(r.Context())
	if err != nil {
		http.Error(w, "Unable to load tenant", http.StatusInternalServerError)
		return
	}
	for _, u := range users {
		if u.TenantID() == tenant && !u.IsBlocked() && u.Type() == user.TypeUser && u.Status() == user.StatusActive {
			props.Recipients = append(props.Recipients, settings.Recipient{ID: u.ID(), Name: u.FirstName() + " " + u.LastName() + " (" + u.Email().Value() + ")"})
		}
	}
	available := make(map[uint]bool, len(props.Recipients))
	for _, recipient := range props.Recipients {
		available[recipient.ID] = true
	}
	for _, d := range s.Catalog().Definitions() {
		rule, err := s.Rule(r.Context(), d.Key)
		if err != nil {
			http.Error(w, "Unable to load notification settings", http.StatusInternalServerError)
			return
		}
		missing := 0
		for _, id := range rule.UserIDs {
			if !available[id] {
				missing++
			}
		}
		props.Rules = append(props.Rules, settings.EventRule{Definition: d, Rule: rule, MissingRecipients: missing})
	}
	component := settings.Index(props)
	if htmx.IsHxRequest(r) {
		component = settings.Content(props)
	}
	templ.Handler(component).ServeHTTP(w, r)
}
func (c *NotificationSettingsController) Index(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService) {
	if composables.CanUserStrict(r.Context(), permissions.NotificationRulesRead) != nil {
		RenderForbidden(w, r)
		return
	}
	c.render(w, r, s, "", 0)
}
func (c *NotificationSettingsController) Save(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService) {
	if composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) != nil {
		RenderForbidden(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	rule := notifications.Rule{EventKey: r.FormValue("event_key"), Enabled: r.FormValue("enabled") == "true", UserIDs: []uint{}}
	for _, value := range r.Form["user_ids"] {
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil || id == 0 {
			http.Error(w, "Invalid recipient", http.StatusBadRequest)
			return
		}
		rule.UserIDs = append(rule.UserIDs, uint(id))
	}
	if err := s.SaveRule(r.Context(), rule); err != nil {
		var classified *serrors.Error
		if !errors.As(err, &classified) || classified.ErrorKind() != "validation" {
			composables.UseLogger(r.Context()).WithError(err).Error("failed to save notification rule")
			http.Error(w, "Unable to save notification rule", http.StatusInternalServerError)
			return
		}
		c.render(w, r, s, "InvalidRule", 0)
		return
	}
	c.render(w, r, s, "Saved", 0)
}
func (c *NotificationSettingsController) Test(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService) {
	if composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) != nil {
		RenderForbidden(w, r)
		return
	}
	tenant, err := composables.UseTenantID(r.Context())
	if err != nil {
		http.Error(w, "Unable to load tenant", http.StatusInternalServerError)
		return
	}
	count, err := s.Publish(r.Context(), notifications.Event{Key: notifications.TestEventKey, ID: uuid.NewString(), TenantID: tenant})
	if err != nil {
		http.Error(w, "Unable to send test notification", http.StatusInternalServerError)
		return
	}
	key := "TestSent"
	if count == 0 {
		key = "TestSkipped"
	}
	c.render(w, r, s, key, count)
}
