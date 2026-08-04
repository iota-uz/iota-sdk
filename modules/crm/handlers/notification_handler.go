// Package handlers provides this package.
package handlers

import (
	"context"
	"errors"
	"log"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/telegram"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// NotificationHandler subscribes to new-message events and forwards them
// to a Telegram bot. OnNewMessage is an event-bus subscriber callback —
// the CRM component wires it via composition.ContributeEventHandler (not
// ContributeHooks) so the subscription and its matching unsubscribe are
// managed by the engine's lifecycle.
type NotificationHandler struct {
	tgBot *telegram.Bot
}

func NewNotificationHandler(botToken string) (*NotificationHandler, error) {
	const op serrors.Op = "crm.handlers.NewNotificationHandler"
	if botToken == "" {
		return nil, serrors.E(op, errors.New("telegram bot token is required"))
	}
	bot, err := telegram.NewBot(botToken)
	if err != nil {
		return nil, serrors.E(op, err, "failed to create telegram bot")
	}
	return &NotificationHandler{tgBot: bot}, nil
}

// OnNewMessage is the eventbus subscriber callback. Compatible with
// eventbus.EventBus.Subscribe.
func (h *NotificationHandler) OnNewMessage(event *chat.MessagedAddedEvent) {
	ctx := context.Background()
	chatID := int64(-1001979082001)
	if err := h.tgBot.SendMessage(
		ctx,
		chatID,
		"Получено новое сообщение",
		nil,
	); err != nil {
		log.Printf("Error sending telegram message: %v", err)
	}
}
