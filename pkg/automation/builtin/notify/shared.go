package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Node types.
const (
	NodeNotify        = "notify"
	NodeSendToContact = "send_to_contact"
)

// Outcomes of the nodes.
const (
	OutcomeSent         = "sent"
	OutcomeNoRecipients = "no_recipients"
	OutcomeNoContact    = "no_contact"
	OutcomeFailed       = "failed"
)

// Codes of a failed node (output.error.code) besides the ones the engine and the delivery layer name.
const (
	// CodeUnavailable: the platform of notifications is not configured or a channel is not available
	// (serrors.FailedPrecondition of the delivery layer).
	CodeUnavailable = "unavailable"
	// CodeInvalidInput: a rendered value the node cannot send (an empty title, a link that leaves the
	// application, a user that is not a number).
	CodeInvalidInput = engine.CodeInvalidInput
	// CodeNoTriggerEvent: participants need the event that started the run, and the run has none.
	CodeNoTriggerEvent = "no_trigger_event"
	// CodeNoContactResolver: no ContactResolver is registered for the entity of the record.
	CodeNoContactResolver = "no_contact_resolver"
	// CodeContactLookup: the contact of the record could not be read now.
	CodeContactLookup = "contact_lookup_failed"
)

// Limits of what a notice carries, those of an in-app notification: the notification counts bytes, not
// characters, so a Cyrillic title has half the length it seems to.
const (
	maxTitle     = 300
	maxBody      = 10000
	maxActionURL = 2000
	maxMessage   = 500
)

// ContactLookup finds the resolver of the contacts of an entity; *notifications.ContactResolvers is one.
type ContactLookup interface {
	Lookup(entity string) (notifications.ContactResolver, bool)
}

// Config is what the nodes depend on.
type Config struct {
	// Sender is the delivery layer's AudienceSender (SendTx queues in the step's transaction, Resolve
	// reads for a dry run).
	Sender notifications.AudienceSender
	// Catalog holds the notification definitions of the events that trigger graphs: the recipient keys of
	// a definition are the participants a notify node may name. Nil: no participants.
	Catalog *notifications.Catalog
	// Contacts looks up the resolvers send_to_contact asks whether a record can be reached. Nil: every
	// record is taken as reachable and the delivery layer records a missing address when it delivers.
	Contacts ContactLookup
	// Pool is the database that reads outside the step's transaction use: a dry run's resolution
	// and a contact lookup (which needs a connection besides the one the step's transaction holds). The
	// engine's step context carries no pool, so Register requires it.
	Pool *pgxpool.Pool
	// BaseURL is the address of the application (https://erp.example.com): a link of a notice may be an
	// absolute path or a URL on this host. Empty: absolute paths only.
	BaseURL string
}

// validate refuses a configuration the nodes could not work with, so it fails when the nodes are registered
// and not in a run.
func (c Config) validate() error {
	switch {
	case c.Sender == nil:
		return errors.New("automation notify: Config.Sender is required")
	case c.Pool == nil:
		return errors.New("automation notify: Config.Pool is required: dry runs and contact lookups read outside the step's transaction")
	}
	if strings.TrimSpace(c.BaseURL) != "" {
		if u, err := url.Parse(strings.TrimSpace(c.BaseURL)); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("automation notify: Config.BaseURL %q is not an http(s) address", c.BaseURL)
		}
	}
	return nil
}

// Register adds both node types to the catalog registry.
func Register(reg catalog.Registry, cfg Config) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	for _, nt := range []catalog.NodeType{NewNotify(cfg), NewSendToContact(cfg)} {
		if err := reg.RegisterNode(nt); err != nil {
			return err
		}
	}
	return nil
}

// base is what both node types share.
type base struct {
	cfg  Config
	host string // the application's host from BaseURL
}

func newBase(cfg Config) base {
	b := base{cfg: cfg}
	if u, err := url.Parse(strings.TrimSpace(cfg.BaseURL)); err == nil && u.Host != "" {
		b.host = strings.ToLower(u.Host)
	}
	return b
}

// tenantCtx is the context the delivery layer reads the tenant from.
func tenantCtx(ctx context.Context, run engine.StepContext) context.Context {
	return composables.WithTenantID(ctx, run.TenantID)
}

// readCtx is the context for reads that must not run in the step's transaction (a failed statement there
// would abort the step): the tenant, the pool, and no transaction.
func (b base) readCtx(ctx context.Context, run engine.StepContext) context.Context {
	ctx = context.WithValue(ctx, constants.TxKey, nil)
	if b.cfg.Pool != nil {
		ctx = composables.WithPool(ctx, b.cfg.Pool)
	}
	return tenantCtx(ctx, run)
}

