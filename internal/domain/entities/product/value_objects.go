package product

type StatusEnum string

const (
	InStock       StatusEnum = "in_stock"
	InDevelopment StatusEnum = "in_development"
	Approved      StatusEnum = "approved"
)

func (s StatusEnum) IsValid() bool {
	return s == InStock || s == InDevelopment || s == Approved
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

func (p Status) Get() StatusEnum {
	return p.value
}

func (p Status) String() string {
	return string(p.value)
}
