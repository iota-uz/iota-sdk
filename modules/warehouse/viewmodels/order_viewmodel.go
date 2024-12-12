package viewmodels

type Order struct {
	ID        string
	Type      string
	Status    string
	CreatedAt string
	UpdatedAt string
}

type OrderItem struct {
	Position Position
	Quantity string
}
