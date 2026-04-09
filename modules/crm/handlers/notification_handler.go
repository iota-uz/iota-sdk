// Package handlers provides this package.
package handlers

import (
	"context"
	"fmt"
	"log"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/telegram"
)

// NotificationHandler subscribes to new-message events and forwards them
// to a Telegram bot. The Telegram bot is constructed lazily by the caller;
// use NewNotificationHandler to build one from a bot token and then
// register via composition.ContributeHooks.
type NotificationHandler struct {
	tgBot *telegram.Bot
}

func NewNotificationHandler(botToken string) (*NotificationHandler, error) {
	if botToken == "" {
		return nil, fmt.Errorf("crm: notification handler requires a telegram bot token")
	}
	bot, err := telegram.NewBot(botToken)
	if err != nil {
		return nil, fmt.Errorf("crm: failed to create telegram bot: %w", err)
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
