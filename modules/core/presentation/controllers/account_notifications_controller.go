package controllers

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/account"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// AccountNotificationsController is the «Notifications» section of the
// profile: the person's channels and linked accounts (Telegram).
type AccountNotificationsController struct{}

func NewAccountNotificationsController() application.Controller {
	return &AccountNotificationsController{}
}

func (c *AccountNotificationsController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("core.account.notifications", 0, application.Route("", account.NotificationsPath)).WithNav(application.NavNode{
		ID: "core.account.notifications", Parent: "core.account", TitleKey: "Account.Notifications.Title",
		Path: account.NotificationsPath, Surfaces: spotlightOnlySurface(),
	})
}

func (c *AccountNotificationsController) Register(r *mux.Router) {
	router := r.PathPrefix(account.NotificationsPath).Subrouter()
	router.Use(middleware.Authorize(), middleware.RedirectNotAuthenticated(), middleware.ProvideUser(), middleware.ProvideDynamicLogo(), middleware.NavItems(), middleware.WithPageContext())
	router.HandleFunc("", di.H(c.Index)).Methods(http.MethodGet)
	router.HandleFunc("/channels/{channel}", di.H(c.Toggle)).Methods(http.MethodPost)
	router.HandleFunc("/links/{channel}/start", di.H(c.Start)).Methods(http.MethodPost)
	router.HandleFunc("/links/{channel}/status", di.H(c.Status)).Methods(http.MethodGet)
	router.HandleFunc("/links/{channel}/cancel", di.H(c.Cancel)).Methods(http.MethodPost)
	router.HandleFunc("/links/{channel}/unlink", di.H(c.Unlink)).Methods(http.MethodPost)
	router.HandleFunc("/test/{channel}", di.H(c.Test)).Methods(http.MethodPost)
	router.HandleFunc("/test/{channel}/{event}", di.H(c.TestStatus)).Methods(http.MethodGet)
}

func channelParam(r *http.Request) notifications.Channel {
	return notifications.Channel(mux.Vars(r)["channel"])
}

func pageLanguage(r *http.Request) string {
	return composables.UsePageCtx(r.Context()).GetLocale().String()
}

// props builds the section for the current person.
func (c *AccountNotificationsController) props(r *http.Request, channels *services.NotificationChannelService, links *services.NotificationChannelLinkService) (*account.NotificationsProps, error) {
	const op serrors.Op = "AccountNotificationsController.props"
	ctx := r.Context()
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return nil, serrors.New(serrors.PermissionDenied, "no current user").WithOp(op)
	}
	specs, err := channels.AvailableSpecs(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	disabled, err := channels.Disabled(ctx, []uint{u.ID()})
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	lang := pageLanguage(r)
	props := &account.NotificationsProps{}
	for _, spec := range specs {
		row := account.NotificationChannel{
			Channel: string(spec.Channel), Label: spec.LabelFor(lang),
			Enabled:  !slices.Contains(disabled[u.ID()], spec.Channel),
			External: spec.Channel.External(), Linked: spec.Address == notifications.AddressLinked, HasAddress: true,
		}
		if row.Linked {
			status, err := links.StatusOf(ctx, u.ID(), spec.Channel)
			if err != nil {
				return nil, serrors.Wrap(op, err)
			}
			row.HasAddress = status.Linked
			props.Links = append(props.Links, linkView(spec.Channel, row.Label, status))
		}
		props.Channels = append(props.Channels, row)
	}
	return props, nil
}

func linkView(channel notifications.Channel, label string, s notifications.LinkStatus) account.NotificationLink {
	view := account.NotificationLink{Channel: string(channel), Label: label, State: account.LinkNone}
	switch {
	case s.Linked:
		view.State, view.Display = account.LinkActive, s.Display
	case s.Pending:
		view.State, view.ExpiresAt = account.LinkPending, s.ExpiresAt
	case s.Expired:
		view.State = account.LinkExpired
	case s.Revoked != "":
		view.State = account.LinkGone
	}
	return view
}

func (c *AccountNotificationsController) render(w http.ResponseWriter, r *http.Request, props *account.NotificationsProps) {
	component := account.Notifications(props)
	if htmx.IsHxRequest(r) && r.URL.Query().Get("part") == "links" {
		component = account.NotificationLinks(props)
	}
	if !htmx.IsHxRequest(r) {
		component = account.NotificationsPage(props)
	}
	templ.Handler(component).ServeHTTP(w, r)
}

func (c *AccountNotificationsController) renderSection(w http.ResponseWriter, r *http.Request, channels *services.NotificationChannelService, links *services.NotificationChannelLinkService, mutate func(*account.NotificationsProps)) {
	props, err := c.props(r, channels, links)
	if err != nil {
		writeAccountNotificationsError(w, r, err)
		return
	}
	if mutate != nil {
		mutate(props)
	}
	c.render(w, r, props)
}

