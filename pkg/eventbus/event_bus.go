// Package eventbus provides an in-process, reflection-based event bus.
//
// Handlers are plain functions; an event is delivered to every subscriber
// whose parameter list matches the published arguments (see MatchSignature).
//
// The bus is safe for concurrent use: Subscribe, unsubscribe, Clear and
// publishing may run from different goroutines. Handlers are invoked outside
// the internal lock, so a handler may itself subscribe, unsubscribe or publish.
//
// Event payloads frequently carry personal data, so the bus never writes
// argument values to the log; only argument types and handler counts are logged.
package eventbus

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

type Subscriber struct {
	ID      uint64
	Handler interface{}
}

// EventBus is the publish/subscribe contract shared across modules.
//
// New code should publish through PublishCtx (the package function), which
// honours context cancellation and reports handler failures to the caller.
type EventBus interface {
	// Publish delivers args to every matching subscriber. It cannot be
	// cancelled and swallows handler failures (they are only logged).
	//
	// Deprecated: use the package function PublishCtx, or ContextPublisher.PublishCtx,
	// which accept a context and return handler failures.
	Publish(args ...interface{})
	Subscribe(handler interface{}) func()
	Clear()
	SubscribersCount() int
}

// ContextPublisher is implemented by buses that can publish with a context and
// report handler failures. The bus returned by NewEventPublisher implements it.
type ContextPublisher interface {
	// PublishCtx delivers args to every matching subscriber in subscription
	// order. It stops before the next handler once ctx is done and returns
	// ctx.Err() joined with any handler failures collected so far.
	//
	// A handler failure is a panic or a non-nil error returned as the handler's
	// last result. Every matching handler still runs after another one fails;
	// all failures are joined into the returned error (see HandlerError).
	PublishCtx(ctx context.Context, args ...interface{}) error
}

// HandlerError describes a failed event handler. Its message names the handler
// and event types only, never argument values.
type HandlerError struct {
	// Handler is the handler's function type, e.g. "func(*user.Created) error".
	Handler string
	// Event lists the published argument types.
	Event string
	// Panic is the recovered value when the handler panicked; nil otherwise.
	Panic interface{}
	// Err is the error returned by the handler, or a description of the panic.
	Err error
}

func (e *HandlerError) Error() string {
	return fmt.Sprintf("eventbus: handler %s failed for event [%s]: %v", e.Handler, e.Event, e.Err)
}

func (e *HandlerError) Unwrap() error {
	return e.Err
}

// PublishCtx publishes through bus with a context and returns handler failures.
// Buses that do not implement ContextPublisher fall back to the legacy Publish
// after checking ctx, in which case handler failures cannot be reported.
func PublishCtx(ctx context.Context, bus EventBus, args ...interface{}) error {
	if cp, ok := bus.(ContextPublisher); ok {
		return cp.PublishCtx(ctx, args...)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	bus.Publish(args...)
	return nil
}

type publisherImpl struct {
	log         *logrus.Logger
	mu          sync.RWMutex
	subscribers []Subscriber
	nextID      atomic.Uint64
}

var _ ContextPublisher = (*publisherImpl)(nil)

func NewEventPublisher(log *logrus.Logger) EventBus {
	return &publisherImpl{log: log}
}

func MatchSignature(handler interface{}, args []interface{}) bool {
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		return false
	}

	if t.NumIn() != len(args) {
		return false
	}

	for i, arg := range args {
		paramType := t.In(i)
		argType := reflect.TypeOf(arg)

		// Handle nil arguments
		if arg == nil {
			if paramType.Kind() != reflect.Interface && paramType.Kind() != reflect.Ptr {
				return false
			}
			continue
		}

		// If the parameter is an interface, check if argument implements it
		if paramType.Kind() == reflect.Interface {
			if !argType.Implements(paramType) {
				return false
			}
			continue
		}

		// For other types, check direct assignability
		if !argType.AssignableTo(paramType) {
			return false
		}
	}

	return true
}

// Publish implements EventBus.
//
// Deprecated: use PublishCtx.
func (p *publisherImpl) Publish(args ...interface{}) {
	_ = p.publish(context.Background(), args)
}

// PublishCtx implements ContextPublisher.
func (p *publisherImpl) PublishCtx(ctx context.Context, args ...interface{}) error {
	return p.publish(ctx, args)
}

