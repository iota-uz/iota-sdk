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
	publisher.Publish(&args2{
		data: "test",
	})
}

func TestPublisher_Subscribe(t *testing.T) {
	publisher := NewEventPublisher()
	called := false
	var data interface{}
	publisher.Subscribe(func(e *args) {
		called = true
		data = e.data
	})
	publisher.Publish(&args{
		data: "test",
	})
	if !called {
		t.Error("should be called")
	}
	if data != "test" {
		t.Errorf("expected: %v, got: %v", "test", data)
	}
}
