package event

type Subscriber struct {
	Event   string
	Handler func(data interface{})
}

type Publisher struct {
	Subscribers []Subscriber
}

func NewEventPublisher() *Publisher {
	return &Publisher{}
}

func (p *Publisher) Publish(event string, data interface{}) {
	for _, subscriber := range p.Subscribers {
		if subscriber.Event == event {
			subscriber.Handler(data)
		}
	}
}

func (p *Publisher) Subscribe(event string, handler func(data interface{})) {
	p.Subscribers = append(p.Subscribers, Subscriber{Event: event, Handler: handler})
}
