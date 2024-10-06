package inventory

type StatusEnum string

const (
	Success    StatusEnum = "success"
	Incomplete StatusEnum = "incomplete"
	Failed     StatusEnum = "failed"
)

func (s StatusEnum) IsValid() bool {
	return s == Success || s == Incomplete || s == Failed
}

func NewStatus(value string) (*Status, error) {
	status := StatusEnum(value)
	if !status.IsValid() {
		return nil, InvalidStatus
	}
	return &Status{value: status}, nil
}

type Status struct {
	value StatusEnum
}

func (s Status) Get() StatusEnum {
	return s.value
}

func (s Status) String() string {
	return string(s.value)
}
