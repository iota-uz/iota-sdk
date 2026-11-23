package controllers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	settings "github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/notification_settings"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const maxRecipientSearchLength = 100

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
	router.HandleFunc("/recipients", di.H(c.Recipients)).Methods(http.MethodGet)
	router.HandleFunc("/queue", di.H(c.Queue)).Methods(http.MethodGet)
	router.HandleFunc("/queue/{id}/retry", di.H(c.Retry)).Methods(http.MethodPost)
	router.HandleFunc("/test", di.H(c.Test)).Methods(http.MethodPost)
	router.HandleFunc("/users", di.H(c.UserChannels)).Methods(http.MethodGet)
	router.HandleFunc("/users/{id:[0-9]+}", di.H(c.SaveUserChannels)).Methods(http.MethodPost)
}
func (c *NotificationSettingsController) render(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService, message string, delivered int) {
	props := &settings.Props{Channels: s.Channels(), CanManage: composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) == nil, MessageKey: message, Delivered: delivered}
	var err error
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
	definitions := s.Catalog().Definitions()
	rules := make(map[string]notifications.Rule, len(definitions))
	var named []uint
	for _, d := range definitions {
		rule, err := s.Rule(r.Context(), d.Key)
		if err != nil {
			http.Error(w, "Unable to load notification settings", http.StatusInternalServerError)
			return
		}
		rules[d.Key] = rule
		named = append(named, rule.UserIDs...)
	}
	recipients, err := s.RecipientsByIDs(r.Context(), named)
	if err != nil {
		http.Error(w, "Unable to load recipients", http.StatusInternalServerError)
		return
	}
	available := make(map[uint]bool, len(recipients))
	for _, recipient := range recipients {
		props.Recipients = append(props.Recipients, settings.Recipient{ID: recipient.ID, Name: recipient.Name, Hint: recipient.Hint})
		available[recipient.ID] = true
	}
	for _, d := range definitions {
		rule := rules[d.Key]
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

// Recipients serves one page of the rule picker's people. Only managers edit
// rules, so read-only users have no reason to list the directory.
func (c *NotificationSettingsController) Recipients(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService) {
	if composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) != nil {
		RenderForbidden(w, r)
		return
	}
	search := r.URL.Query().Get("q")
	if utf8.RuneCountInString(search) > maxRecipientSearchLength {
		http.Error(w, "Search is too long", http.StatusBadRequest)
		return
	}
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}
	page, err := s.SearchRecipients(r.Context(), search, offset)
	if err != nil {
		composables.UseLogger(r.Context()).WithError(err).Error("failed to search notification recipients")
		http.Error(w, "Unable to load recipients", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settings.NewRecipientPage(page.Items, page.HasMore)); err != nil {
		composables.UseLogger(r.Context()).WithError(err).Error("failed to write notification recipients")
	}
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
	for _, value := range r.Form["channels"] {
		rule.Channels = append(rule.Channels, notifications.Channel(value))
	}
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
	// With a channel picker on the page, an empty selection must not silently
	// fall back to in-app (for example after an adapter was removed).
	if len(s.Channels()) > 1 && len(rule.Channels) == 0 {
		c.render(w, r, s, "InvalidRule", 0)
		return
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

// UserChannels lists people with their personal channel choices.
func (c *NotificationSettingsController) UserChannels(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService) {
	c.renderUserChannels(w, r, s, "")
}

func (c *NotificationSettingsController) renderUserChannels(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService, message string) {
	if composables.CanUserStrict(r.Context(), permissions.NotificationRulesRead) != nil {
		RenderForbidden(w, r)
		return
	}
	// ?q= stays the plain search box; chips come on top as ?f=. A save posts
	// both back, so the list it re-renders is the one the person was looking at.
	search := r.FormValue("q")
	if utf8.RuneCountInString(search) > maxRecipientSearchLength {
		http.Error(w, "Search is too long", http.StatusBadRequest)
		return
	}
	offset, err := strconv.Atoi(r.Form.Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}
	channels := s.Channels()
	registry := c.userChannelFilterRegistry(r, s, channels)
	conditions := registry.Decode(r.Form)
	page, err := s.FilterRecipients(r.Context(), search, recipientFilters(conditions, channels), offset)
	if err != nil {
		composables.UseLogger(r.Context()).WithError(err).Error("failed to search notification recipients")
		http.Error(w, "Unable to load recipients", http.StatusInternalServerError)
		return
	}
	ids := make([]uint, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.ID)
	}
	disabled, err := s.UserChannels(r.Context(), ids)
	if err != nil {
		http.Error(w, "Unable to load channel settings", http.StatusInternalServerError)
		return
	}
	props := &settings.UserChannelsProps{
		Channels: channels, Search: search, MessageKey: message,
		CanManage: composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) == nil,
		Filters: filterbuilder.Props{
			Registry: registry, Filters: conditions, ID: "user-channel-filters", ShowResetAlways: true,
		},
		Offset:  offset,
		HasMore: page.HasMore,
	}
	for _, item := range page.Items {
		props.Rows = append(props.Rows, settings.UserChannelRow{Recipient: settings.Recipient{ID: item.ID, Name: item.Name, Hint: item.Hint}, Disabled: disabled[item.ID]})
	}
	component := settings.UserChannels(props)
	// A history restore ("Back" to a pushed chip URL) needs the whole page.
	if !htmx.WantsFullPage(r) {
		switch htmx.Target(r) {
		case settings.UserChannelsListID:
			component = settings.UserChannelsListWithChips(props)
		case settings.UserChannelsMoreID:
			component = settings.UserChannelRows(props)
		default:
			component = settings.UserChannelsContent(props)
		}
	}
	templ.Handler(component).ServeHTTP(w, r)
}

