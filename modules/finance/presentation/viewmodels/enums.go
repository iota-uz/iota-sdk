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
	IndividualType  CounterpartyLegalType = "INDIVIDUAL"
	LegalEntityType CounterpartyLegalType = "LEGAL_ENTITY"
	LLCType         CounterpartyLegalType = "LLC"
	JSCType         CounterpartyLegalType = "JSC"
	INCType         CounterpartyLegalType = "INC"
	LTDType         CounterpartyLegalType = "LTD"
	PLCType         CounterpartyLegalType = "PLC"
	LLPType         CounterpartyLegalType = "LLP"
	GMBHType        CounterpartyLegalType = "GMBH"
	AGType          CounterpartyLegalType = "AG"
	SAType          CounterpartyLegalType = "SA"
	PTYLTDType      CounterpartyLegalType = "PTYLTD"
	CCORPType       CounterpartyLegalType = "CCORP"
	SCORPType       CounterpartyLegalType = "SCORP"
	SPType          CounterpartyLegalType = "SP"
	SCType          CounterpartyLegalType = "SC"
	OUType          CounterpartyLegalType = "OU"
	ABType          CounterpartyLegalType = "AB"
	ASType          CounterpartyLegalType = "AS"
	SARLType        CounterpartyLegalType = "SARL"
	BVType          CounterpartyLegalType = "BV"
	KKType          CounterpartyLegalType = "KK"
	SAOType         CounterpartyLegalType = "SAO"
	LLLPType        CounterpartyLegalType = "LLLP"
	UABType         CounterpartyLegalType = "UAB"
	SPZOOType       CounterpartyLegalType = "SPZOO"
	SRLType         CounterpartyLegalType = "SRL"
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
		LegalEntityType,
		LLCType,
		JSCType,
		INCType,
		LTDType,
		PLCType,
		LLPType,
		GMBHType,
		AGType,
		SAType,
		PTYLTDType,
		CCORPType,
		SCORPType,
		SPType,
		SCType,
		OUType,
		ABType,
		ASType,
		SARLType,
		BVType,
		KKType,
		SAOType,
		LLLPType,
		UABType,
		SPZOOType,
		SRLType,
	}
}

func (t CounterpartyType) String() string {
	return string(t)
}

func (t CounterpartyType) LocalizedString(pageCtx *types.PageContext) string {
	key := "Counterparties.Types." + string(t)
	return pageCtx.T(key)
}

func (t CounterpartyType) ToDomain() counterparty.Type {
	return counterparty.Type(t)
}

func CounterpartyTypeFromDomain(domainType counterparty.Type) CounterpartyType {
	return CounterpartyType(domainType)
}

func (l CounterpartyLegalType) String() string {
	return string(l)
}

func (l CounterpartyLegalType) LocalizedString(pageCtx *types.PageContext) string {
	key := "Counterparties.LegalTypes." + string(l)
	return pageCtx.T(key)
}

func (l CounterpartyLegalType) ToDomain() counterparty.LegalType {
	return counterparty.LegalType(l)
}

func CounterpartyLegalTypeFromDomain(domainType counterparty.LegalType) CounterpartyLegalType {
	return CounterpartyLegalType(domainType)
}
