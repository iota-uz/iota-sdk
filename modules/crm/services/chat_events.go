package services

import (
	"context"
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ChatMessageAddedEventID is the versioned ID of the event published for every
// message added to a chat.
const ChatMessageAddedEventID = "crm.chat.message_added.v1"

// ChatEntityID is the ID of the crm.chat native entity.
const ChatEntityID = "crm.chat"

// ChatMessageBodyLimit is the number of characters of a message text a
// crm.chat.message_added.v1 payload carries; the rest is cut off.
const ChatMessageBodyLimit = 4000

// ChatEntity describes modules/crm `chats` (key: bigint) for the extension
// runtime.
func ChatEntity() data.NativeEntity {
	return data.NativeEntity{
		ID: ChatEntityID, Table: "chats", KeyColumn: "id", KeyType: data.KeyInt64, TenantColumn: "tenant_id",
		ReadPermission: permissions.ClientRead.Name(),
		Label:          localized("Chat", "Чат", "Chat", "Чат"),
		Fields: []data.NativeField{
			{ID: "clientId", Column: "client_id", Type: manifest.Type{Kind: manifest.KindInt64}},
			{ID: "lastMessageAt", Column: "last_message_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
			{ID: "createdAt", Column: "created_at", Type: manifest.Type{Kind: manifest.KindDateTime}},
		},
	}
}

// ChatMessageAddedPayload is the payload of crm.chat.message_added.v1. The
// text is cut to ChatMessageBodyLimit characters and attachments are not
// carried.
type ChatMessageAddedPayload struct {
	ChatID    int64     `json:"chatId,string" wire:"int64"`
	ClientID  int64     `json:"clientId,string" wire:"int64"`
	MessageID int64     `json:"messageId,string" wire:"int64"`
	Transport string    `json:"transport" wire:"enum=telegram|whatsapp|instagram|sms|email|phone|website|other"`
	Direction string    `json:"direction" wire:"enum=inbound|outbound"`
	Body      string    `json:"body" wire:"string,maxLength=4000"`
	SentAt    time.Time `json:"sentAt"`
}

// Directions of a chat message.
const (
	ChatDirectionInbound  = "inbound"
	ChatDirectionOutbound = "outbound"
)

// ChatMessageAddedEvent is published in the transaction that adds a message to
// a chat, whether the client wrote it or a user sent it.
var ChatMessageAddedEvent = native.Event[ChatMessageAddedPayload](ChatMessageAddedEventID, native.EventSpec{
	Subject:     ChatEntityID,
	Label:       localized("Chat message added", "Сообщение добавлено в чат", "Chatga xabar qo'shildi", "Чатга хабар қўшилди"),
	Description: "A message was added to a CRM chat. Published once per message in the transaction that stores it; inbound when the client wrote it, outbound when a user sent it.",
	Descriptions: localized(
		"A message was added to a CRM chat.",
		"В чат CRM добавлено сообщение.",
		"CRM chatiga xabar qo'shildi.",
		"CRM чатига хабар қўшилди."),
	ReadPermission: permissions.ClientRead.Name(),
	Sensitive:      map[string][]string{"body": {permissions.ClientRead.Name()}},
	Category:       "crm.chats",
})

// chatTransport maps a transport to the payload enum; one the contract does
// not list is "other", so the payload is always valid.
func chatTransport(t chat.Transport) string {
	switch t {
	case chat.TelegramTransport, chat.WhatsAppTransport, chat.InstagramTransport, chat.SMSTransport,
		chat.EmailTransport, chat.PhoneTransport, chat.WebsiteTransport, chat.OtherTransport:
		return string(t)
	}
	return string(chat.OtherTransport)
}

// chatDirection is inbound for a message of a client (or of a sender the CRM
// does not know) and outbound for a message of a user.
func chatDirection(m chat.Member) string {
	if m != nil && m.Sender() != nil && m.Sender().Type() == chat.UserSenderType {
		return ChatDirectionOutbound
	}
	return ChatDirectionInbound
}

// truncateBody cuts text to limit characters.
func truncateBody(text string, limit int) string {
	if r := []rune(text); len(r) > limit {
		return string(r[:limit])
	}
	return text
}

func chatMessagePayload(c chat.Chat, m chat.Message) ChatMessageAddedPayload {
	sentAt := m.CreatedAt()
	if m.SentAt() != nil {
		sentAt = *m.SentAt()
	}
	p := ChatMessageAddedPayload{
		ChatID: int64(c.ID()), ClientID: int64(c.ClientID()), MessageID: int64(m.ID()),
		Transport: string(chat.OtherTransport), Direction: chatDirection(m.Sender()),
		Body: truncateBody(m.Message(), ChatMessageBodyLimit), SentAt: sentAt,
	}
	if m.Sender() != nil {
		p.Transport = chatTransport(m.Sender().Transport())
	}
	return p
}

// publishChatMessagesAdded enqueues crm.chat.message_added.v1 for every
// message of after that before did not have, in the transaction held by ctx.
// before is the chat before the change.
func publishChatMessagesAdded(ctx context.Context, before, after chat.Chat) error {
	const op = serrors.Op("crm.publishChatMessagesAdded")
	known := make(map[uint]bool, len(before.Messages()))
	for _, m := range before.Messages() {
		known[m.ID()] = true
	}
	ref := &spi.ObjectRef{Entity: ChatEntityID, Key: strconv.FormatUint(uint64(after.ID()), 10)}
	for _, m := range after.Messages() {
		if known[m.ID()] {
			continue
		}
		if err := events.PublishInTx(ctx, after.TenantID(), ChatMessageAddedEvent, ref, chatMessagePayload(after, m)); err != nil {
			return serrors.Wrap(op, err)
		}
	}
	return nil
}
