package viewmodels

// Revenue is one currency of what is due from a client, split by basis.
type Revenue struct {
	CurrencyCode string
	Contract     string
	Accepted     string
	Open         string
	Invoiced     string
	Paid         string
	// Overaccepted is set when signed documents exceed the contract.
	Overaccepted bool
}
