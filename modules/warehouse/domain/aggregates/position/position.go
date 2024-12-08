package position

import (
	"errors"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/upload"
	"github.com/iota-agency/iota-sdk/pkg/utils/sequence"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidRow = errors.New("invalid row")
)

type Position struct {
	ID        uint
	Title     string
	Barcode   string
	UnitID    uint
	Unit      unit.Unit
	Images    []upload.Upload
	CreatedAt time.Time
	UpdatedAt time.Time
}

type XlsRow struct {
	Title    string
	Barcode  string
	Unit     string
	Quantity int
}

func XlsRowFromStrings(row []string) (*XlsRow, error) {
	if len(row) != 4 {
		return nil, ErrInvalidRow
	}
	quantity, err := strconv.Atoi(sequence.RemoveNonNumeric(row[3]))
	if err != nil {
		return nil, err
	}
	return &XlsRow{
		Title:    strings.Trim(row[0], " "),
		Barcode:  strings.Trim(row[1], " "),
		Unit:     strings.Trim(strings.ToLower(row[2]), " "),
		Quantity: quantity,
	}, nil
}
