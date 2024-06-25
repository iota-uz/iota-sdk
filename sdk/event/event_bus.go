package event

import "reflect"

type Event interface {
	Name() string
	Data() interface{}
}

type Subscriber struct {
	Handler interface{}
}

type Publisher struct {
	Subscribers []Subscriber
}

func NewEventPublisher() *Publisher {
	return &Publisher{}
}

func (p *Publisher) checkSignature(handler interface{}, args []interface{}) bool {
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		return false
	}
	if t.NumIn() != len(args) {
		return false
	}
	for i, arg := range args {
		if t.In(i) != reflect.TypeOf(arg) {
			return false
		}
	}
	return true
}

func (p *Publisher) Publish(args ...interface{}) {
	for _, subscriber := range p.Subscribers {
		v := reflect.ValueOf(subscriber.Handler)
		if !p.checkSignature(subscriber.Handler, args) {
			continue
		}
		in := make([]reflect.Value, len(args))
		for i, arg := range args {
			in[i] = reflect.ValueOf(arg)
		}
		v.Call(in)
	}
}

func (p *Publisher) Subscribe(handler interface{}) {
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		panic("handler must be a function")
	}
	p.Subscribers = append(
		p.Subscribers,
		Subscriber{Handler: handler},
	)
}

func (p *Publisher) Unsubscribe(handler interface{}) {
	for i, subscriber := range p.Subscribers {
		if subscriber.Handler == handler {
			p.Subscribers = append(p.Subscribers[:i], p.Subscribers[i+1:]...)
			return
		}
	}
}

func (p *Publisher) Clear() {
	p.Subscribers = []Subscriber{}
}

func (p *Publisher) SubscribersCount() int {
	return len(p.Subscribers)
}
