package share

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const maxRequestBytes = 32 << 10

type AccessResolver func(*http.Request) (Access, error)

type HTTPHandler struct {
	store                    Store
	access                   AccessResolver
	baseURL                  string
	scheduledDeliveryEnabled bool
}

type HTTPOption func(*HTTPHandler)

// WithScheduledDelivery enables the schedule API capability. Hosts must only
// set it after registering a working WorkbookExporter, Mailer, and periodic
// ScheduleRunner; RBAC alone must not advertise an inert feature.
func WithScheduledDelivery() HTTPOption {
	return func(handler *HTTPHandler) { handler.scheduledDeliveryEnabled = true }
}

// NewHTTPHandler constructs the saved-view and scheduled-delivery API.
//
// Register mounts state-changing POST and DELETE routes. The host must mount
// the returned routes below its CSRF middleware; authentication and RBAC from
// AccessResolver do not replace CSRF verification.
func NewHTTPHandler(store Store, access AccessResolver, baseURL string, options ...HTTPOption) (*HTTPHandler, error) {
	if store == nil || access == nil {
		return nil, fmt.Errorf("store and access resolver are required")
	}
	baseURL = "/" + strings.Trim(strings.TrimSpace(baseURL), "/")
	if baseURL == "/" {
		baseURL = "/lens/share"
	}
	handler := &HTTPHandler{store: store, access: access, baseURL: baseURL}
	for _, option := range options {
		if option != nil {
			option(handler)
		}
	}
	return handler, nil
}

func (h *HTTPHandler) ViewsEndpoint() string     { return h.baseURL + "/views" }
func (h *HTTPHandler) SchedulesEndpoint() string { return h.baseURL + "/schedules" }

func (h *HTTPHandler) Register(router *mux.Router) {
	router.HandleFunc(h.ViewsEndpoint(), h.views)
	router.HandleFunc(h.ViewsEndpoint()+"/{id}", h.view)
	router.HandleFunc(h.SchedulesEndpoint(), h.schedules)
	router.HandleFunc(h.SchedulesEndpoint()+"/{id}", h.schedule)
}

func (h *HTTPHandler) views(w http.ResponseWriter, r *http.Request) {
	access, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		dashboardID, ok := readDashboardID(w, r)
		if !ok {
			return
		}
		views, err := h.store.ListViews(r.Context(), access, dashboardID)
		if err != nil {
			h.writeStoreError(w, err)
			return
		}
		writeHTTPJSON(w, http.StatusOK, map[string]any{
			"views":       views,
			"defaultView": defaultViewForRoles(views, access.RoleIDs),
			"capabilities": map[string]any{
				"manageTeam": access.ManageTeam, "scheduleMail": access.ScheduleMail, "roles": access.Roles,
			},
		})
	case http.MethodPost:
		var view SavedView
		if err := decodeHTTPJSON(w, r, &view); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		stored, err := h.store.PutView(r.Context(), access, view)
		if err != nil {
			h.writeStoreError(w, err)
			return
		}
		writeHTTPJSON(w, http.StatusOK, stored)
	default:
		writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func defaultViewForRoles(views []SavedView, roleIDs []uint) *SavedView {
	roles := append([]uint(nil), roleIDs...)
	slices.Sort(roles)
	for _, roleID := range roles {
		for index := range views {
			if views[index].DefaultRoleID != nil && *views[index].DefaultRoleID == roleID {
				matched := views[index]
				return &matched
			}
		}
	}
	return nil
}

func (h *HTTPHandler) view(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	access, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "invalid view id")
		return
	}
	if err := h.store.DeleteView(r.Context(), access, id); err != nil {
		h.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) schedules(w http.ResponseWriter, r *http.Request) {
	access, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		dashboardID, ok := readDashboardID(w, r)
		if !ok {
			return
		}
		schedules, err := h.store.ListSchedules(r.Context(), access, dashboardID)
		if err != nil {
			h.writeStoreError(w, err)
			return
		}
		writeHTTPJSON(w, http.StatusOK, map[string]any{"schedules": schedules})
	case http.MethodPost:
		var schedule ExportSchedule
		if err := decodeHTTPJSON(w, r, &schedule); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
			return
		}
		stored, err := h.store.PutSchedule(r.Context(), access, schedule)
		if err != nil {
			h.writeStoreError(w, err)
			return
		}
		writeHTTPJSON(w, http.StatusOK, stored)
	default:
		writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func readDashboardID(w http.ResponseWriter, r *http.Request) (string, bool) {
	dashboardID := strings.TrimSpace(r.URL.Query().Get("dashboard"))
	if dashboardID == "" {
		writeHTTPError(w, http.StatusBadRequest, "dashboard is required")
		return "", false
	}
	if len(dashboardID) > 120 {
		writeHTTPError(w, http.StatusBadRequest, "dashboard cannot exceed 120 characters")
		return "", false
	}
	return dashboardID, true
}

func (h *HTTPHandler) schedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	access, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}
	if err := h.store.DeleteSchedule(r.Context(), access, id); err != nil {
		h.writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (Access, bool) {
	access, err := h.access(r)
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			writeHTTPError(w, http.StatusUnauthorized, "authentication required")
		} else {
			writeHTTPError(w, http.StatusInternalServerError, "internal server error")
		}
		return Access{}, false
	}
	if access.TenantID == uuid.Nil || access.UserID == 0 {
		writeHTTPError(w, http.StatusUnauthorized, "authentication required")
		return Access{}, false
	}
	access.ScheduleMail = access.ScheduleMail && h.scheduledDeliveryEnabled
	return access, true
}

func (h *HTTPHandler) writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeHTTPError(w, http.StatusNotFound, "record not found")
	case errors.Is(err, ErrForbidden):
		writeHTTPError(w, http.StatusForbidden, "operation forbidden")
	case errors.Is(err, ErrInvalid):
		message := strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": ")
		writeHTTPError(w, http.StatusBadRequest, message)
	default:
		writeHTTPError(w, http.StatusInternalServerError, "internal server error")
	}
}

func decodeHTTPJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request must contain one JSON value")
	}
	return nil
}

func writeHTTPJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func writeHTTPError(w http.ResponseWriter, status int, message string) {
	writeHTTPJSON(w, status, map[string]string{"error": message})
}
