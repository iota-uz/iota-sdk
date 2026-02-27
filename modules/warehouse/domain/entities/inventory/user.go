package inventory

// User is an interface that represents a user in the context of warehouse inventory.
// It is used to decouple the warehouse module from the core module's user aggregate.
type User interface {
	// ID returns the unique identifier of the user.
	ID() uint
	// FirstName returns the first name of the user.
	FirstName() string
	// LastName returns the last name of the user.
	LastName() string
}
