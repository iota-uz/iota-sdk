package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// Code operations of a bot connection (env.connections.<name>.sendMessage, .getMe). The bot token is part of
// Telegram's URLs, so the connection offers no HTTP methods and no fetch: code reaches the Bot API only
// through these operations, and never sees the token.
const (
	OpSendMessage = "sendMessage"
	OpGetMe       = "getMe"
)

var (
	_ connectors.CodeAccessor = (*Connector)(nil)
	_ connectors.CodeOperator = (*Connector)(nil)
)

// CodeAccess offers the operations sendMessage and getMe.
func (c *Connector) CodeAccess() connectors.CodeAccess {
	return connectors.CodeAccess{Operations: []connectors.CodeOperation{
		{
			Name:        OpSendMessage,
			Description: "Send a text message to a chat of the bot.",
			Params:      `{ chat_id: string | number; text: string; parse_mode?: "HTML" }`,
			Result:      `{ chat_id: number; message_id: number }`,
		},
		{
			Name:        OpGetMe,
			Description: "The bot's own account: its id, username and name.",
			Result:      `{ id: number; username: string; first_name: string; is_bot: boolean; can_join_groups?: boolean; can_read_all_group_messages?: boolean }`,
		},
	}}
}

// Authorize sets nothing: the token is in the Bot API's URL, which only this connector builds.
func (c *Connector) Authorize(context.Context, connectors.Credentials, http.Header) error { return nil }

// CallCodeOperation serves sendMessage and getMe with the bot of the connection's credentials.
func (c *Connector) CallCodeOperation(ctx context.Context, call connectors.CodeCall) (any, error) {
	token := call.Input.Credentials["token"]
	if token == "" {
		return nil, &connectors.CodeOperationError{Code: "provider_refused", Message: "the connection has no bot token"}
	}
	switch call.Operation {
	case OpSendMessage:
		return c.codeSend(ctx, call, token)
	case OpGetMe:
		return c.codeGetMe(ctx, call, token)
	}
	return nil, &connectors.CodeOperationError{Code: "unknown_operation", Message: "the connection has no operation " + call.Operation}
}

func (c *Connector) codeSend(ctx context.Context, call connectors.CodeCall, token string) (any, error) {
	var args struct {
		ChatID    any    `json:"chat_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
	}
	if err := decodeArgs(call.Args, &args); err != nil {
		return nil, err
	}
	chat, err := chatArg(args.ChatID)
	if err != nil {
		return nil, invalidArgs("chat_id: " + err.Error())
	}
	ref, sent, err := c.send(ctx, token, chat, outgoing{Text: args.Text, ParseMode: args.ParseMode})
	if err != nil {
		return nil, c.codeFailure(ctx, call.Input, err)
	}
	c.recordUsage(ctx, call.Input, sent)
	return map[string]any{"chat_id": ref.ChatID, "message_id": ref.MessageID}, nil
}

func (c *Connector) codeGetMe(ctx context.Context, call connectors.CodeCall, token string) (any, error) {
	if err := decodeArgs(call.Args, &struct{}{}); err != nil {
		return nil, err
	}
	me, err := c.bot(token).getMe(ctx)
	if err != nil {
		return nil, c.codeFailure(ctx, call.Input, err)
	}
	return map[string]any{
		"id": me.Id, "username": me.Username, "first_name": me.FirstName, "is_bot": me.IsBot,
		"can_join_groups": me.CanJoinGroups, "can_read_all_group_messages": me.CanReadAllGroupMessages,
	}, nil
}

// codeFailure turns a failed Bot API call into what code handles: a typed refusal, or (when the outcome is
// unknown, a cut connection or a 5xx) the error with a message that names no URL or token.
func (c *Connector) codeFailure(ctx context.Context, in catalog.CallInput, err error) error {
	err = callFailure(err)
	var wait *RetryAfterError
	var ce *catalog.CallError
	switch {
	case errors.As(err, &wait):
		return &connectors.CodeOperationError{Code: "rate_limited", Message: "Telegram limits the rate of requests; repeat in " + wait.After.String(), Retryable: true}
	case errors.As(err, &ce):
		switch ce.Code {
		case catalog.CallUnauthorized:
			c.needsReauth(ctx, in)
			return &connectors.CodeOperationError{Code: "provider_refused", Message: ce.Message}
		case catalog.CallInvalidRequest:
			return &connectors.CodeOperationError{Code: "invalid_args", Message: ce.Message}
		}
		return &connectors.CodeOperationError{Code: "provider_refused", Message: ce.Message}
	}
	return errors.New("telegram: the request failed and its outcome is unknown")
}

func invalidArgs(msg string) error {
	return &connectors.CodeOperationError{Code: "invalid_args", Message: msg}
}

// decodeArgs reads the code's argument strictly: an object of the known fields (null and no argument are an
// empty object).
func decodeArgs(raw json.RawMessage, into any) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		raw = []byte("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return invalidArgs("the argument is not an object of the operation's fields: " + strings.TrimPrefix(err.Error(), "json: "))
	}
	return nil
}
