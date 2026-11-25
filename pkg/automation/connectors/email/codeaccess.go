package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// OpSend is the code operation of an smtp connection (env.connections.<name>.send).
const OpSend = "send"

var (
	_ connectors.CodeAccessor = (*smtpConnector)(nil)
	_ connectors.CodeOperator = (*smtpConnector)(nil)
)

// CodeAccess offers the operation send: a message through the connection's SMTP server or sign-in, from the
// connection's own sender address.
func (c *smtpConnector) CodeAccess() connectors.CodeAccess {
	return connectors.CodeAccess{Operations: []connectors.CodeOperation{{
		Name:        OpSend,
		Description: "Send an email through the connection, from its sender address.",
		Params:      `{ to: string | string[]; cc?: string | string[]; bcc?: string | string[]; subject: string; text?: string; html?: string; reply_to?: string }`,
		Result:      `{ message_id: string; accepted: string[]; rejected: string[] }`,
	}}}
}

// Authorize sets nothing: the operation sends over SMTP with the connection's own credentials.
func (c *smtpConnector) Authorize(context.Context, connectors.Credentials, http.Header) error {
	return nil
}

// sendArgs is the argument of send; a list field takes one address or a list.
type sendArgs struct {
	To      addressArg `json:"to"`
	Cc      addressArg `json:"cc"`
	Bcc     addressArg `json:"bcc"`
	Subject *string    `json:"subject"`
	Text    string     `json:"text"`
	HTML    string     `json:"html"`
	ReplyTo string     `json:"reply_to"`
}

// addressArg is a string or a list of strings.
type addressArg []string

func (a *addressArg) UnmarshalJSON(raw []byte) error {
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		*a = addressArg{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return errors.New("an address or a list of addresses")
	}
	*a = many
	return nil
}

func (a addressArg) values() []any { return strs(a) }

// CallCodeOperation sends the message with the send node's own path: the same checks of the addresses, the
// same message, the same delivery and the same handling of a refused sign-in.
func (c *smtpConnector) CallCodeOperation(ctx context.Context, call connectors.CodeCall) (any, error) {
	if call.Operation != OpSend {
		return nil, &connectors.CodeOperationError{Code: "unknown_operation", Message: "the connection has no operation " + call.Operation}
	}
	raw := bytes.TrimSpace(call.Args)
	if len(raw) == 0 || string(raw) == "null" {
		raw = []byte("{}")
	}
	var args sendArgs
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&args); err != nil {
		return nil, invalidArgs("the argument is not an object of to, cc, bcc, subject, text, html and reply_to: " + strings.TrimPrefix(err.Error(), "json: "))
	}
	switch {
	case len(args.To) == 0:
		return nil, invalidArgs("to has no address")
	case args.Subject == nil:
		return nil, invalidArgs("subject is required")
	case len(*args.Subject) > 998:
		return nil, invalidArgs("subject is at most 998 characters")
	case strings.TrimSpace(args.Text) == "" && strings.TrimSpace(args.HTML) == "":
		return nil, invalidArgs("the message has no text or html")
	case len(args.Text) > 1000000 || len(args.HTML) > 1000000:
		return nil, invalidArgs("text and html are at most 1000000 characters")
	}
	params := map[string]any{"to": args.To.values(), "subject": *args.Subject, "text": args.Text}
	if len(args.Cc) > 0 {
		params["cc"] = args.Cc.values()
	}
	if len(args.Bcc) > 0 {
		params["bcc"] = args.Bcc.values()
	}
	if args.HTML != "" {
		params["html"] = args.HTML
	}
	if args.ReplyTo != "" {
		params["reply_to"] = args.ReplyTo
	}
	in := call.Input
	in.Params = params
	// Every call of the code is its own message: its id comes from a key no other call has.
	in.IdempotencyKey = "code:" + in.RunID.String() + ":" + uuid.NewString()
	in.Attempt = 1
	res, err := (&sendNode{c.hub}).Execute(ctx, in)
	if err != nil {
		return nil, codeFailure(err)
	}
	return res.Output, nil
}

func invalidArgs(msg string) error {
	return &connectors.CodeOperationError{Code: "invalid_args", Message: msg}
}

// codeFailure turns the send node's failure into what code handles.
func codeFailure(err error) error {
	var ce *catalog.CallError
	if !errors.As(err, &ce) {
		return errors.New("email: the message was not sent")
	}
	switch ce.Code {
	case catalog.CallInvalidRequest:
		return &connectors.CodeOperationError{Code: "invalid_args", Message: ce.Message}
	case catalog.CallRateLimited:
		return &connectors.CodeOperationError{Code: "rate_limited", Message: ce.Message, Retryable: true}
	case catalog.CallUnavailable:
		return &connectors.CodeOperationError{Code: "unavailable", Message: ce.Message, Retryable: true}
	case catalog.CallOutcomeUnknown:
		return &connectors.CodeOperationError{Code: "outcome_unknown", Message: ce.Message}
	}
	return &connectors.CodeOperationError{Code: "provider_refused", Message: ce.Message}
}
