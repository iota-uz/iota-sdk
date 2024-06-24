package event

import "testing"

type args struct {
	data interface{}
}

func (a *args) Name() string {
	return "test"
}

func (a *args) Data() interface{} {
	return a
}

func TestPublisher_Publish(t *testing.T) {
	type args2 struct {
		data interface{}
	}
	publisher := NewEventPublisher()
	publisher.Subscribe(func(e *args) {
		t.Error("should not be called")
	})
	publisher.Publish(&args2{})
}

func TestPublisher_Subscribe(t *testing.T) {
	publisher := NewEventPublisher()
	called := false
	publisher.Subscribe(func(e *args) {
		called = true
	})
	publisher.Publish(&args{})
	if !called {
		t.Error("should be called")
	}
}
