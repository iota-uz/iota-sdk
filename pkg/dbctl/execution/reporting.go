package execution

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type Event struct {
	Time      time.Time `json:"time"`
	Type      string    `json:"event_type"`
	Operation string    `json:"operation,omitempty"`
	StepID    string    `json:"step_id,omitempty"`
	Status    string    `json:"status,omitempty"`
	Message   string    `json:"message,omitempty"`
	Payload   any       `json:"payload,omitempty"`
}

func Emit(out io.Writer, jsonOutput bool, event Event) {
	if out == nil {
		return
	}
	event.Time = time.Now().UTC()
	if jsonOutput {
		payload, _ := json.Marshal(event)
		_, _ = fmt.Fprintln(out, string(payload))
		return
	}
	line := fmt.Sprintf("[%s] %s", event.Type, event.Message)
	if event.StepID != "" {
		line = fmt.Sprintf("[%s] (%s) %s", event.Type, event.StepID, event.Message)
	}
	_, _ = fmt.Fprintln(out, line)
}
