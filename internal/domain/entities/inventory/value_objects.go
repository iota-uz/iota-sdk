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
	s := &Status{}
	if err := s.Set(StatusEnum(value)); err != nil {
		return nil, err
	}
	return s, nil
}

type Status struct {
	value StatusEnum
}

func (s Status) Get() StatusEnum {
	return s.value
}

func (s Status) Set(value StatusEnum) error {
	if !value.IsValid() {
		return InvalidStatus
	}
	s.value = value
	return nil
}

func (s Status) String() string {
	return string(s.value)
}
