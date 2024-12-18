package positionservice

import "slices"

func uniqueUnits(rows []*XlsRow) []string {
	var units []string
	for _, row := range rows {
		if !slices.Contains(units, row.Unit) {
			units = append(units, row.Unit)
		}
	}
	return units
}
