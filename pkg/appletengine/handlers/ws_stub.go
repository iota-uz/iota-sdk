package handlers

import (
	"context"
	"fmt"

	"github.com/iota-uz/applets"
	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
)

type WSBroadcaster interface {
	Send(appletID, connectionID string, payload any) error
}

type WSStub struct {
	broadcaster WSBroadcaster
}

func NewWSStub(broadcaster WSBroadcaster) *WSStub {
	return &WSStub{broadcaster: broadcaster}
}

func (s *WSStub) Register(registry *appletenginerpc.Registry, appletName string) error {
	if s.broadcaster == nil {
		return fmt.Errorf("ws broadcaster is required")
	}
	router := applets.NewTypedRPCRouter()
	if err := applets.AddProcedure(router, fmt.Sprintf("%s.ws.send", appletName), applets.Procedure[any, any]{
		Handler: func(ctx context.Context, params any) (any, error) {
			payload, ok := params.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid params: %w", applets.ErrInvalid)
			}
			connectionID, _ := payload["connectionId"].(string)
			if connectionID == "" {
				return nil, fmt.Errorf("connectionId is required: %w", applets.ErrInvalid)
			}
			if err := s.broadcaster.Send(appletName, connectionID, payload["data"]); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, nil
		},
	}); err != nil {
		return err
	}
	for methodName, method := range router.Config().Methods {
		if err := registry.RegisterServerOnly(appletName, methodName, method, nil); err != nil {
			return err
		}
	}
	return nil
}
