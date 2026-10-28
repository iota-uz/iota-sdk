package history

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/health"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// Consumer is a downstream handler of stored business events, such as a projection or an
// integration. Deliveries are at least once: Handle must be idempotent by event ID. Its database
// writes share the transaction that marks the delivery done.
type Consumer interface {
	// Name identifies the consumer in the outbox and must stay stable across releases.
	Name() string
	Handle(ctx context.Context, e businessevent.Event) error
}

type Consumers struct {
	mu     sync.RWMutex
	byName map[string]Consumer
}

func NewConsumers() *Consumers {
	return &Consumers{byName: make(map[string]Consumer)}
}

func (c *Consumers) Register(consumers ...Consumer) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, consumer := range consumers {
		name := consumer.Name()
		if !namePattern.MatchString(name) {
			return fmt.Errorf("invalid business event consumer name %q", name)
		}
		if _, exists := c.byName[name]; exists {
			return fmt.Errorf("business event consumer already registered: %s", name)
		}
		c.byName[name] = consumer
	}
	return nil
}

func (c *Consumers) names() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(c.byName))
	for name := range c.byName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *Consumers) get(name string) (Consumer, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	consumer, ok := c.byName[name]
	return consumer, ok
}

// ContributeConsumers registers downstream consumers when the container is built.
func ContributeConsumers(builder *composition.Builder, factory func(*composition.Container) ([]Consumer, error)) {
	composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
		consumers, err := composition.Resolve[*Consumers](container)
		if err != nil {
			return nil, err
		}
		list, err := factory(container)
		if err != nil {
			return nil, err
		}
		return nil, consumers.Register(list...)
	})
}

// WithOutbox schedules every recorded event for every registered consumer in the same
// transaction, so an event and its pending deliveries commit together.
func WithOutbox(w Writer, consumers *Consumers, deliveries businessevent.DeliveryRepository) Writer {
	return &outboxWriter{next: w, consumers: consumers, deliveries: deliveries}
}

type outboxWriter struct {
	next       Writer
	consumers  *Consumers
	deliveries businessevent.DeliveryRepository
}

func (w *outboxWriter) Record(ctx context.Context, in Input) (uuid.UUID, error) {
	const op serrors.Op = "history.outboxWriter.Record"
	id, err := w.next.Record(ctx, in)
	if err != nil {
		return uuid.Nil, err
	}
	if names := w.consumers.names(); len(names) > 0 {
		if err := w.deliveries.Enqueue(ctx, id, names); err != nil {
			return uuid.Nil, serrors.Wrap(op, fmt.Errorf("%w: %w", ErrPersistenceFailed, err))
		}
	}
	return id, nil
}

var errConsumerMissing = errors.New("consumer is not registered")

// Dispatcher delivers scheduled events to consumers. A failed delivery is retried with a growing
// delay and stays visible in ListFailed and the system diagnostics until it succeeds; it never
// touches the stored event.
type Dispatcher struct {
	consumers  *Consumers
	events     businessevent.Repository
	deliveries businessevent.DeliveryRepository
	pool       *pgxpool.Pool
	logger     *logrus.Logger
}

func NewDispatcher(consumers *Consumers, events businessevent.Repository, deliveries businessevent.DeliveryRepository, pool *pgxpool.Pool, logger *logrus.Logger) *Dispatcher {
	return &Dispatcher{consumers: consumers, events: events, deliveries: deliveries, pool: pool, logger: logger}
}

// Deliver hands the next due delivery of any tenant to its consumer. It reports whether one was due.
func (d *Dispatcher) Deliver(ctx context.Context) (bool, error) {
	const op serrors.Op = "history.Dispatcher.Deliver"
	if ctx.Value(constants.TxKey) != nil {
		return false, serrors.New(serrors.FailedPrecondition, "delivery requires its own transaction").WithOp(op)
	}
	base := composables.WithPool(ctx, d.pool)
	var claimed *businessevent.Delivery
	err := composables.InTx(base, func(txCtx context.Context) error {
		delivery, err := d.deliveries.Claim(txCtx)
		if err != nil || delivery == nil {
			return err
		}
		claimed = delivery
		consumer, ok := d.consumers.get(delivery.Consumer)
		if !ok {
			return errConsumerMissing
		}
		txCtx = composables.WithTenantID(txCtx, delivery.TenantID)
		event, err := d.events.Get(txCtx, delivery.EventID)
		if err != nil {
			return err
		}
		if err := handle(txCtx, consumer, event); err != nil {
			return err
		}
		return d.deliveries.Complete(txCtx, *delivery)
	})
	if err == nil || claimed == nil {
		return claimed != nil, serrors.Wrap(op, err)
	}
	failCtx, cancel := context.WithTimeout(composables.WithTenantID(context.WithoutCancel(base), claimed.TenantID), 10*time.Second)
	defer cancel()
	if failErr := composables.InTx(failCtx, func(txCtx context.Context) error {
		return d.deliveries.Fail(txCtx, *claimed, err.Error())
	}); failErr != nil {
		return true, serrors.Wrap(op, errors.Join(err, failErr))
	}
	return true, serrors.Wrap(op, fmt.Errorf("event %s of operation %s to %s: %w", claimed.EventID, claimed.OperationID, claimed.Consumer, err))
}

func handle(ctx context.Context, consumer Consumer, event businessevent.Event) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("consumer panicked: %v", recovered)
		}
	}()
	return consumer.Handle(ctx, event)
}

// Retry makes a failed delivery of the current tenant due now. The event itself, with its actor
// and time, is never rewritten.
func (d *Dispatcher) Retry(ctx context.Context, eventID uuid.UUID, consumer string) error {
	return composables.InTx(ctx, func(txCtx context.Context) error {
		return d.deliveries.Retry(txCtx, eventID, consumer)
	})
}

func (d *Dispatcher) ListFailed(ctx context.Context, limit int) ([]businessevent.Delivery, error) {
	return d.deliveries.ListFailed(ctx, limit)
}

// Start runs the delivery loop until the returned stop function is called.
func (d *Dispatcher) Start(parent context.Context) (composition.StopFn, error) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			d.drain(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func(stopCtx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-stopCtx.Done():
			return stopCtx.Err()
		}
	}, nil
}

func (d *Dispatcher) drain(ctx context.Context) {
	for range 100 {
		if ctx.Err() != nil {
			return
		}
		due, err := d.Deliver(ctx)
		if err != nil && ctx.Err() == nil {
			d.logger.WithError(err).Error("business event delivery failed; retry scheduled")
		}
		if !due {
			return
		}
	}
}

// Probe reports undelivered events in the system diagnostics with the IDs needed to retry them.
func (d *Dispatcher) Probe(ctx context.Context) health.Capability {
	capability := health.Capability{Key: "business_history_outbox", Name: "Business history outbox", Enabled: true, Source: "logging", Status: health.StatusHealthy}
	summary, err := d.deliveries.Failures(composables.WithPool(ctx, d.pool))
	switch {
	case err != nil:
		capability.Status = health.StatusUnknown
		capability.Message = "failed to read the outbox"
	case summary.Oldest != nil:
		capability.Status = health.StatusDegraded
		capability.Message = fmt.Sprintf("%d failed deliveries; oldest: event %s of operation %s to %s, %d attempts",
			summary.Count, summary.Oldest.EventID, summary.Oldest.OperationID, summary.Oldest.Consumer, summary.Oldest.Attempts)
	}
	return capability
}
