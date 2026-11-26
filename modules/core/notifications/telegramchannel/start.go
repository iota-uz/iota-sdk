package telegramchannel

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
)

// StartHandler completes a Telegram link on `/start u_<token>`.
type StartHandler struct {
	links notifications.ChannelLinkService
	// language answers a token whose user is unknown.
	language string
}

var (
	_ telegram.StartHandler     = (*StartHandler)(nil)
	_ telegram.StartTenantByKey = (*StartHandler)(nil)
)

// NewTelegramStartHandler is the telegram.StartHandler of the Telegram
// channel: Prefix "u_", TenantOf and Handle through the link service. Handle
// answers «Telegram connected» or «The link has expired»; a /start from a
// group is ignored.
func NewTelegramStartHandler(links notifications.ChannelLinkService) *StartHandler {
	return &StartHandler{links: links, language: "en"}
}

// WithLanguage sets the language of the answer to an unknown token.
func (h *StartHandler) WithLanguage(language string) *StartHandler {
	if language != "" {
		h.language = language
	}
	return h
}

// Prefix implements telegram.StartHandler.
func (h *StartHandler) Prefix() string { return Prefix }

func token(payload string) string { return strings.TrimPrefix(payload, Prefix) }

// TenantOf implements telegram.StartHandler.
func (h *StartHandler) TenantOf(ctx context.Context, payload string) (uuid.UUID, bool, error) {
	return h.links.TenantOf(ctx, token(payload))
}

// TenantOfUpdate implements telegram.StartTenantByKey: the tenant of the token
// an update used, so the answer to its redelivery is counted for that tenant.
func (h *StartHandler) TenantOfUpdate(ctx context.Context, updateKey string) (uuid.UUID, bool, error) {
	if linker, ok := h.links.(notifications.UpdateLinker); ok {
		return linker.TenantOfUpdate(ctx, updateKey)
	}
	return uuid.Nil, false, nil
}

// Handle implements telegram.StartHandler.
func (h *StartHandler) Handle(ctx context.Context, u telegram.StartUpdate) (string, error) {
	if u.ChatType != "private" || u.ChatID == 0 {
		return "", nil
	}
	tok := token(u.Payload)
	linker, keyed := h.links.(notifications.UpdateLinker)
	language := h.language
	if keyed {
		if l := linker.LanguageOf(ctx, tok); l != "" {
			language = l
		}
	}
	display := DisplayName(u.Username, u.FirstName)
	address := strconv.FormatInt(u.ChatID, 10)
	var err error
	switch {
	case keyed:
		// An update whose payload resolved to no tenant may still be the
		// redelivery of the update that used it: CompleteLinkUpdate says so.
		_, err = linker.CompleteLinkUpdate(ctx, u.ConnectionID, tok, address, display, u.UpdateKey)
	case u.TenantID == uuid.Nil:
		err = notifications.ErrLinkTokenInvalid
	default:
		err = h.links.CompleteLink(ctx, u.ConnectionID, tok, address, display)
	}
	switch {
	case errors.Is(err, notifications.ErrLinkTokenInvalid):
		return phrase(language, phraseExpired), nil
	case err != nil:
		return "", err
	}
	return phrase(language, phraseConnected), nil
}

// DisplayName names a Telegram account for people: @username, else the first name.
func DisplayName(username, firstName string) string {
	if username = strings.TrimSpace(username); username != "" {
		return "@" + username
	}
	return strings.TrimSpace(firstName)
}

type phraseKey int

const (
	phraseConnected phraseKey = iota
	phraseExpired
	phraseOpen
)

var phrases = map[phraseKey]map[string]string{
	phraseConnected: {
		"en":      "Telegram is connected. Notifications will arrive in this chat.",
		"ru":      "Telegram подключён. Уведомления будут приходить в этот чат.",
		"uz":      "Telegram ulandi. Bildirishnomalar shu chatga keladi.",
		"uz-Cyrl": "Telegram уланди. Билдиришномалар шу чатга келади.",
	},
	phraseExpired: {
		"en":      "This link has expired. Get a new one in your profile.",
		"ru":      "Ссылка устарела. Получите новую в профиле.",
		"uz":      "Havola eskirgan. Profilingizda yangisini oling.",
		"uz-Cyrl": "Ҳавола эскирган. Профилингизда янгисини олинг.",
	},
	phraseOpen: {
		"en":      "Open",
		"ru":      "Открыть",
		"uz":      "Ochish",
		"uz-Cyrl": "Очиш",
	},
}

// phrase is a bot phrase in a language, falling back to English.
func phrase(language string, key phraseKey) string {
	set := phrases[key]
	if v, ok := set[language]; ok {
		return v
	}
	if base, _, ok := strings.Cut(language, "-"); ok {
		if v, ok := set[base]; ok {
			return v
		}
	}
	return set["en"]
}
