// Package controllers provides this package.
package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

type SpotlightController struct {
	app      application.Application
	basePath string
}

type spotlightSearchPayload struct {
	SearchID string                       `json:"search_id"`
	HTML     string                       `json:"html"`
	Loading  bool                         `json:"loading"`
	Complete bool                         `json:"complete"`
	Pending  int                          `json:"pending"`
	Stages   []spotlight.SearchStageState `json:"stages"`
}

func NewSpotlightController(app application.Application) application.Controller {
	return &SpotlightController{
		app:      app,
		basePath: "/spotlight",
	}
}

func (c *SpotlightController) Key() string {
	return c.basePath
}

func (c *SpotlightController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.ProvideUser(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideLocalizer(c.app),
	)
	router.HandleFunc("/search", c.Search).Methods(http.MethodGet)
	router.HandleFunc("/stream", c.Stream).Methods(http.MethodGet)
	router.HandleFunc("/cancel", c.Cancel).Methods(http.MethodPost)
}

func (c *SpotlightController) Search(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")

	q := r.URL.Query().Get("q")
	if q == "" {
		_ = json.NewEncoder(w).Encode(spotlightSearchPayload{
			SearchID: "",
			HTML:     "",
			Loading:  false,
			Complete: true,
			Pending:  0,
		})
		return
	}

	req, err := c.buildSearchRequest(r, q)
	if err != nil {
		http.Error(w, "Failed to build Spotlight request", http.StatusInternalServerError)
		return
	}

	snapshot, err := c.app.Spotlight().CreateSession(r.Context(), req)
	if err != nil {
		composables.UseLogger(r.Context()).WithError(err).WithField("query", q).Error("spotlight session creation failed")
		http.Error(w, "Failed to search Spotlight", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(c.snapshotPayload(r, snapshot))
}

func (c *SpotlightController) Stream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sessionID := r.URL.Query().Get("search_id")
	if sessionID == "" {
		http.Error(w, "search_id is required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	updates, err := c.app.Spotlight().SubscribeSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, "search session not found", http.StatusNotFound)
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case snapshot, ok := <-updates:
			if !ok {
				return
			}
			if err := writeSSE(w, "update", c.snapshotPayload(r, snapshot)); err != nil {
				return
			}
			flusher.Flush()
			if snapshot.Completed {
				return
			}
		}
	}
}

func (c *SpotlightController) Cancel(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("search_id")
	if sessionID == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	c.app.Spotlight().CancelSession(sessionID)
	w.WriteHeader(http.StatusNoContent)
}

func (c *SpotlightController) buildSearchRequest(r *http.Request, q string) (spotlight.SearchRequest, error) {
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		return spotlight.SearchRequest{}, err
	}

	userID := ""
	roles := make([]string, 0, 8)
	permissions := make([]string, 0, 32)
	if user, userErr := composables.UseUser(r.Context()); userErr == nil {
		userID = fmt.Sprintf("%d", user.ID())
		for _, role := range user.Roles() {
			roles = append(roles, role.Name())
		}
		for _, permission := range user.Permissions() {
			permissions = append(permissions, permission.Name())
		}
	}

	intent := spotlight.SearchIntentMixed
	if spotlight.IsHowQuery(q) {
		intent = spotlight.SearchIntentHelp
	}

	return spotlight.SearchRequest{
		Query:       q,
		TenantID:    tenantID,
		UserID:      userID,
		Roles:       roles,
		Permissions: permissions,
		TopK:        30,
		Intent:      intent,
	}, nil
}

func (c *SpotlightController) snapshotPayload(r *http.Request, snapshot spotlight.SearchSessionSnapshot) spotlightSearchPayload {
	return spotlightSearchPayload{
		SearchID: snapshot.ID,
		HTML:     c.renderSnapshotHTML(r, snapshot),
		Loading:  snapshot.Loading,
		Complete: snapshot.Completed,
		Pending:  snapshot.PendingCount(),
		Stages:   append([]spotlight.SearchStageState(nil), snapshot.Stages...),
	}
}

func (c *SpotlightController) renderSnapshotHTML(r *http.Request, snapshot spotlight.SearchSessionSnapshot) string {
	view := spotlight.ToViewResponse(snapshot.Response)
	items := make([]templ.Component, 0, 64)
	index := 0

	appendGroup := func(title string, hits []spotlight.SearchHit) {
		if len(hits) == 0 {
			return
		}
		groupItems := make([]templ.Component, 0, len(hits))
		for _, hit := range hits {
			groupItems = append(groupItems, spotlight.HitToComponent(hit, snapshot.Query))
		}
		items = append(items, spotlight.GroupToComponent(title, groupItems, index))
		index += len(groupItems)
	}

	for _, group := range view.Groups {
		localizedTitle := group.Title
		if group.TitleKey != "" {
			localizedTitle = intl.MustT(r.Context(), group.TitleKey)
		}
		appendGroup(localizedTitle, group.Hits)
	}

	if len(items) == 0 {
		if snapshot.Completed {
			return renderComponent(r, spotlightui.NotFound())
		}
		return ""
	}
	return renderComponent(r, spotlightui.SpotlightResults(items))
}

func renderComponent(r *http.Request, component templ.Component) string {
	var buffer bytes.Buffer
	if err := component.Render(r.Context(), &buffer); err != nil {
		composables.UseLogger(r.Context()).WithError(err).Error("spotlight component render failed")
		return ""
	}
	return buffer.String()
}

func writeSSE(w http.ResponseWriter, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	return nil
}
