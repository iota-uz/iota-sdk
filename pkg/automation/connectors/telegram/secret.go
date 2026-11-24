package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
)

// secretLen is the length of a webhook secret: 32 characters of [A-Za-z0-9_-], what the Bot API takes.
const secretLen = 32

// WebhookSecret is the secret Telegram sends back in X-Telegram-Bot-Api-Secret-Token for a connection:
// derived from its bot token and its identity (a connection UUID or "platform:telegram_bot") with an
// HMAC, so Verify needs no stored copy and a replaced token replaces the secret. It is never returned to a
// client or logged.
func WebhookSecret(token, connection string) string {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte("iota-sdk.telegram.webhook.v1:" + connection))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))[:secretLen]
}

// secretEqual compares in constant time.
func secretEqual(got, want string) bool {
	return len(got) == len(want) && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// botID is the numeric id every bot token starts with ("<id>:<secret>"), 0 when the token has another shape.
func botID(token string) int64 {
	id, _, ok := strings.Cut(token, ":")
	if !ok {
		return 0
	}
	var n int64
	for _, r := range id {
		if r < '0' || r > '9' || n > 1<<50 {
			return 0
		}
		n = n*10 + int64(r-'0')
	}
	return n
}
