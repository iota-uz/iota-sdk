package services

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
)

const NotificationDispatchBatchSize = 100

type NotificationDispatchService struct {
	router *NotificationRoutingService
	repo   notifications.DispatchRepository
	inbox  *NotificationService
}

func NewNotificationDispatchService(router *NotificationRoutingService, repo notifications.DispatchRepository, inbox *NotificationService) *NotificationDispatchService {
	return &NotificationDispatchService{router: router, repo: repo, inbox: inbox}
}
func (s *NotificationDispatchService) Enqueue(ctx context.Context, event notifications.Event) (int, error) {
	const op serrors.Op = "NotificationDispatchService.Enqueue"
	normalized, err := s.router.Catalog().Normalize(event)
	if err != nil {
		return 0, serrors.New(serrors.Invalid, err.Error()).WithOp(op)
	}
	rule, ids, err := s.router.ResolveRecipients(ctx, normalized)
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	if !rule.Enabled {
		return 0, nil
	}
	if _, err := s.router.ensureNotice(ctx, normalized, notifications.NoticeRule); err != nil {
		return 0, serrors.Wrap(op, err)
	}
	job, err := s.repo.Enqueue(ctx, notifications.DispatchRequest{Event: normalized, Rule: rule, Mode: notifications.NoticeRule, RecipientIDs: ids})
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	return job.Total(), nil
}

// MaxExplicitRecipients bounds the recipients of one explicit or audience
// send, counted after resolution and deduplication.
const MaxExplicitRecipients = 5000

var _ notifications.AudienceSender = (*NotificationDispatchService)(nil)

// AddressedRequest is an explicit send: the producer names who receives the
// notice, so the administrator's event rule is neither read nor changed. It is
// an audience send whose audience holds only users, records and addresses.
type AddressedRequest struct {
	Event notifications.Event
	// RecipientIDs is shorthand for user recipients.
	RecipientIDs []uint
	// Recipients may be users, business records resolved through a registered
	// ContactResolver, or direct channel addresses.
	Recipients []notifications.Addressee
	// Channels defaults to in-app only. In-app applies to user recipients;
	// record and address recipients are reached on the external channels.
	Channels []notifications.Channel
	// Level overrides the event's default level when set.
	Level notification.Level
	// Producer identifies the invocation, event or workflow occurrence behind
	// the send. It is stored with the notice and, when the event has no
	// DedupeKey, derives one so a replayed producer step is not sent twice.
	Producer notifications.Producer
}

// AddressedReceipt reports what an explicit send accepted.
type AddressedReceipt = notifications.AudienceReceipt

// EnqueueAddressedTx queues an explicit send in the caller's Go-owned
// transaction: the notice, its recipients and the dispatch job commit with the
// business data and a rollback removes them all. The context must carry the
// tenant. Resolution of contacts, rendering and providers run in workers after
// commit, never inside tx.
func (s *NotificationDispatchService) EnqueueAddressedTx(ctx context.Context, tx pgx.Tx, req AddressedRequest) (AddressedReceipt, error) {
	return s.EnqueueExplicit(composables.WithTx(ctx, tx), req)
}

// EnqueueAddressed queues delivery to explicit recipients and returns how many
// the job holds. See EnqueueExplicit.
func (s *NotificationDispatchService) EnqueueAddressed(ctx context.Context, req AddressedRequest) (int, error) {
	receipt, err := s.EnqueueExplicit(ctx, req)
	return receipt.Recipients, err
}

// EnqueueExplicit queues an explicit send in the transaction in ctx. See Send.
func (s *NotificationDispatchService) EnqueueExplicit(ctx context.Context, req AddressedRequest) (AddressedReceipt, error) {
	return s.Send(ctx, notifications.AudienceRequest{
		Event: req.Event, Audience: notifications.Audience{Users: req.RecipientIDs, Addressees: req.Recipients},
		Channels: req.Channels, Level: req.Level, Producer: req.Producer,
	})
}

// SendTx queues an audience send in the caller's transaction. See Send.
func (s *NotificationDispatchService) SendTx(ctx context.Context, tx pgx.Tx, req notifications.AudienceRequest) (notifications.AudienceReceipt, error) {
	return s.Send(composables.WithTx(ctx, tx), req)
}