func writeAccountNotificationsError(w http.ResponseWriter, r *http.Request, err error) {
	if serrors.CodeOf(err) == serrors.PermissionDenied {
		RenderForbidden(w, r)
		return
	}
	composables.UseLogger(r.Context()).WithError(err).Error("failed to load notification settings of the account")
	http.Error(w, "Unable to load notification settings", http.StatusInternalServerError)
}

// Index serves the section (htmx) or the page.
func (c *AccountNotificationsController) Index(w http.ResponseWriter, r *http.Request, channels *services.NotificationChannelService, links *services.NotificationChannelLinkService) {
	c.renderSection(w, r, channels, links, func(p *account.NotificationsProps) {
		if r.URL.Query().Get("error") == "unavailable" {
			p.ErrorKey = "Unavailable"
		}
	})
}

// Toggle turns one channel on or off for the current person.
func (c *AccountNotificationsController) Toggle(w http.ResponseWriter, r *http.Request, channels *services.NotificationChannelService, links *services.NotificationChannelLinkService) {
	u, err := composables.UseUser(r.Context())
	if err != nil || u == nil {
		RenderForbidden(w, r)
		return
	}
	enabled, err := strconv.ParseBool(r.FormValue("enabled"))
	if err != nil {
		http.Error(w, "Invalid value", http.StatusBadRequest)
		return
	}
	if err := channels.SetChannelEnabled(r.Context(), u.ID(), channelParam(r), enabled); err != nil {
		if serrors.CodeOf(err) == serrors.Invalid {
			http.Error(w, "The channel is not available", http.StatusUnprocessableEntity)
			return
		}
		composables.UseLogger(r.Context()).WithError(err).Error("failed to switch a notification channel")
		http.Error(w, "Unable to save", http.StatusInternalServerError)
		return
	}
	// The accounts block follows the switch (the test action shows only for a channel that is on); the
	// switches themselves are not re-rendered, so a change made meanwhile is never dropped.
	htmx.SetTrigger(w, account.ChannelsChangedEvent, "")
	w.WriteHeader(http.StatusNoContent)
}

