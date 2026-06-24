package pykernel

import (
	"fmt"
	"time"
)

// ExecRequest is one unit of code submitted to a kernel.
type ExecRequest struct {
	// Code is the Python source to execute in the kernel's persistent namespace.
	Code string
	// Timeout is the wall-clock cap for this exec; 0 selects the policy default.
	Timeout time.Duration
	// OutputCap is the maximum captured stdout+result size in bytes; 0 selects
	// the default (~50KB). The cap is enforced on both kernel and host side.
	OutputCap int
	// Label is a human-readable tag for tracing and observability.
	Label string
}

// ExecEventKind discriminates the streamed ExecEvent union.
type ExecEventKind int

const (
	// EventStdout carries an incremental stdout chunk.
	EventStdout ExecEventKind = iota
	// EventResult carries the converted value of the final expression.
	EventResult
	// EventMetric carries a structured progress/metric sample.
	EventMetric
	// EventLog carries a structured log line emitted via the log capability.
	EventLog
	// EventError carries an uncaught Python exception.
	EventError
	// EventDone is terminal: the exec finished (successfully or with an error).
	// Exactly one EventDone closes the stream.
	EventDone
)

func (k ExecEventKind) String() string {
	switch k {
	case EventStdout:
		return "stdout"
	case EventResult:
		return "result"
	case EventMetric:
		return "metric"
	case EventLog:
		return "log"
	case EventError:
		return "error"
	case EventDone:
		return "done"
	default:
		return fmt.Sprintf("ExecEventKind(%d)", int(k))
	}
}

// ExecEvent is one item in the stream returned by Kernel.Exec. The populated
// payload field corresponds to Kind.
type ExecEvent struct {
	Kind      ExecEventKind
	Stdout    []byte         // EventStdout
	Result    *ResultPayload // EventResult
	Metric    *MetricPayload // EventMetric
	Log       *LogPayload    // EventLog
	Err       *ExecError     // EventError
	Timestamp time.Time
}

// ResultPayload carries the value of the final expression, already converted at
// the host↔kernel boundary (datetime → ISO-8601, UUID → string, money as
// integer tiyin, Decimal → string for precision).
type ResultPayload struct {
	// Text is the text/plain repr, already output-capped.
	Text string
	// MIME describes Data, e.g. "text/plain" or "image/png" for a rendered chart.
	MIME string
	// Data is a binary artifact (e.g. a chart), or a workdir-relative reference.
	Data []byte
	// Truncated reports whether the result was clipped to the output cap.
	Truncated bool
}

// MetricPayload is a structured progress sample emitted by the kernel (e.g. the
// migration emit_metric capability).
type MetricPayload struct {
	Name  string
	Value float64
	Tags  map[string]string
}

// LogPayload is a structured log line emitted by the kernel.
type LogPayload struct {
	Level   string
	Message string
	Fields  map[string]any
}

// ExecError describes an uncaught Python exception raised during an exec.
type ExecError struct {
	Type      string
	Message   string
	Traceback string
}

func (e *ExecError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Type == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}