// producer names the step: the run, and the node, token and sequence of the step, so the same step applied
// again is the same producer and the delivery layer returns the notice it queued the first time.
func producer(run engine.StepContext, node graph.Node) notifications.Producer {
	if run.RunID == uuid.Nil {
		// A node test or a dry run is no run: its identity never repeats.
		return notifications.Producer{InstanceID: "test", Occurrence: node.ID + ":" + uuid.NewString()}
	}
	return notifications.Producer{InstanceID: run.RunID.String(), Occurrence: fmt.Sprintf("%s:%s:%d", node.ID, run.TokenID, run.Seq)}
}

// subjectRef is the run's subject as the notice's subject, whose access the delivery layer checks for each
// user; a run about nothing or a conversation has none.
func subjectRef(run engine.StepContext) notifications.SubjectReference {
	switch run.Subject.Entity {
	case "", graph.SubjectNone, graph.SubjectConversation:
		return notifications.SubjectReference{}
	}
	if run.Subject.Key == "" {
		return notifications.SubjectReference{}
	}
	return notifications.SubjectReference{Type: run.Subject.Entity, ID: run.Subject.Key}
}

// notice is the event of the definition the graph's text fills.
func notice(run engine.StepContext, title, body, actionURL string) notifications.Event {
	data := map[string]string{dataBody: body}
	if title != "" {
		data[dataTitle] = title
	}
	if actionURL != "" {
		data[dataActionURL] = actionURL
	}
	return notifications.Event{Key: EventKey, TenantID: run.TenantID, Subject: subjectRef(run), Data: data}
}

// failure maps an error of the delivery layer to the node's failure: the platform of notifications that is
// not configured or a channel that is not available is "unavailable", anything else carries the code of the
// error. A failure of the platform itself is returned as it is, so the engine retries the step.
func failure(err error) error {
	if err == nil {
		return nil
	}
	var se *engine.StepError
	if errors.As(err, &se) || host.IsInfrastructure(err) {
		return err
	}
	code := serrors.CodeOf(err)
	switch code { //nolint:exhaustive // every other code carries its own name and message
	case serrors.FailedPrecondition:
		return &engine.StepError{Code: CodeUnavailable, Message: short(err.Error())}
	case serrors.Internal:
		// An unclassified error (a failed statement, say) may carry text that has no place in a run's output.
		return &engine.StepError{Code: code.String(), Message: "the notification platform failed to queue the notice"}
	}
	return &engine.StepError{Code: code.String(), Message: short(err.Error()), Retryable: code == serrors.Unavailable || code == serrors.RateLimited || code == serrors.Timeout}
}

