package controllers

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
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
	router.HandleFunc("/queue", di.H(c.Queue)).Methods(http.MethodGet)
	router.HandleFunc("/queue/{id}/retry", di.H(c.Retry)).Methods(http.MethodPost)
	router.HandleFunc("/test", di.H(c.Test)).Methods(http.MethodPost)
}
func (c *NotificationSettingsController) render(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService, message string, delivered int) {
	users, err := s.Recipients(r.Context())
	if err != nil {
		http.Error(w, "Unable to load recipients", http.StatusInternalServerError)
		return
	}
	props := &settings.Props{CanManage: composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) == nil, MessageKey: message, Delivered: delivered}
	props.Groups, err = s.Groups(r.Context())
	if err != nil {
		http.Error(w, "Unable to load groups", http.StatusInternalServerError)
		return
	}
	props.Roles, err = s.Roles(r.Context())
	if err != nil {
		http.Error(w, "Unable to load roles", http.StatusInternalServerError)
		return
	}
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
		groups := map[uuid.UUID]bool{}
		for _, g := range props.Groups {
			groups[g.ID] = true
		}
		for _, id := range rule.GroupIDs {
			if !groups[id] {
				missing++
			}
		}
		roles := map[uint]bool{}
		for _, role := range props.Roles {
			roles[role.ID] = true
		}
		for _, id := range rule.RoleIDs {
			if !roles[id] {
				missing++
			}
		}
		for _, id := range rule.UserIDs {
			if !available[id] {
				missing++
			}
		}
		props.Rules = append(props.Rules, settings.EventRule{Definition: d, Rule: rule, MissingRecipients: missing, MissingUsers: missingUserIDs(rule.UserIDs, available), MissingGroups: missingGroupIDs(rule.GroupIDs, groups), MissingRoles: missingUserIDs(rule.RoleIDs, roles), MissingKeys: missingRecipientKeys(rule.RecipientKeys, d.RecipientKeys)})
	}
	sort.SliceStable(props.Rules, func(i, j int) bool {
		if props.Rules[i].Definition.Module != props.Rules[j].Definition.Module {
			return props.Rules[i].Definition.Module < props.Rules[j].Definition.Module
		}
		return props.Rules[i].Definition.Key < props.Rules[j].Definition.Key
	})
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
	rule := notifications.Rule{Level: notification.Level(r.FormValue("level")), RecipientKeys: r.Form["recipient_keys"], EventKey: r.FormValue("event_key"), Enabled: r.FormValue("enabled") == "true", UserIDs: []uint{}}
	for _, value := range r.Form["user_ids"] {
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil || id == 0 {
			http.Error(w, "Invalid recipient", http.StatusBadRequest)
			return
		}
		rule.UserIDs = append(rule.UserIDs, uint(id))
	}
	for _, value := range r.Form["group_ids"] {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			http.Error(w, "Invalid group", http.StatusBadRequest)
			return
		}
		rule.GroupIDs = append(rule.GroupIDs, id)
	}
	for _, value := range r.Form["role_ids"] {
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil || id == 0 {
			http.Error(w, "Invalid role", http.StatusBadRequest)
			return
		}
		rule.RoleIDs = append(rule.RoleIDs, uint(id))
	}
	if err := s.SaveRule(r.Context(), rule); err != nil {
		if serrors.CodeOf(err) != serrors.Invalid {
			composables.UseLogger(r.Context()).WithError(err).Error("failed to save notification rule")
			http.Error(w, "Unable to save notification rule", http.StatusInternalServerError)
			return
		}
		c.render(w, r, s, "InvalidRule", 0)
		return
	}
	c.render(w, r, s, "Saved", 0)
}
func missingUserIDs(ids []uint, available map[uint]bool) []uint {
	var missing []uint
	for _, id := range ids {
		if !available[id] {
			missing = append(missing, id)
		}
	}
	return missing
}
func missingGroupIDs(ids []uuid.UUID, available map[uuid.UUID]bool) []uuid.UUID {
	var missing []uuid.UUID
	for _, id := range ids {
		if !available[id] {
			missing = append(missing, id)
		}
	}
	return missing
}
func missingRecipientKeys(keys []string, definitions []notifications.RecipientDefinition) []string {
	available := map[string]bool{}
	for _, d := range definitions {
		available[d.Key] = true
	}
	var missing []string
	for _, key := range keys {
		if !available[key] {
			missing = append(missing, key)
		}
	}
	return missing
}

func (c *NotificationSettingsController) Queue(w http.ResponseWriter, r *http.Request, dispatch *services.NotificationDispatchService) {
	if composables.CanUserStrict(r.Context(), permissions.NotificationRulesRead) != nil {
		RenderForbidden(w, r)
		return
	}
	stats, err := dispatch.Progress(r.Context())
	if err != nil {
		http.Error(w, "Unable to load delivery queue", http.StatusInternalServerError)
		return
	}
	jobs, err := dispatch.Jobs(r.Context())
	if err != nil {
		http.Error(w, "Unable to load delivery queue", http.StatusInternalServerError)
		return
	}
	templ.Handler(settings.Queue(stats, jobs, composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) == nil)).ServeHTTP(w, r)
}
func (c *NotificationSettingsController) Retry(w http.ResponseWriter, r *http.Request, dispatch *services.NotificationDispatchService) {
	if composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) != nil {
		RenderForbidden(w, r)
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid dispatch ID", http.StatusBadRequest)
		return
	}
	if err := dispatch.Retry(r.Context(), id); err != nil {
		http.Error(w, "Unable to retry delivery", http.StatusBadRequest)
		return
	}
	c.Queue(w, r, dispatch)
}
func (c *NotificationSettingsController) Test(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService, dispatch *services.NotificationDispatchService) {
	if composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) != nil {
		RenderForbidden(w, r)
		return
	}
	tenant, err := composables.UseTenantID(r.Context())
	if err != nil {
		http.Error(w, "Unable to load tenant", http.StatusInternalServerError)
		return
	}
	count, err := dispatch.Enqueue(r.Context(), notifications.Event{Key: notifications.TestEventKey, ID: uuid.NewString(), TenantID: tenant})
	if err != nil {
		http.Error(w, "Unable to enqueue test notification", http.StatusInternalServerError)
		return
	}
	key := "Queued"
	if count == 0 {
		key = "TestSkipped"
	}
	c.render(w, r, s, key, count)
}