// userChannelFilterRegistry describes the chips of the channels-per-person
// list: the person's roles and groups, and the channels they turned off
// (offered only when there is more than the in-app channel to turn off).
func (c *NotificationSettingsController) userChannelFilterRegistry(r *http.Request, s *services.NotificationRoutingService, channels []notifications.Channel) *filterbuilder.Registry {
	pageCtx := composables.UsePageCtx(r.Context())
	var fields []filterbuilder.FieldDef
	if roles, err := s.Roles(r.Context()); err != nil {
		composables.UseLogger(r.Context()).WithError(err).Warn("failed to load roles for the channel filters")
	} else if len(roles) > 0 {
		options := make([]filterbuilder.Option, 0, len(roles))
		for _, role := range roles {
			options = append(options, filterbuilder.Opt(strconv.FormatUint(uint64(role.ID), 10), role.Name))
		}
		fields = append(fields, filterbuilder.FieldDef{
			Key: notifications.RecipientFilterRole, Type: filterq.FieldTypeReference,
			Label: pageCtx.T("NotificationSettings.FilterRole"), Options: options,
		})
	}
	if groups, err := s.Groups(r.Context()); err != nil {
		composables.UseLogger(r.Context()).WithError(err).Warn("failed to load groups for the channel filters")
	} else if len(groups) > 0 {
		options := make([]filterbuilder.Option, 0, len(groups))
		for _, group := range groups {
			options = append(options, filterbuilder.Opt(group.ID.String(), group.Name))
		}
		fields = append(fields, filterbuilder.FieldDef{
			Key: notifications.RecipientFilterGroup, Type: filterq.FieldTypeReference,
			Label: pageCtx.T("NotificationSettings.FilterGroup"), Options: options,
		})
	}
	if len(channels) > 1 {
		options := make([]filterbuilder.Option, 0, len(channels))
		for _, channel := range channels {
			options = append(options, filterbuilder.Opt(string(channel), pageCtx.T(settings.ChannelLabelKey(channel))))
		}
		fields = append(fields, filterbuilder.FieldDef{
			Key: notifications.RecipientFilterOptedOut, Type: filterq.FieldTypeReference,
			Label: pageCtx.T("NotificationSettings.FilterOptedOut"), Options: options,
			// "is empty": every channel on the page is on for the person.
			Nullable: true,
		})
	}
	return filterbuilder.NewRegistry(fields...)
}

// recipientFilters hands the chips to the recipient directory. "No channel
// turned off" means none of the channels shown, so an opt-out kept for a
// channel that is not installed does not hide the person.
func recipientFilters(conditions filterq.FilterSet, channels []notifications.Channel) filterq.FilterSet {
	out := make(filterq.FilterSet, 0, len(conditions))
	for _, cond := range conditions {
		if cond.Field == notifications.RecipientFilterOptedOut && cond.Op == filterq.OpEmpty {
			values := make([]string, 0, len(channels))
			for _, channel := range channels {
				values = append(values, string(channel))
			}
			cond = filterq.Condition{Field: cond.Field, Op: filterq.OpIsNot, Values: values}
		}
		out = append(out, cond)
	}
	return out
}

// SaveUserChannels stores which channels one person wants.
func (c *NotificationSettingsController) SaveUserChannels(w http.ResponseWriter, r *http.Request, s *services.NotificationRoutingService) {
	if composables.CanUserStrict(r.Context(), permissions.NotificationRulesManage) != nil {
		RenderForbidden(w, r)
		return
	}
	id, err := strconv.ParseUint(mux.Vars(r)["id"], 10, 32)
	if err != nil || id == 0 {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	enabled := make([]notifications.Channel, 0, len(r.Form["channels"]))
	for _, value := range r.Form["channels"] {
		enabled = append(enabled, notifications.Channel(value))
	}
	if err := s.SetUserChannels(r.Context(), uint(id), enabled); err != nil {
		if serrors.CodeOf(err) != serrors.Invalid {
			composables.UseLogger(r.Context()).WithError(err).Error("failed to save user notification channels")
			http.Error(w, "Unable to save channel settings", http.StatusInternalServerError)
			return
		}
		c.renderUserChannels(w, r, s, "InvalidChannels")
		return
	}
	c.renderUserChannels(w, r, s, "UserChannelsSaved")
}