func short(s string) string {
	if len(s) <= maxMessage {
		return s
	}
	cut := maxMessage
	for cut > 0 && !isRuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

func invalid(format string, args ...any) error {
	return &engine.StepError{Code: CodeInvalidInput, Message: fmt.Sprintf(format, args...)}
}

// normalized is v as the engine stores it: decoded from its JSON, so the output a node returns and the one
// a later step reads have the same types.
func normalized(out map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := expr.UnmarshalJSON(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// result is an effect's outcome and its output in the form the engine stores.
func result(outcome string, out map[string]any) (engine.EffectResult, error) {
	norm, err := normalized(out)
	if err != nil {
		return engine.EffectResult{}, err
	}
	return engine.EffectResult{Outcome: outcome, Output: norm}, nil
}

// skippedOf is output.skipped.
func skippedOf(skipped []notifications.SkippedRecipient) []any {
	out := make([]any, 0, len(skipped))
	for _, s := range skipped {
		out = append(out, map[string]any{"kind": string(s.Kind), "ref": s.Ref, "reason": s.Reason})
	}
	return out
}

var skippedField = expr.ListOf(expr.ObjectOf(expr.Fields{
	{Name: "kind", Field: expr.StringField},
	{Name: "ref", Field: expr.StringField},
	{Name: "reason", Field: expr.StringField},
}))

// ---------------------------------------------------------------------------------------------------
// links

// checkActionURL checks the link of a notice template while a graph is saved: the text before its first
// template must be a path of the application or a URL on the application's host. A link that starts with a
// template is checked when it is rendered (resolveActionURL). The result is the reason it is refused, or "".
func checkActionURL(text, appHost string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	prefix, rest, templated := strings.Cut(text, "{{")
	if prefix == "" {
		return "" // the rendered link is checked when the node runs
	}
	_, reason := classifyLink(prefix, appHost, templated && rest != "")
	return reason
}

// resolveActionURL is a rendered link as the path the notification stores: a path of the application
// as it is, a URL on the application's host as its path, query and fragment. Anything else is refused.
func resolveActionURL(link, appHost string) (string, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return "", nil
	}
	path, reason := classifyLink(link, appHost, false)
	if reason != "" {
		return "", invalid("action_url: %s", reason)
	}
	if len(path) > maxActionURL {
		return "", invalid("action_url is longer than %d characters", maxActionURL)
	}
	// The notification refuses what is not an internal absolute path.
	if _, err := notification.New(1, "x", "", notification.WithActionURL(path)); err != nil {
		return "", invalid("action_url: %v", err)
	}
	return path, nil
}

// classifyLink returns the path a link stands for, or why it is refused. open is true when a template
// follows the text, so an authority that ends the text may still grow into another host.
func classifyLink(link, appHost string, open bool) (string, string) {
	if strings.ContainsFunc(link, unicode.IsControl) || strings.Contains(link, "\\") {
		return "", "must not contain control characters or backslashes"
	}
	if strings.HasPrefix(link, "//") {
		return "", "must lead to the application, not to another host"
	}
	if strings.HasPrefix(link, "/") {
		return internalPath(link)
	}
	scheme, rest, ok := strings.Cut(link, "://")
	if !ok || (!strings.EqualFold(scheme, "http") && !strings.EqualFold(scheme, "https")) {
		return "", "must be a path of the application (/orders/1) or a URL on its host"
	}
	if appHost == "" {
		return "", "the application's address is not known, so only paths of the application are allowed"
	}
	end := strings.IndexAny(rest, "/?#")
	if end < 0 {
		if open {
			return "", "the host of the link must not be a template"
		}
		end = len(rest)
	}
	authority := strings.ToLower(rest[:end])
	if strings.Contains(authority, "@") || authority != appHost {
		return "", "must lead to the application's host " + appHost
	}
	tail := rest[end:]
	if tail == "" || tail[0] != '/' {
		tail = "/" + tail
	}
	return internalPath(tail)
}

// internalPath is a path of the application as the notification stores it: what the notification itself
// would refuse (a path that decodes to a protocol-relative one, //evil.example.com) is refused here, when
// the graph is saved, and not only when a run sends.
func internalPath(path string) (string, string) {
	decoded, err := url.PathUnescape(path)
	if err != nil || strings.HasPrefix(decoded, "//") || strings.ContainsFunc(decoded, unicode.IsControl) || strings.Contains(decoded, "\\") {
		return "", "must be a path of the application, not a link to another host"
	}
	return path, ""
}

// ---------------------------------------------------------------------------------------------------
// params

// channelsOf reads the channels param: empty is the in-app channel.
func channelsOf(params map[string]any) ([]notifications.Channel, error) {
	raw, _ := params["channels"].([]any)
	out := make([]notifications.Channel, 0, len(raw))
	for _, v := range raw {
		s, _ := v.(string)
		c := notifications.Channel(s)
		if !c.Valid() {
			return nil, invalid("channel %q is not a channel of the application", s)
		}
		out = append(out, c)
	}
	return notifications.UniqueChannels(out), nil
}

// levelOf maps the node's level to the SDK's: critical is the SDK's error level.
func levelOf(params map[string]any) (notification.Level, error) {
	switch s, _ := params["level"].(string); s {
	case "", "info":
		return notification.LevelInfo, nil
	case "warning":
		return notification.LevelWarning, nil
	case "critical":
		return notification.LevelError, nil
	default:
		return "", invalid("level %q is not info, warning or critical", s)
	}
}

// text reads a rendered template param.
func text(params map[string]any, key string) string {
	s, _ := params[key].(string)
	return strings.TrimSpace(s)
}

// userIDs reads recipients.users: a list of positive whole numbers.
func userIDs(v any) ([]uint, error) {
	if v == nil {
		return nil, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, invalid("recipients.users is not a list")
	}
	out := make([]uint, 0, len(list))
	for _, item := range list {
		id, ok := wholeNumber(item)
		if !ok || id == 0 {
			return nil, invalid("recipients.users holds %v, which is not a user ID", item)
		}
		out = append(out, uint(id))
	}
	return out, nil
}

// wholeNumber reads a JSON or CEL number that has no fraction.
func wholeNumber(v any) (uint64, bool) {
	switch n := v.(type) {
	case int:
		return uint64(n), n > 0
	case int64:
		return uint64(n), n > 0
	case uint64:
		return n, true
	case float64:
		return uint64(n), n > 0 && n == float64(uint64(n))
	case json.Number:
		id, err := strconv.ParseUint(n.String(), 10, 64)
		return id, err == nil
	}
	return 0, false
}

// strs reads a list of strings.
func strs(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
