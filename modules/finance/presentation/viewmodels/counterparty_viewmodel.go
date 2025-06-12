package viewmodels

type Counterparty struct {
	ID           string
	TIN          string
	Name         string
	Type         CounterpartyType
	LegalType    CounterpartyLegalType
	LegalAddress string
	CreatedAt    string
	UpdatedAt    string
}
