package controllers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	appletenginewsbridge "github.com/iota-uz/iota-sdk/pkg/appletengine/wsbridge"
	"github.com/sirupsen/logrus"
)

type WSController struct {
	bridge   *appletenginewsbridge.Bridge
	logger   *logrus.Logger
	upgrader websocket.Upgrader
}

func NewWSController(bridge *appletenginewsbridge.Bridge, logger *logrus.Logger) *WSController {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &WSController{
		bridge:   bridge,
		logger:   logger,
		upgrader: websocket.Upgrader{},
	}
}

func (c *WSController) Key() string {
	return "applet_ws"
}

func (c *WSController) Register(router *mux.Router) {
	router.HandleFunc("/applets/{applet}/ws", c.handleBrowserWS).Methods(http.MethodGet)
}

func (c *WSController) handleBrowserWS(w http.ResponseWriter, r *http.Request) {
	if c.bridge == nil {
		http.Error(w, "ws bridge unavailable", http.StatusServiceUnavailable)
		return
	}
	appletID := strings.TrimSpace(mux.Vars(r)["applet"])
	if appletID == "" {
		http.Error(w, "missing applet", http.StatusBadRequest)
		return
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-Iota-Tenant-Id"))
	if tenantID == "" {
		http.Error(w, "missing X-Iota-Tenant-Id header", http.StatusBadRequest)
		return
	}

	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		c.logger.WithError(err).Warn("failed to upgrade applet websocket")
		return
	}

	connectionID := c.bridge.AddConnection(r.Context(), appletID, tenantID, conn)
	c.logger.WithField("applet", appletID).WithField("connection_id", connectionID).Info("applet websocket connected")

	defer func() {
		c.bridge.RemoveConnection(context.Background(), connectionID)
		_ = conn.Close()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		c.bridge.DispatchMessage(r.Context(), connectionID, message)
	}
}