func (p *publisherImpl) publish(ctx context.Context, args []interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Snapshot under the read lock and call handlers without holding it, so a
	// handler may subscribe, unsubscribe or publish without deadlocking.
	p.mu.RLock()
	subscribers := slices.Clone(p.subscribers)
	p.mu.RUnlock()

	var (
		event    string
		failures []error
		handled  int
	)
	for _, subscriber := range subscribers {
		if !MatchSignature(subscriber.Handler, args) {
			continue
		}
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		if event == "" {
			event = eventTypes(args)
		}
		if err := p.call(subscriber.Handler, args, event); err != nil {
			failures = append(failures, err)
			continue
		}
		handled++
	}

	if handled == 0 && ctx.Err() == nil {
		if event == "" {
			event = eventTypes(args)
		}
		p.log.WithField("event", event).
			Warn("eventbus.Publish: no matching subscribers for event")
	}

	return errors.Join(failures...)
}

// call invokes handler and converts a panic or a returned error into a
// *HandlerError. Failures are logged with types only, never argument values.
func (p *publisherImpl) call(handler interface{}, args []interface{}, event string) (err error) {
	v := reflect.ValueOf(handler)
	handlerName := v.Type().String()
	in := callArgs(v.Type(), args)

	defer func() {
		if r := recover(); r != nil {
			herr := &HandlerError{
				Handler: handlerName,
				Event:   event,
				Panic:   r,
				Err:     fmt.Errorf("panic: %v", r),
			}
			p.log.WithFields(logrus.Fields{
				"handler": handlerName,
				"event":   event,
			}).Errorf("eventbus: handler %s panicked: %v", handlerName, r)
			err = herr
		}
	}()

	out := v.Call(in)
	if herr := returnedError(out); herr != nil {
		p.log.WithFields(logrus.Fields{
			"handler": handlerName,
			"event":   event,
		}).Errorf("eventbus: handler %s returned error: %v", handlerName, herr)
		return &HandlerError{Handler: handlerName, Event: event, Err: herr}
	}
	return nil
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// returnedError extracts a non-nil error from a handler's last result, if any.
func returnedError(out []reflect.Value) error {
	if len(out) == 0 {
		return nil
	}
	last := out[len(out)-1]
	if !last.Type().Implements(errorType) {
		return nil
	}
	switch last.Kind() { //nolint:exhaustive // only nillable kinds need the check
	case reflect.Interface, reflect.Ptr, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		if last.IsNil() {
			return nil
		}
	}
	err, _ := last.Interface().(error)
	return err
}

// callArgs converts args into reflect.Call inputs for a handler of type fn.
// A nil argument becomes the zero value of the matching parameter (MatchSignature
// only allows nil for pointer and interface parameters); reflect.ValueOf(nil)
// would be rejected by reflect.Call.
func callArgs(fn reflect.Type, args []interface{}) []reflect.Value {
	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		if arg == nil {
			in[i] = reflect.Zero(fn.In(i))
			continue
		}
		in[i] = reflect.ValueOf(arg)
	}
	return in
}

// eventTypes renders the argument types of an event for logs and errors.
func eventTypes(args []interface{}) string {
	names := make([]string, len(args))
	for i, arg := range args {
		if arg == nil {
			names[i] = "nil"
			continue
		}
		names[i] = reflect.TypeOf(arg).String()
	}
	return strings.Join(names, ", ")
}

func (p *publisherImpl) Subscribe(handler interface{}) func() {
	t := reflect.TypeOf(handler)
	if t == nil || t.Kind() != reflect.Func {
		panic("handler must be a function")
	}
	id := p.nextID.Add(1)
	p.mu.Lock()
	p.subscribers = append(p.subscribers, Subscriber{ID: id, Handler: handler})
	p.mu.Unlock()
	return func() {
		p.unsubscribeByID(id)
	}
}

func (p *publisherImpl) unsubscribeByID(id uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subscribers = slices.DeleteFunc(p.subscribers, func(s Subscriber) bool {
		return s.ID == id
	})
}

func (p *publisherImpl) Clear() {
	p.mu.Lock()
	p.subscribers = nil
	p.mu.Unlock()
}

func (p *publisherImpl) SubscribersCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.subscribers)
}
