package inventory

type User interface {
	ID() uint
	FirstName() string
	LastName() string
}
