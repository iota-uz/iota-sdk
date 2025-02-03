package cpassproviders

import (
	"context"
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// SendMessageDTO represents the data needed to send a message
type SendMessageDTO struct {
	Message  string
	To       string
	From     string
	MediaURL string
}

type ReceivedMessageEvent struct {
	From string `json:"From"`
	To   string `json:"To"`
	Body string `json:"Body"`
}

type Provider interface {
	SendMessage(ctx context.Context, dto SendMessageDTO) error
	WebhookHandler(evb eventbus.EventBus) http.HandlerFunc
}
