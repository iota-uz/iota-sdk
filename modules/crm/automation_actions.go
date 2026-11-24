package crm

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	crmPermissions "github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions/servicectx"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// ActionChatSendMessage is the key of the automation action that sends a message to a client's chat.
const ActionChatSendMessage = "crm.chat.send_message"

// contributeAutomationActions offers the chat operation of the CRM services to automation graphs.
func contributeAutomationActions(b *composition.Builder) {
	actions.ContributeFunc(b, func(c *composition.Container) ([]actions.Action, error) {
		pool, err := composition.Resolve[*pgxpool.Pool](c)
		if err != nil {
			return nil, err
		}
		users, err := composition.Resolve[user.Repository](c)
		if err != nil {
			return nil, err
		}
		chats, err := composition.Resolve[*services.ChatService](c)
		if err != nil {
			return nil, err
		}
		return []actions.Action{chatSendAction{pool: pool, users: users, chats: chats}}, nil
	})
}

type chatSendAction struct {
	pool  *pgxpool.Pool
	users user.Repository
	chats *services.ChatService
}

var chatTransports = []string{
	string(chat.SMSTransport), string(chat.TelegramTransport), string(chat.WhatsAppTransport), string(chat.InstagramTransport),
	string(chat.EmailTransport), string(chat.PhoneTransport), string(chat.WebsiteTransport), string(chat.OtherTransport),
}

func (chatSendAction) Key() string { return ActionChatSendMessage }
func (chatSendAction) Spec() actions.ActionSpec {
	return actions.ActionSpec{
		Label: manifest.Label{"en": "Send chat message", "ru": "Отправить сообщение в чат", "uz": "Chatga xabar yuborish", "uz-Cyrl": "Чатга хабар юбориш"},
		Description: manifest.Label{
			"en": "Sends a message to a client's chat by a transport (SMS by default).", "ru": "Отправляет сообщение в чат клиента через транспорт (по умолчанию SMS).",
			"uz": "Mijoz chatiga transport orqali xabar yuboradi (standart SMS).", "uz-Cyrl": "Мижоз чатига транспорт орқали хабар юборади (стандарт SMS).",
		},
		Group: "crm",
		Args: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			{Name: "chat_id", Type: manifest.Type{Kind: manifest.KindInt}},
			{Name: "message", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 4000}},
			{Name: "transport", Type: manifest.Type{Kind: manifest.KindEnum, Values: chatTransports, Optional: true}},
		}},
		Output: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			{Name: "chat_id", Type: manifest.Type{Kind: manifest.KindInt}},
			{Name: "message_id", Type: manifest.Type{Kind: manifest.KindInt}},
		}},
		Permission: crmPermissions.ClientUpdate.Name(),
	}
}

func (a chatSendAction) Run(ctx context.Context, actor spi.Subject, args json.RawMessage) (string, json.RawMessage, error) {
	var in struct {
		ChatID    uint   `json:"chat_id"`
		Message   string `json:"message"`
		Transport string `json:"transport"`
	}
	if err := json.Unmarshal(args, &in); err != nil || in.Message == "" {
		return "", nil, actions.ErrInvalidArgs
	}
	if in.Transport == "" {
		in.Transport = string(chat.SMSTransport)
	}
	sctx, err := servicectx.Context(ctx, a.pool, a.users, actor)
	if err != nil {
		return "", nil, err
	}
	updated, err := a.chats.SendMessage(sctx, services.SendMessageCommand{ChatID: in.ChatID, Transport: chat.Transport(in.Transport), Message: in.Message})
	if errors.Is(err, services.ErrNoChatProvider) {
		return "", nil, &actions.ActionError{Code: "transport_unavailable", Message: "no provider sends messages by " + in.Transport}
	}
	if err != nil {
		return "", nil, servicectx.Refusal(err)
	}
	var last uint
	if msgs := updated.Messages(); len(msgs) > 0 {
		last = msgs[len(msgs)-1].ID()
	}
	out, err := json.Marshal(map[string]any{"chat_id": updated.ID(), "message_id": last})
	return actions.OutcomeDone, out, err
}
