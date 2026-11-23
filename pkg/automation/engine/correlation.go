package engine

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// Asker is an optional interface of a catalog.CallNode whose answer comes later as an inbound event:
// the node sends a question, the engine waits for the event that carries the question's correlation
// token and maps it to the node's outcome.
type Asker interface {
	catalog.CallNode
	// Await is called in the child after Execute succeeded; the parent registers the wait.
	Await(in catalog.CallInput, res catalog.CallResult) (Await, error)
	// Answer maps the matched inbound event to the node outcome. Called in the parent step.
	Answer(payload json.RawMessage) (outcome string, output json.RawMessage, err error)
}

// Correlation is issued by the engine before Execute and passed in ctx.
// Token: base62, 12 chars, globally unique while the wait is open (fits Telegram callback_data).
type Correlation struct{ Token string }

type correlationKey struct{}

// WithCorrelation is ctx carrying the correlation of an ask node's call.
func WithCorrelation(ctx context.Context, c Correlation) context.Context {
	return context.WithValue(ctx, correlationKey{}, c)
}

// CorrelationFrom is the correlation of the call; false for nodes that are not Asker.
func CorrelationFrom(ctx context.Context) (Correlation, bool) {
	c, ok := ctx.Value(correlationKey{}).(Correlation)
	return c, ok && c.Token != ""
}

// Await is the answer wait an ask node asks for.
type Await struct {
	Event   string        // inbound event key, e.g. "telegram.callback_answered.v1"
	Timeout time.Duration // ≤ Limits.WaitMax; elapsed → outcome "timeout"
	Single  bool          // first matching event closes the wait; later ones are ignored
}

// ErrCorrelationClosed is an inbound event with a correlation token that is unknown or whose wait is
// closed: the connector answers the provider without an error ("already answered").
var ErrCorrelationClosed = errors.New("automation: correlation_closed")

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// CorrelationTokenLen is the length of a correlation token.
const CorrelationTokenLen = 12

// NewCorrelationToken is a random base62 token of CorrelationTokenLen characters (~71 bits).
func NewCorrelationToken() string {
	b := make([]byte, CorrelationTokenLen)
	limit := big.NewInt(int64(len(base62)))
	for i := range b {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			panic("automation: no randomness for a correlation token: " + err.Error())
		}
		b[i] = base62[n.Int64()]
	}
	return string(b)
}

// validToken reports whether s has the shape of a correlation token.
func validToken(s string) bool {
	if len(s) != CorrelationTokenLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
			return false
		}
	}
	return true
}
