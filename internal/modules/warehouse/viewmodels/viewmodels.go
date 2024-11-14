package viewmodels

type Product struct {
	ID         string
	PositionID string
	Rfid       string
	Status     string
	CreatedAt  string
	UpdatedAt  string
}

type Position struct {
	ID        string
	Title     string
	Barcode   string
	UnitID    string
	CreatedAt string
	UpdatedAt string
}
