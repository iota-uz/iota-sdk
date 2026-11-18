package controllers_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/webpush"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type pushDevices struct {
	notifications.PushDeviceRepository
	registered []notifications.PushDevice
	revoked    []string
}

func (p *pushDevices) Register(_ context.Context, d notifications.PushDevice) error {
	p.registered = append(p.registered, d)
	return nil
}

func (p *pushDevices) Revoke(_ context.Context, userID uint, deviceID string) error {
	p.revoked = append(p.revoked, deviceID)
	return nil
}

func pushFixture(t *testing.T) (*controllers.NotificationPushController, *services.NotificationChannelService, *pushDevices, webpush.Subscription) {
	t.Helper()
	keys, err := webpush.GenerateKeys()
	require.NoError(t, err)
	adapter, err := webpush.New(webpush.Options{PublicKey: keys.Public, PrivateKey: keys.Private, Subject: "mailto:ops@example.com"})
	require.NoError(t, err)
	devices := &pushDevices{}
	service := services.NewNotificationChannelService(nil, nil, nil, nil, nil).WithPlatform(&services.NotificationPlatform{Devices: devices})
	ua, err := ecdh.P256().GenerateKey(rand.Reader)
	require.NoError(t, err)
	auth := make([]byte, 16)
	_, err = rand.Read(auth)
	require.NoError(t, err)
	sub := webpush.Subscription{Endpoint: "https://fcm.googleapis.com/fcm/send/abc", Keys: webpush.SubscriptionKeys{
		P256DH: base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(auth),
	}}
	return controllers.NewNotificationPushController(adapter).(*controllers.NotificationPushController), service, devices, sub
}

func asUser(r *http.Request, id uint) *http.Request {
	return r.WithContext(composables.WithUser(r.Context(), user.New("Tester", "", nil, "en", user.WithID(id))))
}

func TestNotificationPush_RegistersTheSignedInUsersSubscription(t *testing.T) {
	c, service, devices, sub := pushFixture(t)
	body, err := json.Marshal(map[string]any{"subscription": sub})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c.RegisterDevice(asUser(httptest.NewRequest(http.MethodPut, "/notifications/push/devices?user_id=999", strings.NewReader(string(body))), 42), rec, service)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		DeviceID string `json:"deviceId"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, webpush.DeviceID(sub), out.DeviceID)
	require.Len(t, devices.registered, 1)
	got := devices.registered[0]
	require.Equal(t, uint(42), got.UserID, "the recipient comes from the session, never from the request")
	require.Equal(t, notifications.PushWeb, got.Platform)
	token, err := sub.Token()
	require.NoError(t, err)
	require.Equal(t, token, got.Token)

	// refused: another host, a body with extras, a missing user, junk
	other := sub
	other.Endpoint = "https://evil.example.com/x"
	for name, req := range map[string]*http.Request{
		"foreign endpoint": asUser(httptest.NewRequest(http.MethodPut, "/d", strings.NewReader(mustJSON(t, map[string]any{"subscription": other}))), 42),
		"unknown field":    asUser(httptest.NewRequest(http.MethodPut, "/d", strings.NewReader(mustJSON(t, map[string]any{"subscription": sub, "userId": 7}))), 42),
		"junk":             asUser(httptest.NewRequest(http.MethodPut, "/d", strings.NewReader("{")), 42),
	} {
		rec = httptest.NewRecorder()
		c.RegisterDevice(req, rec, service)
		require.Equal(t, http.StatusBadRequest, rec.Code, name)
	}
	rec = httptest.NewRecorder()
	c.RegisterDevice(httptest.NewRequest(http.MethodPut, "/d", strings.NewReader(string(body))), rec, service)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Len(t, devices.registered, 1, "nothing refused was stored")
}

func TestNotificationPush_ConfigAndRevoke(t *testing.T) {
	c, service, devices, _ := pushFixture(t)
	rec := httptest.NewRecorder()
	c.Config(httptest.NewRequest(http.MethodGet, "/notifications/push/config", nil), rec)
	require.Equal(t, http.StatusOK, rec.Code)
	var cfg struct {
		VAPIDPublicKey string `json:"vapidPublicKey"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cfg))
	require.NotEmpty(t, cfg.VAPIDPublicKey)

	req := mux.SetURLVars(asUser(httptest.NewRequest(http.MethodDelete, "/notifications/push/devices/web:abc", nil), 42), map[string]string{"deviceId": "web:abc"})
	rec = httptest.NewRecorder()
	c.Revoke(req, rec, service)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, []string{"web:abc"}, devices.revoked)

	rec = httptest.NewRecorder()
	c.Revoke(mux.SetURLVars(httptest.NewRequest(http.MethodDelete, "/x", nil), map[string]string{"deviceId": "web:abc"}), rec, service)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}
