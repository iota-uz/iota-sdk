package telegram

import (
	"context"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type SendMessageOpts gotgbot.SendMessageOpts

type Bot struct {
	client *gotgbot.Bot
}

func NewBot(token string) (*Bot, error) {
	client, err := gotgbot.NewBot(token, &gotgbot.BotOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}
	return &Bot{client: client}, nil
}

func (b *Bot) SendMessage(ctx context.Context, chatID int64, text string, options *SendMessageOpts) error {
	_, err := b.client.SendMessage(chatID, text, (*gotgbot.SendMessageOpts)(options))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return nil
}