// Resolve expands the audience of req exactly as Send would, without writing.
func (s *NotificationDispatchService) Resolve(ctx context.Context, req notifications.AudienceRequest) (notifications.ResolvedAudience, error) {
	const op serrors.Op = "NotificationDispatchService.Resolve"
	prepared, err := s.prepare(ctx, req)
	if err != nil {
		return notifications.ResolvedAudience{}, serrors.Wrap(op, err)
	}
	return prepared.resolved, nil
}

// Send queues a send to an audience in the transaction in ctx, so rollback
// removes the notice and job. The administrator's rules are neither read nor
// changed. Groups and roles are expanded in the context's tenant and their
// membership is fixed now; eligibility, subject access and permissions are
// rechecked per user recipient, and policy per external target, when the
// worker processes the job. Errors a caller can act on are returned before
// anything is written: Invalid for an unknown notice, a malformed recipient,
// a record or address without an external channel, an unknown participant,
// participants without a source, a record entity without a contact resolver
// or more than MaxExplicitRecipients recipients; FailedPrecondition when the
// platform is not configured or a channel is unavailable.
func (s *NotificationDispatchService) Send(ctx context.Context, req notifications.AudienceRequest) (notifications.AudienceReceipt, error) {
	const op serrors.Op = "NotificationDispatchService.Send"
	prepared, err := s.prepare(ctx, req)
	if err != nil {
		return notifications.AudienceReceipt{}, serrors.Wrap(op, err)
	}
	invalid := func(message string) (notifications.AudienceReceipt, error) {
		return notifications.AudienceReceipt{}, serrors.New(serrors.Invalid, message).WithOp(op)
	}
	resolved := prepared.resolved
	for _, skipped := range resolved.Skipped {
		switch skipped.Reason {
		case notifications.SkipUnknownParticipant:
			return invalid("unknown notification participant: " + skipped.Ref)
		case notifications.SkipNoContactResolver:
			return invalid("no contact resolver is registered for the recipient entity")
		}
	}
	if len(resolved.Addressees) > 0 && !slices.ContainsFunc(prepared.channels, notifications.Channel.External) {
		return invalid("record and address recipients need an external channel")
	}
	if resolved.Total() > MaxExplicitRecipients {
		return invalid("too many notification recipients")
	}
	if resolved.Total() == 0 {
		return notifications.AudienceReceipt{}, nil
	}
	event := prepared.event
	// Explicit jobs have their own identity namespace, so an event rule that
	// already queued the same event ID cannot swallow the requested audience.
	identity := eventIdentity(event)
	event.DedupeKey = "addressed:" + identity
	if len(event.DedupeKey) > 200 {
		event.DedupeKey = fmt.Sprintf("addressed:sha256:%x", sha256.Sum256([]byte(identity)))
	}
	rule := notifications.Rule{EventKey: event.Key, Configured: true, Enabled: true, Level: req.Level, Channels: prepared.channels, UserIDs: resolved.UserIDs}
	notice, err := s.router.ensureNotice(ctx, event, notifications.NoticeExplicit)
	if err != nil {
		return notifications.AudienceReceipt{}, serrors.Wrap(op, err)
	}
	job, err := s.repo.Enqueue(ctx, notifications.DispatchRequest{Event: event, Rule: rule, Mode: notifications.NoticeExplicit, RecipientIDs: resolved.UserIDs, Addressees: resolved.Addressees})
	if err != nil {
		return notifications.AudienceReceipt{}, serrors.Wrap(op, err)
	}
	return notifications.AudienceReceipt{NoticeID: notice.ID, Recipients: job.Total()}, nil
}

type preparedAudience struct {
	event    notifications.Event
	channels []notifications.Channel
	resolved notifications.ResolvedAudience
}

