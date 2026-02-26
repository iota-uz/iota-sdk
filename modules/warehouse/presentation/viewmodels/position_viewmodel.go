package viewmodels

type Position struct {
	ID        string
	Title     string
	Barcode   string
	UnitID    string
	Unit      Unit
	Images    []*Upload
	CreatedAt string
	UpdatedAt string
}
