package handlers

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/telegram"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

type NotificationHandler struct {
	pool  *pgxpool.Pool
	tgBot *telegram.Bot
}

func RegisterNotificationHandler(app application.Application, botToken string) *NotificationHandler {
	bot, err := telegram.NewBot(botToken)
	if err != nil {
		log.Fatalf("Error creating telegram bot: %v", err)
	}
	handler := &NotificationHandler{
		pool:  app.DB(),
		tgBot: bot,
	}
	app.EventPublisher().Subscribe(handler.onNewMessage)
	return handler
}

func (h *NotificationHandler) onNewMessage(event *chat.MessagedAddedEvent) {
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
