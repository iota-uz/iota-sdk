package counterparty

import "errors"

type Type string
type LegalType string

const (
	Customer   Type = "CUSTOMER"
	Supplier   Type = "SUPPLIER"
	Individual Type = "INDIVIDUAL"
	Other      Type = "OTHER"
)

const (
	LLC    LegalType = "LLC"    // Limited Liability Company
	JSC    LegalType = "JSC"    // Joint Stock Company
	INC    LegalType = "INC"    // Incorporated
	LTD    LegalType = "LTD"    // Limited
	PLC    LegalType = "PLC"    // Public Limited Company
	LLP    LegalType = "LLP"    // Limited Liability Partnership
	GMBH   LegalType = "GMBH"   // Gesellschaft mit beschränkter Haftung (Germany)
	AG     LegalType = "AG"     // Aktiengesellschaft (Germany, Switzerland, Austria)
	SA     LegalType = "SA"     // Société Anonyme (France, Belgium, Spain)
	PTYLTD LegalType = "PTYLTD" // Proprietary Limited (Australia, South Africa)
	CCORP  LegalType = "CCORP"  // C Corporation (USA)
	SCORP  LegalType = "SCORP"  // S Corporation (USA)
	SP     LegalType = "SP"     // Sole Proprietorship
	SC     LegalType = "SC"     // Sociedad Colectiva (Spain, Latin America)
	OU     LegalType = "OU"     // Osaühing (Estonia)
	AB     LegalType = "AB"     // Aktiebolag (Sweden)
	AS     LegalType = "AS"     // Aksjeselskap (Norway)
	SARL   LegalType = "SARL"   // Société à Responsabilité Limitée (France, Luxembourg)
	BV     LegalType = "BV"     // Besloten Vennootschap (Netherlands)
	KK     LegalType = "KK"     // Kabushiki Kaisha (Japan)
	SAO    LegalType = "SAO"    // Societate pe Acțiuni (Romania)
	LLLP   LegalType = "LLLP"   // Limited Liability Limited Partnership (USA)
	UAB    LegalType = "UAB"    // Uždaroji Akcinė Bendrovė (Lithuania)
	SPZOO  LegalType = "SPZOO"  // Spółka z ograniczoną odpowiedzialnością (Poland)
	SRL    LegalType = "SRL"    // Sociedad de Responsabilidad Limitada (Latin America, Italy)
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
	case Customer, Supplier, Individual, Other:
		return true
	}
	return false
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
	case LLC, JSC, INC, LTD, PLC, LLP, GMBH, AG, SA, PTYLTD, CCORP, SCORP, SP, SC, OU, AB, AS, SARL, BV, KK, SAO, LLLP, UAB, SPZOO, SRL:
		return true
	}
	return false
}
