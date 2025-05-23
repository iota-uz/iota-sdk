package positionservice

import (
	"log"
	"strconv"
	"strings"

	"github.com/iota-uz/utils/sequence"
	"github.com/xuri/excelize/v2"
)

type XlsRow struct {
	Title    string
	Barcode  string
	Unit     string
	Quantity int
}

func positionRowsFromFile(path string) ([]*XlsRow, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer func(f *excelize.File) {
		if err := f.Close(); err != nil {
			log.Println(err)
		}
	}(f)
	sheets := f.GetSheetList()
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	positionRows := make([]*XlsRow, 0, len(rows))
	for i, row := range rows {
		if i == 0 {
			continue
		}

		if len(row) != 4 {
			return nil, NewErrInvalidCell("D", uint(i+1))
		}
		quantity, err := strconv.Atoi(sequence.RemoveNonNumeric(row[3]))
		if err != nil {
			return nil, NewErrInvalidCell("D", uint(i+1))
		}

		title := strings.Trim(row[0], " ")
		if title == "" {
			return nil, NewErrInvalidCell("A", uint(i+1))
		}
		barcode := strings.Trim(row[1], " ")
		if barcode == "" {
			return nil, NewErrInvalidCell("B", uint(i+1))
		}
		unit := strings.Trim(strings.ToLower(row[2]), " ")
		if unit == "" {
			return nil, NewErrInvalidCell("C", uint(i+1))
		}

		positionRows = append(positionRows, &XlsRow{
			Title:    title,
			Barcode:  barcode,
			Unit:     unit,
			Quantity: quantity,
		})
	}

	return positionRows, nil
}
