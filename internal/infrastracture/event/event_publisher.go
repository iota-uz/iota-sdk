package event

import "fmt"

type Publisher struct{}

func NewEventPublisher() *Publisher {
	return &Publisher{}
}

func (p *Publisher) Publish(event string, data interface{}) {
	fmt.Printf("Event published: %s - Data: %v\n", event, data)
}
