package tax

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
)

// Tin - Taxpayer Identification Number (ИНН - Идентификационный номер налогоплательщика)
type Tin interface {
	Value() string
	Country() country.Country
}

// Pin - Personal Identification Number (ПИНФЛ - Персональный идентификационный номер физического лица)
type Pin interface {
	Value() string
	Country() country.Country
}
