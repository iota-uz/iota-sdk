package passport

type Passport interface {
	Series() string
	Number() string
	Identifier() string
}
