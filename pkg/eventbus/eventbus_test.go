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

// TestPublisher_PanicRecovery verifies that panics in event handlers are caught and logged
func TestPublisher_PanicRecovery(t *testing.T) {
	t.Parallel()

	t.Run("handler panic is caught and logged", func(t *testing.T) {
		logBuffer := bytes.Buffer{}
		log := logrus.New()
		log.SetOutput(&logBuffer)
		log.SetLevel(logrus.ErrorLevel)

		publisher := NewEventPublisher(log)

		// Subscribe a handler that panics
		publisher.Subscribe(func(e *args) {
			panic("intentional panic for testing")
		})

		// Publish should not panic
		publisher.Publish(&args{data: "test"})

		// Verify panic was logged at error level
		output := logBuffer.String()
		if output == "" {
			t.Error("panic should have been logged")
		}
		if !strings.Contains(output, "panicked") {
			t.Errorf("log should contain 'panicked', got: %q", output)
		}
		if !strings.Contains(output, "intentional panic for testing") {
			t.Errorf("log should contain panic message, got: %q", output)
		}
	})

	t.Run("panic includes event args in log", func(t *testing.T) {
		logBuffer := bytes.Buffer{}
		log := logrus.New()
		log.SetOutput(&logBuffer)
		log.SetLevel(logrus.ErrorLevel)

		publisher := NewEventPublisher(log)

		publisher.Subscribe(func(e *args) {
			panic("test panic")
		})

		testData := &args{data: "important-data"}
		publisher.Publish(testData)

		output := logBuffer.String()
		// Verify event args are included in log for debugging
		if !strings.Contains(output, "args") {
			t.Errorf("log should include event args for debugging, got: %q", output)
		}
	})

	t.Run("multiple handlers with one panicking", func(t *testing.T) {
		logBuffer := bytes.Buffer{}
		log := logrus.New()
		log.SetOutput(&logBuffer)
		log.SetLevel(logrus.ErrorLevel)

		publisher := NewEventPublisher(log)

		called1 := false
		called2 := false

		// First handler - executes successfully
		publisher.Subscribe(func(e *args) {
			called1 = true
		})

		// Second handler - panics
		publisher.Subscribe(func(e *args) {
			panic("handler 2 panic")
		})

		// Third handler - should still execute after panic
		publisher.Subscribe(func(e *args) {
			called2 = true
		})

		publisher.Publish(&args{data: "test"})

		// Verify first handler was called
		if !called1 {
			t.Error("first handler should have been called")
		}

		// Verify third handler was called (panic shouldn't stop other handlers)
		if !called2 {
			t.Error("third handler should have been called despite second handler panic")
		}

		// Verify panic was logged
		output := logBuffer.String()
		if !strings.Contains(output, "panicked") {
			t.Errorf("panic should have been logged, got: %q", output)
		}
	})

	t.Run("no matching subscribers warning when all handlers panic", func(t *testing.T) {
		logBuffer := bytes.Buffer{}
		log := logrus.New()
		log.SetOutput(&logBuffer)
		log.SetLevel(logrus.WarnLevel)

		publisher := NewEventPublisher(log)

		// Subscribe handler that panics
		publisher.Subscribe(func(e *args) {
			panic("always panics")
		})

		publisher.Publish(&args{data: "test"})

		output := logBuffer.String()
		// When all matching handlers panic, should log "no matching subscribers"
		if !strings.Contains(output, "no matching subscribers") {
			t.Errorf("should warn about no matching subscribers when all panic, got: %q", output)
		}
	})

	t.Run("handled correctly when some handlers panic", func(t *testing.T) {
		logBuffer := bytes.Buffer{}
		log := logrus.New()
		log.SetOutput(&logBuffer)
		log.SetLevel(logrus.WarnLevel)

		publisher := NewEventPublisher(log)

		called := false

		// First handler panics
		publisher.Subscribe(func(e *args) {
			panic("first handler panic")
		})

		// Second handler succeeds
		publisher.Subscribe(func(e *args) {
			called = true
		})

		publisher.Publish(&args{data: "test"})

		// Verify successful handler was called
		if !called {
			t.Error("successful handler should have been called")
		}

		output := logBuffer.String()
		// Should NOT log "no matching subscribers" since one handler succeeded
		if strings.Contains(output, "no matching subscribers") {
			t.Error("should not warn about no matching subscribers when at least one handler succeeds")
		}
	})
}

// TestPublisher_PanicWithNilHandler verifies behavior with edge cases
func TestPublisher_PanicWithNilHandler(t *testing.T) {
	t.Parallel()

	t.Run("handler panics with nil argument", func(t *testing.T) {
		logBuffer := bytes.Buffer{}
		log := logrus.New()
		log.SetOutput(&logBuffer)
		log.SetLevel(logrus.ErrorLevel)

		publisher := NewEventPublisher(log)

		publisher.Subscribe(func(e *args) {
			// Accessing nil will cause panic
			_ = e.data.(string)
		})

		// Publish with nil should be handled gracefully
		publisher.Publish(&args{data: nil})

		// Should recover and log panic
		output := logBuffer.String()
		if !strings.Contains(output, "panicked") {
			t.Errorf("nil argument panic should be caught, got: %q", output)
		}
	})
}
