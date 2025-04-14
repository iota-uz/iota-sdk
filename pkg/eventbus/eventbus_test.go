package eventbus

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/logging"

	"github.com/sirupsen/logrus"
)

type args struct {
	data interface{}
}

func TestPublisher_Publish(t *testing.T) {
	type args2 struct {
		data interface{}
	}
	logBuffer := bytes.Buffer{}
	log := logrus.New()
	log.SetOutput(&logBuffer)
	log.SetLevel(logrus.WarnLevel)
	publisher := NewEventPublisher(log)
	publisher.Subscribe(func(e *args) {
		t.Error("should not be called")
	})
	publisher.Publish(&args2{
		data: "test",
	})

	if output := logBuffer.String(); output == "" {
		t.Error("should have logged")
	} else if !strings.Contains(output, "eventbus.Publish: no matching subscribers") {
		t.Errorf("should have contained no matching subscribers but got: %q", output)
	}
}

func TestPublisher_Subscribe(t *testing.T) {
	publisher := NewEventPublisher(logging.ConsoleLogger(logrus.WarnLevel))
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

func TestMatchSignature(t *testing.T) {
	type args struct {
	}
	type args2 struct {
	}
	if !MatchSignature(func(e *args) {}, []interface{}{&args{}}) {
		t.Error("expected true")
	}
	if MatchSignature(func(e *args) {}, []interface{}{&args2{}}) {
		t.Error("expected false")
	}
	if MatchSignature(func(e *args) {}, []interface{}{}) {
		t.Error("expected false")
	}
	if MatchSignature(func(e *args) {}, []interface{}{&args{}, &args{}}) {
		t.Error("expected false")
	}

	if !MatchSignature(func(ctx context.Context) {}, []interface{}{context.Background()}) {
		t.Error("expected true")
	}
}
