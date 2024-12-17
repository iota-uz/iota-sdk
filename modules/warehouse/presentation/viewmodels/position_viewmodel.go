package viewmodels

import "github.com/iota-agency/iota-sdk/pkg/presentation/viewmodels"

type Position struct {
	ID        string
	Title     string
	Barcode   string
	UnitID    string
	Unit      Unit
	Images    []*viewmodels.Upload
	CreatedAt string
	UpdatedAt string
}
