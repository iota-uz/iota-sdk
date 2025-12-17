package viewmodels

import (
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

type CounterpartyType string
type CounterpartyLegalType string

const (
	CustomerType CounterpartyType = "CUSTOMER"
	SupplierType CounterpartyType = "SUPPLIER"
	BothType     CounterpartyType = "BOTH"
	OtherType    CounterpartyType = "_OTHER"
)

const (
	IndividualType         CounterpartyLegalType = "INDIVIDUAL"
	LLCType                CounterpartyLegalType = "LLC"
	JSCType                CounterpartyLegalType = "JSC"
	SoleProprietorshipType CounterpartyLegalType = "SOLE_PROPRIETORSHIP"
)

func AllCounterpartyTypes() []CounterpartyType {
	return []CounterpartyType{
		CustomerType,
		SupplierType,
		BothType,
		OtherType,
	}
}

func AllCounterpartyLegalTypes() []CounterpartyLegalType {
	return []CounterpartyLegalType{
		IndividualType,
		LLCType,
		JSCType,
		SoleProprietorshipType,
	}
}

func (t CounterpartyType) String() string {
	return string(t)
}

func (t CounterpartyType) LocalizedString(pageCtx types.PageContextProvider) string {
	key := "Counterparties.Types." + string(t)
	return pageCtx.T(key)
}

func (t CounterpartyType) ToDomain() counterparty.Type {
	return counterparty.Type(t)
}

func CounterpartyTypeFromDomain(domainType counterparty.Type) CounterpartyType {
	return CounterpartyType(domainType)
}

func CounterpartyTypeFromString(typeStr string) CounterpartyType {
	return CounterpartyType(typeStr)
}

func (l CounterpartyLegalType) String() string {
	return string(l)
}

func (l CounterpartyLegalType) LocalizedString(pageCtx types.PageContextProvider) string {
	key := "Counterparties.LegalTypes." + string(l)
	return pageCtx.T(key)
}

func (l CounterpartyLegalType) ToDomain() counterparty.LegalType {
	return counterparty.LegalType(l)
}

func CounterpartyLegalTypeFromDomain(domainType counterparty.LegalType) CounterpartyLegalType {
	return CounterpartyLegalType(domainType)
}

func CounterpartyLegalTypeFromString(typeStr string) CounterpartyLegalType {
	return CounterpartyLegalType(typeStr)
}
