package position

import "fmt"

type Status string

const (
	StatusAvailable   Status = "AVAILABLE"
	StatusReserved    Status = "RESERVED"
	StatusOutOfStock  Status = "OUT_OF_STOCK"
	StatusBackordered Status = "BACKORDERED"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusAvailable, StatusReserved, StatusOutOfStock, StatusBackordered:
		return true
	default:
		return false
	}
}

func (s Status) String() string {
	return string(s)
}

func ParseStatus(s string) (Status, error) {
	status := Status(s)
	if !status.IsValid() {
		return "", fmt.Errorf("invalid position status: %s", s)
	}
	return status, nil
}
