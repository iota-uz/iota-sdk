package composition

type Descriptor struct {
	Name         string
	Capabilities []Capability
	Requires     []string
}

type Component interface {
	Descriptor() Descriptor
	Build(*Builder) error
}