// prepare validates the envelope and channels and resolves the audience.
func (s *NotificationDispatchService) prepare(ctx context.Context, req notifications.AudienceRequest) (preparedAudience, error) {
	invalid := func(message string) (preparedAudience, error) {
		return preparedAudience{}, serrors.New(serrors.Invalid, message)
	}
	if s.router.platform == nil || s.router.platform.Notices == nil || s.router.platform.Renderers == nil {
		return preparedAudience{}, serrors.New(serrors.FailedPrecondition, "notification platform is not configured")
	}
	if err := req.Producer.Validate(); err != nil {
		return invalid(err.Error())
	}
	event := req.Event
	if strings.TrimSpace(event.ID) == "" {
		event.ID = uuid.NewString()
	}
	if !req.Producer.IsZero() {
		event.Producer = req.Producer
	}
	channels := notifications.NormalizeChannels(req.Channels)
	available := s.router.Channels()
	for _, c := range channels {
		if !c.Valid() {
			return invalid("invalid notification channel")
		}
		if !slices.Contains(available, c) {
			return preparedAudience{}, serrors.New(serrors.FailedPrecondition, "notification channel is not available: "+string(c))
		}
	}
	if event.DedupeKey == "" {
		event.DedupeKey = notifications.DeriveAudienceDedupeKey(event.Producer, event, req.Audience, req.Source, channels)
	}
	event, err := s.router.Catalog().Normalize(event)
	if err != nil {
		return invalid(err.Error())
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return preparedAudience{}, err
	}
	if tenant != event.TenantID || (req.Level != "" && !req.Level.Valid()) {
		return invalid("invalid notification event envelope")
	}
	if err := s.router.validateActor(ctx, event); err != nil {
		return preparedAudience{}, err
	}
	resolved, err := s.router.audienceResolver(true).Resolve(ctx, req.Audience, req.Source)
	if errors.Is(err, notifications.ErrInvalidAudience) {
		return invalid(err.Error())
	}
	if err != nil {
		return preparedAudience{}, err
	}
	return preparedAudience{event: event, channels: channels, resolved: resolved}, nil
}

func (s *NotificationDispatchService) Progress(ctx context.Context) (notifications.DispatchStats, error) {
	return s.repo.Stats(ctx)
}
func (s *NotificationDispatchService) Jobs(ctx context.Context) ([]notifications.DispatchSummary, error) {
	return s.repo.List(ctx)
}
func (s *NotificationDispatchService) Retry(ctx context.Context, id uuid.UUID) error {
	return s.repo.Retry(ctx, id)
}

// Process commits notification writes and their checkpoint together. A stopped
// process leaves the last committed checkpoint available to any replica.
func (s *NotificationDispatchService) Process(ctx context.Context) (bool, error) {
	const op serrors.Op = "NotificationDispatchService.Process"
	if ctx.Value(constants.TxKey) != nil {
		return false, serrors.New(serrors.FailedPrecondition, "Dispatch processing requires its own transaction").WithOp(op)
	}
	var jobID uuid.UUID
	committed := []notification.Notification{}
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		job, err := s.repo.Next(txCtx)
		if err != nil {
			return err
		}
		if job == nil {
			return nil
		}
		jobID = job.ID
		end := min(job.Cursor+NotificationDispatchBatchSize, job.Total())
		txCtx = context.WithValue(txCtx, notificationDeliveryBufferKey{}, &committed)
		users := len(job.RecipientIDs)
		delivered := 0
		if from, to := min(job.Cursor, users), min(end, users); from < to {
			n, err := s.router.deliverUsers(txCtx, job.Event, job.Rule, job.RecipientIDs[from:to], job.Mode)
			if err != nil {
				return err
			}
			delivered += n
		}
		if end > users {
			n, err := s.router.deliverAddressees(txCtx, job.Event, job.Rule, job.Addressees[max(job.Cursor-users, 0):end-users])
			if err != nil {
				return err
			}
			delivered += n
		}
		return s.repo.Advance(txCtx, job.ID, end-job.Cursor, delivered)
	})
	if err != nil {
		if jobID != uuid.Nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if retryErr := s.repo.Fail(cleanup, jobID); retryErr != nil {
				return true, serrors.WrapContext(op, retryErr, "failed to record dispatch retry after delivery failure")
			}
		}
		return jobID != uuid.Nil, serrors.Wrap(op, err)
	}
	for _, n := range committed {
		if ctx.Err() != nil {
			break
		}
		s.inbox.NotifyCommitted(ctx, n)
	}
	return jobID != uuid.Nil, nil
}
