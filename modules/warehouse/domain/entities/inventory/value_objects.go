package inventory

import "fmt"

type Status string
type Type string

const (
	Success    Status = "success"
	Incomplete Status = "incomplete"
	Failed     Status = "failed"
)

func (s Status) IsValid() bool {
	return s == Success || s == Incomplete || s == Failed
}

func NewStatus(value string) (Status, error) {
	status := Status(value)
	if !status.IsValid() {
		return "", fmt.Errorf("invalid status: %s", value)
	}
	return status, nil
}