// Start issues a link and sends the browser (a new tab, or the app on a
// phone) to the provider's confirmation.
func (c *AccountNotificationsController) Start(w http.ResponseWriter, r *http.Request, links *services.NotificationChannelLinkService) {
	start, err := links.StartLink(r.Context(), channelParam(r))
	if err != nil {
		if !errors.Is(err, notifications.ErrChannelUnavailable) {
			composables.UseLogger(r.Context()).WithError(err).Error("failed to start a channel link")
		}
		http.Redirect(w, r, account.NotificationsPath+"?error=unavailable", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, start.URL, http.StatusSeeOther)
}

// Status re-renders the section; with ?pending=1 it answers 204 while the
// link still waits, so the poller does not repaint an unchanged section. A
// link that arrived is announced with a toast.
func (c *AccountNotificationsController) Status(w http.ResponseWriter, r *http.Request, channels *services.NotificationChannelService, links *services.NotificationChannelLinkService) {
	channel := channelParam(r)
	status, err := links.Status(r.Context(), channel)
	if err != nil {
		writeAccountNotificationsError(w, r, err)
		return
	}
	if r.URL.Query().Get("pending") == "1" {
		if status.Pending {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if status.Linked {
			pageCtx := composables.UsePageCtx(r.Context())
			label := string(channel)
			if spec, _, ok := channels.Registry().Lookup(channel); ok {
				label = spec.LabelFor(pageLanguage(r))
			}
			htmx.ToastSuccess(w, pageCtx.T("Account.Notifications.Toast.LinkedTitle", map[string]any{"Channel": label}),
				pageCtx.T("Account.Notifications.Toast.Linked", map[string]any{"Channel": label, "Account": status.Display}))
		}
	}
	c.renderSection(w, r, channels, links, nil)
}

// Cancel drops a link that waits for confirmation.
func (c *AccountNotificationsController) Cancel(w http.ResponseWriter, r *http.Request, channels *services.NotificationChannelService, links *services.NotificationChannelLinkService) {
	if err := links.CancelLink(r.Context(), channelParam(r)); err != nil {
		writeAccountNotificationsError(w, r, err)
		return
	}
	c.renderSection(w, r, channels, links, nil)
}

// Unlink removes the person's account of a channel.
func (c *AccountNotificationsController) Unlink(w http.ResponseWriter, r *http.Request, channels *services.NotificationChannelService, links *services.NotificationChannelLinkService) {
	channel := channelParam(r)
	if err := links.Unlink(r.Context(), channel); err != nil {
		composables.UseLogger(r.Context()).WithError(err).Error("failed to unlink a notification channel")
		c.renderSection(w, r, channels, links, func(p *account.NotificationsProps) {
			for i := range p.Links {
				if p.Links[i].Channel == string(channel) {
					p.Links[i].ErrorKey = "Unlink"
				}
			}
		})
		return
	}
	pageCtx := composables.UsePageCtx(r.Context())
	label := string(channel)
	if spec, _, ok := channels.Registry().Lookup(channel); ok {
		label = spec.LabelFor(pageLanguage(r))
	}
	htmx.ToastSuccess(w, pageCtx.T("Account.Notifications.Toast.UnlinkedTitle", map[string]any{"Channel": label}), "")
	c.renderSection(w, r, channels, links, nil)
}

// Test sends a test notice to the current person on one channel.
func (c *AccountNotificationsController) Test(w http.ResponseWriter, r *http.Request, channels *services.NotificationChannelService, dispatch *services.NotificationDispatchService) {
	ctx := r.Context()
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		RenderForbidden(w, r)
		return
	}
	channel := channelParam(r)
	label := string(channel)
	if spec, _, ok := channels.Registry().Lookup(channel); ok {
		label = spec.LabelFor(pageLanguage(r))
	}
	result := account.NotificationTestResult{Channel: string(channel), Label: label}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		http.Error(w, "Unable to load tenant", http.StatusInternalServerError)
		return
	}
	if err := channels.Validate(ctx, []notifications.Channel{channel}); err != nil || !channel.External() {
		result.Status = "unavailable"
		templ.Handler(account.TestResult(result)).ServeHTTP(w, r)
		return
	}
	disabled, err := channels.Disabled(ctx, []uint{u.ID()})
	if err != nil {
		http.Error(w, "Unable to load channel settings", http.StatusInternalServerError)
		return
	}
	if slices.Contains(disabled[u.ID()], channel) {
		result.Status = "disabled"
		templ.Handler(account.TestResult(result)).ServeHTTP(w, r)
		return
	}
	receipt, err := dispatch.Send(ctx, notifications.AudienceRequest{
		Event:    notifications.Event{Key: notifications.TestEventKey, ID: uuid.NewString(), TenantID: tenant},
		Audience: notifications.Audience{Users: []uint{u.ID()}},
		Channels: []notifications.Channel{channel},
	})
	if err != nil {
		composables.UseLogger(ctx).WithError(err).Error("failed to send a test notification")
		result.Status = "other"
	}
	result.EventID = receipt.NoticeID.String()
	templ.Handler(account.TestResult(result)).ServeHTTP(w, r)
}

// TestStatus says what became of a test notice (by its notice ID).
func (c *AccountNotificationsController) TestStatus(w http.ResponseWriter, r *http.Request, channels *services.NotificationChannelService) {
	ctx := r.Context()
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		RenderForbidden(w, r)
		return
	}
	channel := channelParam(r)
	eventID := mux.Vars(r)["event"]
	noticeID, err := uuid.Parse(eventID)
	if err != nil {
		http.Error(w, "Invalid notice", http.StatusBadRequest)
		return
	}
	polls, _ := strconv.Atoi(r.URL.Query().Get("polls"))
	label := string(channel)
	if spec, _, ok := channels.Registry().Lookup(channel); ok {
		label = spec.LabelFor(pageLanguage(r))
	}
	result := account.NotificationTestResult{Channel: string(channel), Label: label, EventID: eventID, Polls: polls}
	deliveries, err := channels.NoticeDeliveries(ctx, noticeID)
	if err != nil {
		http.Error(w, "Unable to load the result", http.StatusInternalServerError)
		return
	}
	for _, d := range deliveries {
		if d.Channel != channel || d.UserID != u.ID() {
			continue
		}
		switch d.Status {
		case notifications.DeliverySent:
			result.Status = "sent"
		case notifications.DeliveryNoAddress:
			result.Status = "no_address"
		case notifications.DeliverySkipped:
			result.Status = "unavailable"
		case notifications.DeliveryFailed, notifications.DeliveryAmbiguous:
			result.Status = "other"
			if d.ErrorCode == notifications.ErrorCodeProviderRejected {
				result.Status = "gone"
			}
		case notifications.DeliveryUnresolved, notifications.DeliveryPending:
		}
	}
	// A notice that never left the queue in two minutes is reported, not polled forever.
	if result.Status == "" && polls > 120 {
		result.Status = "other"
	}
	templ.Handler(account.TestResult(result)).ServeHTTP(w, r)
}
