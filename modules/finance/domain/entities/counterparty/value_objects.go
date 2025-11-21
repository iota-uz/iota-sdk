package counterparty

import "errors"

type Type string
type LegalType string

const (
	Customer Type = "CUSTOMER"
	Supplier Type = "SUPPLIER"
	Both     Type = "BOTH"
	Other    Type = "_OTHER"
)

const (
	Individual         LegalType = "INDIVIDUAL"          // Individual
	LLC                LegalType = "LLC"                 // Limited Liability Company
	JSC                LegalType = "JSC"                 // Joint Stock Company
	SoleProprietorship LegalType = "SOLE_PROPRIETORSHIP" // Sole Proprietorship
)

func NewType(t string) (Type, error) {
	typ := Type(t)
	if !typ.IsValid() {
		return "", errors.New("invalid type")
	}
	return typ, nil
}

func (t Type) IsValid() bool {
	switch t {
	case Customer, Supplier, Both, Other:
		return true
	}
	return false
}

func ParseType(s string) (Type, error) {
	return NewType(s)
}

func NewLegalType(l string) (LegalType, error) {
	legalType := LegalType(l)
	if !legalType.IsValid() {
		return "", errors.New("invalid legal type")
	}
	return legalType, nil
}

func (l LegalType) IsValid() bool {
	switch l {
	case Individual, LLC, JSC, SoleProprietorship:
		return true
	}
	return false
}

func ParseLegalType(s string) (LegalType, error) {
	return NewLegalType(s)
}
