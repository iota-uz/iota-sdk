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
		payload, err := json.Marshal(event)
		if err != nil {
			_, _ = fmt.Fprintf(out, "[error] marshal event: %v\n", err)
			return
		}
		_, _ = fmt.Fprintln(out, string(payload))
		return
	}
	var line string
	switch {
	case event.StepID != "" && event.Message != "":
		line = fmt.Sprintf("[%s] (%s) %s", event.Type, event.StepID, event.Message)
	case event.StepID != "":
		line = fmt.Sprintf("[%s] (%s)", event.Type, event.StepID)
	case event.Message != "":
		line = fmt.Sprintf("[%s] %s", event.Type, event.Message)
	default:
		line = fmt.Sprintf("[%s]", event.Type)
	}
	_, _ = fmt.Fprintln(out, line)
}
