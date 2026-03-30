// Package controllers provides Granite HTTP handlers for the core module.
package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	spotlightui "github.com/iota-uz/iota-sdk/components/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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

type spotlightSearchQuery struct {
	Q string `form:"q"`
}

type spotlightSessionQuery struct {
	SearchID string `form:"search_id"`
}

type spotlightAISessionQuery struct {
	SessionID string `form:"session_id"`
	RunID     string `form:"run_id"`
}

type spotlightAICreateRequest struct {
	Q string `json:"q"`
}

type spotlightAIMessageRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
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
	router.HandleFunc("/ai/sessions", c.CreateAISession).Methods(http.MethodPost)
	router.HandleFunc("/ai/stream", c.StreamAISession).Methods(http.MethodGet)
	router.HandleFunc("/ai/messages", c.SendAIMessage).Methods(http.MethodPost)
	router.HandleFunc("/ai/cancel", c.CancelAISession).Methods(http.MethodPost)
}

func (c *SpotlightController) Search(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")

	queryDTO, err := composables.UseQuery(&spotlightSearchQuery{}, r)
	if err != nil {
		http.Error(w, "Failed to parse Spotlight query", http.StatusBadRequest)
		return
	}
	q := strings.TrimSpace(queryDTO.Q)
	if q == "" {
		if err := json.NewEncoder(w).Encode(spotlightSearchPayload{
			SearchID: "",
			HTML:     "",
			Loading:  false,
			Complete: true,
			Pending:  0,
		}); err != nil {
			composables.UseLogger(r.Context()).WithError(err).Warn("spotlight empty search payload encode failed")
		}
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

	payload, err := c.snapshotPayload(r, snapshot)
	if err != nil {
		composables.UseLogger(r.Context()).WithError(err).WithField("query", q).Error("spotlight search payload render failed")
		http.Error(w, "Failed to render Spotlight results", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		composables.UseLogger(r.Context()).WithError(err).WithField("query", q).Warn("spotlight search payload encode failed")
	}
}

func (c *SpotlightController) Stream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	queryDTO, err := composables.UseQuery(&spotlightSessionQuery{}, r)
	if err != nil {
		http.Error(w, "Failed to parse search session", http.StatusBadRequest)
		return
	}
	sessionID := strings.TrimSpace(queryDTO.SearchID)
	if sessionID == "" {
		http.Error(w, "search_id is required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	updates, err := c.app.Spotlight().SubscribeSession(r.Context(), sessionID, c.sessionAccess(r))
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
			payload, err := c.snapshotPayload(r, snapshot)
			if err != nil {
				composables.UseLogger(r.Context()).WithError(err).WithField("search_id", sessionID).Error("spotlight stream payload render failed")
				return
			}
			if err := writeSSE(w, "update", payload); err != nil {
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
	queryDTO, err := composables.UseQuery(&spotlightSessionQuery{}, r)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sessionID := strings.TrimSpace(queryDTO.SearchID)
	if sessionID == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	c.app.Spotlight().CancelSession(sessionID, c.sessionAccess(r))
	w.WriteHeader(http.StatusNoContent)
}

func (c *SpotlightController) CreateAISession(w http.ResponseWriter, r *http.Request) {
	service := spotlight.ResolveAISearchService(c.app.Services())
	if service == nil {
		http.Error(w, "AI Spotlight unavailable", http.StatusNotFound)
		return
	}

	var reqBody spotlightAICreateRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Failed to parse AI Spotlight request", http.StatusBadRequest)
		return
	}

	query := strings.TrimSpace(reqBody.Q)
	if query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	snapshot, err := service.CreateSession(r.Context(), spotlight.AISearchCreateRequest{
		Query: query,
		Actor: c.aiActor(r),
	})
	if err != nil {
		composables.UseLogger(r.Context()).WithError(err).WithField("query", query).Error("spotlight ai session creation failed")
		http.Error(w, "Failed to start AI search", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snapshot)
}

func (c *SpotlightController) StreamAISession(w http.ResponseWriter, r *http.Request) {
	service := spotlight.ResolveAISearchService(c.app.Services())
	if service == nil {
		http.Error(w, "AI Spotlight unavailable", http.StatusNotFound)
		return
	}

	queryDTO, err := composables.UseQuery(&spotlightAISessionQuery{}, r)
	if err != nil {
		http.Error(w, "Failed to parse AI Spotlight session", http.StatusBadRequest)
		return
	}
	sessionID := strings.TrimSpace(queryDTO.SessionID)
	runID := strings.TrimSpace(queryDTO.RunID)
	if sessionID == "" || runID == "" {
		http.Error(w, "session_id and run_id are required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	updates, err := service.Subscribe(r.Context(), sessionID, runID, c.aiSessionAccess(r))
	if err != nil {
		http.Error(w, "AI Spotlight session not found", http.StatusNotFound)
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
			if err := writeSSE(w, "update", snapshot); err != nil {
				return
			}
			flusher.Flush()
			if snapshot.Completed && snapshot.RunID == runID {
				return
			}
		}
	}
}

func (c *SpotlightController) SendAIMessage(w http.ResponseWriter, r *http.Request) {
	service := spotlight.ResolveAISearchService(c.app.Services())
	if service == nil {
		http.Error(w, "AI Spotlight unavailable", http.StatusNotFound)
		return
	}

	var reqBody spotlightAIMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Failed to parse AI Spotlight message", http.StatusBadRequest)
		return
	}

	sessionID := strings.TrimSpace(reqBody.SessionID)
	message := strings.TrimSpace(reqBody.Message)
	if sessionID == "" || message == "" {
		http.Error(w, "session_id and message are required", http.StatusBadRequest)
		return
	}

	snapshot, err := service.SendMessage(r.Context(), spotlight.AISearchMessageRequest{
		SessionID: sessionID,
		Message:   message,
		Actor:     c.aiActor(r),
	})
	if err != nil {
		composables.UseLogger(r.Context()).WithError(err).WithField("session_id", sessionID).Error("spotlight ai message failed")
		http.Error(w, "Failed to continue AI search", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snapshot)
}

func (c *SpotlightController) CancelAISession(w http.ResponseWriter, r *http.Request) {
	service := spotlight.ResolveAISearchService(c.app.Services())
	if service == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	queryDTO, err := composables.UseQuery(&spotlightAISessionQuery{}, r)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	service.Cancel(strings.TrimSpace(queryDTO.SessionID), strings.TrimSpace(queryDTO.RunID), c.aiSessionAccess(r))
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

func (c *SpotlightController) sessionAccess(r *http.Request) spotlight.SearchSessionAccess {
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		return spotlight.SearchSessionAccess{}
	}

	userID := ""
	if user, userErr := composables.UseUser(r.Context()); userErr == nil {
		userID = fmt.Sprintf("%d", user.ID())
	}

	return spotlight.SearchSessionAccess{
		TenantID: tenantID,
		UserID:   userID,
	}
}

func (c *SpotlightController) aiSessionAccess(r *http.Request) spotlight.AISearchSessionAccess {
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		return spotlight.AISearchSessionAccess{}
	}

	userID := ""
	if user, userErr := composables.UseUser(r.Context()); userErr == nil {
		userID = fmt.Sprintf("%d", user.ID())
	}

	return spotlight.AISearchSessionAccess{
		TenantID: tenantID,
		UserID:   userID,
	}
}

func (c *SpotlightController) aiActor(r *http.Request) spotlight.AISearchActor {
	tenantID, _ := composables.UseTenantID(r.Context())
	actor := spotlight.AISearchActor{
		TenantID: tenantID,
	}
	if locale, ok := intl.UseLocale(r.Context()); ok {
		actor.Language = locale.String()
	}

	if user, err := composables.UseUser(r.Context()); err == nil {
		actor.UserID = fmt.Sprintf("%d", user.ID())
		for _, role := range user.Roles() {
			actor.Roles = append(actor.Roles, role.Name())
		}
		for _, permission := range user.Permissions() {
			actor.Permissions = append(actor.Permissions, permission.Name())
		}
	}

	return actor
}

func (c *SpotlightController) snapshotPayload(r *http.Request, snapshot spotlight.SearchSessionSnapshot) (spotlightSearchPayload, error) {
	html, err := c.renderSnapshotHTML(r, snapshot)
	if err != nil {
		return spotlightSearchPayload{}, err
	}
	return spotlightSearchPayload{
		SearchID: snapshot.ID,
		HTML:     html,
		Loading:  snapshot.Loading,
		Complete: snapshot.Completed,
		Pending:  snapshot.PendingCount(),
		Stages:   append([]spotlight.SearchStageState(nil), snapshot.Stages...),
	}, nil
}

func (c *SpotlightController) renderSnapshotHTML(r *http.Request, snapshot spotlight.SearchSessionSnapshot) (string, error) {
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

	for _, group := range snapshot.Response.Groups {
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
		return "", nil
	}
	return renderComponent(r, spotlightui.SpotlightResults(items))
}

func renderComponent(r *http.Request, component templ.Component) (string, error) {
	const op serrors.Op = "core.controllers.renderComponent"

	var buffer bytes.Buffer
	if err := component.Render(r.Context(), &buffer); err != nil {
		return "", serrors.E(op, err)
	}
	return buffer.String(), nil
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
