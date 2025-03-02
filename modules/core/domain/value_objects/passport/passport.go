package passport

import "time"

type Passport interface {
	Series() string
	Number() string
	Identifier() string // Series + Number
	FirstName() string
	LastName() string
	MiddleName() string
	Gender() string
	BirthDate() time.Time
	BirthPlace() string
	Nationality() string
	PassportType() string
	IssuedAt() time.Time
	IssuedBy() string
	IssuingCountry() string
	ExpiresAt() time.Time
	MachineReadableZone() string
	BiometricData() map[string]interface{}
	SignatureImage() []byte
	Remarks() string
}

// Option is a function type that configures a passport
type Option func(*passport)

// Full Name
func WithFullName(firstName, lastName, middleName string) Option {
	return func(p *passport) {
		p.firstName = firstName
		p.lastName = lastName
		p.middleName = middleName
	}
}

// Gender
func WithGender(gender string) Option {
	return func(p *passport) {
		p.gender = gender
	}
}

// Birth Details
func WithBirthDetails(birthDate time.Time, birthPlace string) Option {
	return func(p *passport) {
		p.birthDate = birthDate
		p.birthPlace = birthPlace
	}
}

// Nationality
func WithNationality(nationality string) Option {
	return func(p *passport) {
		p.nationality = nationality
	}
}

// Passport Type
func WithPassportType(passportType string) Option {
	return func(p *passport) {
		p.passportType = passportType
	}
}

// Issued At
func WithIssuedAt(issuedAt time.Time) Option {
	return func(p *passport) {
		p.issuedAt = issuedAt
	}
}

// Issued By
func WithIssuedBy(issuedBy string) Option {
	return func(p *passport) {
		p.issuedBy = issuedBy
	}
}

// Issuing Country
func WithIssuingCountry(issuingCountry string) Option {
	return func(p *passport) {
		p.issuingCountry = issuingCountry
	}
}

// Expiration Date
func WithExpiresAt(expiresAt time.Time) Option {
	return func(p *passport) {
		p.expiresAt = expiresAt
	}
}

// Machine Readable Zone
func WithMachineReadableZone(mrz string) Option {
	return func(p *passport) {
		p.machineReadableZone = mrz
	}
}

// Biometric Data
func WithBiometricData(data map[string]interface{}) Option {
	return func(p *passport) {
		p.biometricData = data
	}
}

// Signature Image
func WithSignatureImage(signature []byte) Option {
	return func(p *passport) {
		p.signatureImage = signature
	}
}

// Remarks
func WithRemarks(remarks string) Option {
	return func(p *passport) {
		p.remarks = remarks
	}
}

// Constructor
func New(series, number string, opts ...Option) Passport {
	p := &passport{
		series: series,
		number: number,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Struct Implementation
type passport struct {
	firstName           string
	lastName            string
	middleName          string
	gender              string
	birthDate           time.Time
	birthPlace          string
	nationality         string
	passportType        string
	series              string
	number              string
	issuedAt            time.Time
	issuedBy            string
	issuingCountry      string
	expiresAt           time.Time
	machineReadableZone string
	biometricData       map[string]interface{}
	signatureImage      []byte
	remarks             string
}

// Getters Implementation
func (p *passport) Series() string {
	return p.series
}

func (p *passport) Number() string {
	return p.number
}

func (p *passport) Identifier() string {
	return p.series + p.number
}

func (p *passport) FirstName() string {
	return p.firstName
}

func (p *passport) LastName() string {
	return p.lastName
}

func (p *passport) MiddleName() string {
	return p.middleName
}

func (p *passport) Gender() string {
	return p.gender
}

func (p *passport) BirthDate() time.Time {
	return p.birthDate
}

func (p *passport) BirthPlace() string {
	return p.birthPlace
}

func (p *passport) Nationality() string {
	return p.nationality
}

func (p *passport) PassportType() string {
	return p.passportType
}

func (p *passport) IssuedAt() time.Time {
	return p.issuedAt
}

func (p *passport) IssuedBy() string {
	return p.issuedBy
}

func (p *passport) IssuingCountry() string {
	return p.issuingCountry
}

func (p *passport) ExpiresAt() time.Time {
	return p.expiresAt
}

func (p *passport) MachineReadableZone() string {
	return p.machineReadableZone
}

func (p *passport) BiometricData() map[string]interface{} {
	return p.biometricData
}

func (p *passport) SignatureImage() []byte {
	return p.signatureImage
}

func (p *passport) Remarks() string {
	return p.remarks
}
