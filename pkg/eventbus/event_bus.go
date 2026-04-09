// Package eventbus provides this package.
package eventbus

import (
	"reflect"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

type Subscriber struct {
	ID      uint64
	Handler interface{}
}

type EventBus interface {
	Publish(args ...interface{})
	Subscribe(handler interface{}) func()
	Clear()
	SubscribersCount() int
}

type publisherImpl struct {
	log         *logrus.Logger
	Subscribers []Subscriber
	nextID      atomic.Uint64
}

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

func (p *publisherImpl) Publish(args ...interface{}) {
	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		in[i] = reflect.ValueOf(arg)
	}

	handled := false
	for _, subscriber := range p.Subscribers {
		v := reflect.ValueOf(subscriber.Handler)
		if !MatchSignature(subscriber.Handler, args) {
			continue
		}
		// Wrap handler invocation with panic recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					handlerName := v.Type().String()
					// Log panic with error level and include event args for debugging
					p.log.Errorf("eventbus: handler %s panicked with args %v: %v", handlerName, args, r)
				}
			}()
			v.Call(in)
			// Only mark as handled if handler completed successfully without panic
			handled = true
		}()
	}

	if !handled {
		p.log.Warnf("eventbus.Publish: no matching subscribers for event with args: %v", in)
		return
	}
}

func (p *publisherImpl) Subscribe(handler interface{}) func() {
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		panic("handler must be a function")
	}
	id := p.nextID.Add(1)
	p.Subscribers = append(
		p.Subscribers,
		Subscriber{ID: id, Handler: handler},
	)
	return func() {
		p.unsubscribeByID(id)
	}
}

func (p *publisherImpl) unsubscribeByID(id uint64) {
	for i, subscriber := range p.Subscribers {
		if subscriber.ID == id {
			p.Subscribers = append(p.Subscribers[:i], p.Subscribers[i+1:]...)
			return
		}
	}
}

func (p *publisherImpl) Clear() {
	p.Subscribers = []Subscriber{}
}

func (p *publisherImpl) SubscribersCount() int {
	return len(p.Subscribers)
}
