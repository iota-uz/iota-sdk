package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/webpush"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// maxPushRegistrationBytes bounds a registration request body; a real
// PushSubscription is a few hundred bytes.
const maxPushRegistrationBytes = 8 << 10

// NotificationPushController is the user-facing API a browser's service worker
// registration talks to: it fetches the VAPID public key and registers or
// revokes the PushSubscription of the signed-in user. It exists only when the
// application enabled web push (core.ModuleOptions.NotificationWebPush).
//
//	GET    /notifications/push/config            -> {"vapidPublicKey": "..."}
//	PUT    /notifications/push/devices           {"subscription": {endpoint, keys: {p256dh, auth}}}  -> {"deviceId": "web:..."}
//	DELETE /notifications/push/devices/{deviceId}  -> 204
//
// The recipient is always the authenticated user; a device can only be revoked
// by the user it belongs to.
type NotificationPushController struct {
	webPush *webpush.Adapter
}

// NewNotificationPushController returns the controller for a configured adapter.
func NewNotificationPushController(adapter *webpush.Adapter) application.Controller {
	return &NotificationPushController{webPush: adapter}
}

func (c *NotificationPushController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("core.notifications.push", 0, application.Route("", "/notifications/push", application.Authenticated()))
}

func (c *NotificationPushController) Register(r *mux.Router) {
	router := r.PathPrefix("/notifications/push").Subrouter()
	router.Use(middleware.Authorize(), middleware.RequireAuthorization(), middleware.ProvideUser())
	router.HandleFunc("/config", di.H(c.Config)).Methods(http.MethodGet)
	router.HandleFunc("/devices", di.H(c.RegisterDevice)).Methods(http.MethodPut)
	router.HandleFunc("/devices/{deviceId}", di.H(c.Revoke)).Methods(http.MethodDelete)
}

type pushConfigResponse struct {
	VAPIDPublicKey string `json:"vapidPublicKey"`
}

type pushRegisterRequest struct {
	Subscription webpush.Subscription `json:"subscription"`
}

type pushRegisterResponse struct {
	DeviceID string `json:"deviceId"`
}

func writeJSON(w http.ResponseWriter, status int, body []byte, err error) {
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// Config returns the public key the page passes to pushManager.subscribe.
func (c *NotificationPushController) Config(_ *http.Request, w http.ResponseWriter) {
	body, err := json.Marshal(pushConfigResponse{VAPIDPublicKey: c.webPush.PublicKey()})
	writeJSON(w, http.StatusOK, body, err)
}

// RegisterDevice stores or refreshes the signed-in user's browser subscription.
// Registering the same subscription again returns the same device.
func (c *NotificationPushController) RegisterDevice(r *http.Request, w http.ResponseWriter, channels *services.NotificationChannelService) {
	u, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req pushRegisterRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPushRegistrationBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Invalid push subscription", http.StatusBadRequest)
		return
	}
	if err := c.webPush.ValidateSubscription(req.Subscription); err != nil {
		http.Error(w, "Invalid push subscription", http.StatusBadRequest)
		return
	}
	token, err := req.Subscription.Token()
	if err != nil {
		http.Error(w, "Invalid push subscription", http.StatusBadRequest)
		return
	}
	device := notifications.PushDevice{UserID: u.ID(), DeviceID: webpush.DeviceID(req.Subscription), Token: token, Platform: notifications.PushWeb}
	if err := channels.RegisterDevice(r.Context(), device); err != nil {
		http.Error(w, "Unable to register push device", http.StatusInternalServerError)
		return
	}
	body, err := json.Marshal(pushRegisterResponse{DeviceID: device.DeviceID})
	writeJSON(w, http.StatusOK, body, err)
}

// Revoke stops pushes to one of the signed-in user's devices.
func (c *NotificationPushController) Revoke(r *http.Request, w http.ResponseWriter, channels *services.NotificationChannelService) {
	u, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	deviceID := mux.Vars(r)["deviceId"]
	if deviceID == "" || len(deviceID) > 200 {
		http.Error(w, "Invalid device", http.StatusBadRequest)
		return
	}
	if err := channels.RevokeDevice(r.Context(), u.ID(), deviceID); err != nil {
		http.Error(w, "Unable to revoke push device", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
