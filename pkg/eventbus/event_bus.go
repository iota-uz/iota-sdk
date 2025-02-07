package eventbus

import (
	"reflect"
)

type Subscriber struct {
	Handler interface{}
}

type EventBus interface {
	Publish(args ...interface{})
	Subscribe(handler interface{})
	Unsubscribe(handler interface{})
	Clear()
	SubscribersCount() int
}

type publisherImpl struct {
	Subscribers []Subscriber
}

func NewEventPublisher() EventBus {
	return &publisherImpl{}
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
	for _, subscriber := range p.Subscribers {
		v := reflect.ValueOf(subscriber.Handler)
		if !MatchSignature(subscriber.Handler, args) {
			continue
		}
		in := make([]reflect.Value, len(args))
		for i, arg := range args {
			in[i] = reflect.ValueOf(arg)
		}
		v.Call(in)
	}
}

func (p *publisherImpl) Subscribe(handler interface{}) {
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		panic("handler must be a function")
	}
	p.Subscribers = append(
		p.Subscribers,
		Subscriber{Handler: handler},
	)
}

func (p *publisherImpl) Unsubscribe(handler interface{}) {
	for i, subscriber := range p.Subscribers {
		if subscriber.Handler == handler {
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
