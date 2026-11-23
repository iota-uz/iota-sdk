package pykernel

import (
	"fmt"
	"time"
)

// ExecRequest is one unit of code submitted to a kernel.
type ExecRequest struct {
	// Code is the Python source to execute in the kernel's persistent namespace.
	Code string
	// Timeout is the wall-clock cap for this exec; 0 selects the Manager's
	// default, and a value above the Manager's cap is lowered to it.
	Timeout time.Duration
	// OutputCap is the maximum captured stdout+result size in bytes; 0 selects
	// the default (~50KB). The cap is enforced on both kernel and host side.
	OutputCap int
	// Label is a human-readable tag for tracing and observability.
	Label string
	// SeparateStderr reports stderr as EventStdout chunks flagged Stderr
	// instead of merging it into stdout.
	SeparateStderr bool
	// BindContext runs capability calls of this exec under the Exec ctx, so
	// cancelling the exec cancels the capability and ctx values reach it.
	// Otherwise calls run under the kernel's lifetime context.
	BindContext bool
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
	// EventTruncated signals the output cap was hit and a marker was inserted.
	EventTruncated
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
	case EventTruncated:
		return "truncated"
	case EventDone:
		return "done"
	default:
		return fmt.Sprintf("ExecEventKind(%d)", int(k))
	}
}

// ExecEvent is one item in the stream returned by Kernel.Exec. The populated
// payload field corresponds to Kind.
type ExecEvent struct {
	Kind   ExecEventKind
	Stdout []byte // EventStdout
	// Stderr marks an EventStdout chunk written to stderr; it is only set when
	// the exec asked for SeparateStderr.
	Stderr    bool
	Result    *ResultPayload // EventResult
	Metric    *MetricPayload // EventMetric
	Log       *LogPayload    // EventLog
	Err       *ExecError     // EventError
	Done      *DonePayload   // EventDone
	Timestamp time.Time
}

// TerminationReason says why an exec ended, so a consumer can show cancel,
// timeout, quota or a lost runtime without parsing error text.
type TerminationReason string

const (
	// ReasonCompleted: the code ran to the end without an uncaught exception.
	ReasonCompleted TerminationReason = "completed"
	// ReasonError: the code raised; the EventError carries the exception.
	ReasonError TerminationReason = "error"
	// ReasonCancelled: the Exec context was cancelled.
	ReasonCancelled TerminationReason = "cancelled"
	// ReasonTimeout: the wall-clock timeout expired.
	ReasonTimeout TerminationReason = "timeout"
	// ReasonQuotaExceeded: the exec grew the session workdir past its quota.
	ReasonQuotaExceeded TerminationReason = "quota_exceeded"
	// ReasonKernelExited: the kernel process died (CPU or memory limit, crash,
	// dispose) before the exec finished.
	ReasonKernelExited TerminationReason = "kernel_exited"
)

// DonePayload is the terminal summary of an exec. It reports which output
// streams lost data to the output cap or to a consumer that stopped reading,
// why the exec ended, and the namespace/file state it ran against, so a
// consumer can render all of it without parsing text.
type DonePayload struct {
	StdoutTruncated bool
	StderrTruncated bool
	// Reason is why the exec ended.
	Reason TerminationReason
	// EventsDropped counts events discarded because the consumer did not read
	// the stream fast enough; the bridge never blocks on a slow consumer.
	EventsDropped int
	// NamespaceReset and FilesPreserved repeat KernelInfo as it was when this
	// exec started (see KernelInfo).
	NamespaceReset bool
	FilesPreserved bool
}

// Truncated reports whether any output of the exec was lost.
func (d *DonePayload) Truncated() bool {
	return d != nil && (d.StdoutTruncated || d.StderrTruncated || d.EventsDropped > 0)
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
