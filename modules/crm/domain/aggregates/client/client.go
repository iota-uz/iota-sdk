package client

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/general"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
)

type Option func(c *client)

// --- Option setters ---

func WithID(id uint) Option {
	return func(c *client) {
		c.id = id
	}
}

func WithLastName(lastName string) Option {
	return func(c *client) {
		c.lastName = lastName
	}
}

func WithMiddleName(middleName string) Option {
	return func(c *client) {
		c.middleName = middleName
	}
}

func WithAddress(address string) Option {
	return func(c *client) {
		c.address = address
	}
}

func WithEmail(email internet.Email) Option {
	return func(c *client) {
		c.email = email
	}
}

func WithDateOfBirth(dob *time.Time) Option {
	return func(c *client) {
		c.dateOfBirth = dob
	}
}

func WithPassport(p passport.Passport) Option {
	return func(c *client) {
		c.passport = p
	}
}

func WithPin(pin tax.Pin) Option {
	return func(c *client) {
		c.pin = pin
	}
}

func WithComments(comments string) Option {
	return func(c *client) {
		c.comments = comments
	}
}

func WithGender(g general.Gender) Option {
	return func(c *client) {
		c.gender = g
	}
}

func WithPhone(phone phone.Phone) Option {
	return func(c *client) {
		c.phone = phone
	}
}

func WithContacts(contacts []Contact) Option {
	return func(c *client) {
		c.contacts = contacts
	}
}

func WithCreatedAt(t time.Time) Option {
	return func(c *client) {
		c.createdAt = t
	}
}

func WithUpdatedAt(t time.Time) Option {
	return func(c *client) {
		c.updatedAt = t
	}
}

// --- Interface ---

type ContactType string

const (
	ContactTypeEmail    ContactType = "email"
	ContactTypePhone    ContactType = "phone"
	ContactTypeTelegram ContactType = "telegram"
	ContactTypeWhatsApp ContactType = "whatsapp"
	ContactTypeOther    ContactType = "other"
)

type Client interface {
	ID() uint
	FirstName() string
	LastName() string
	MiddleName() string
	Phone() phone.Phone
	Address() string
	Email() internet.Email
	DateOfBirth() *time.Time
	Gender() general.Gender
	Passport() passport.Passport
	Pin() tax.Pin
	Comments() string
	Contacts() []Contact
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetContacts(contacts []Contact) Client
	AddContact(contact Contact) Client
	RemoveContact(contactID uint) Client
	SetPhone(number phone.Phone) Client
	SetName(firstName, lastName, middleName string) Client
	SetAddress(address string) Client
	SetEmail(email internet.Email) Client
	SetDateOfBirth(dob *time.Time) Client
	SetGender(gender general.Gender) Client
	SetPassport(p passport.Passport) Client
	SetPIN(pin tax.Pin) Client
	SetComments(comments string) Client
}

type Contact interface {
	ID() uint
	Type() ContactType
	Value() string
	CreatedAt() time.Time
	UpdatedAt() time.Time
}

// --- Constructor ---

func New(firstName string, opts ...Option) (Client, error) {
	c := &client{
		firstName: firstName,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

type client struct {
	id          uint
	firstName   string
	lastName    string
	middleName  string
	phone       phone.Phone
	address     string
	email       internet.Email
	dateOfBirth *time.Time
	gender      general.Gender
	passport    passport.Passport
	pin         tax.Pin
	comments    string
	contacts    []Contact
	createdAt   time.Time
	updatedAt   time.Time
}

func (c *client) ID() uint {
	return c.id
}

func (c *client) FirstName() string {
	return c.firstName
}

func (c *client) LastName() string {
	return c.lastName
}

func (c *client) MiddleName() string {
	return c.middleName
}

func (c *client) Phone() phone.Phone {
	return c.phone
}

func (c *client) Address() string {
	return c.address
}

func (c *client) Email() internet.Email {
	return c.email
}

func (c *client) DateOfBirth() *time.Time {
	return c.dateOfBirth
}

func (c *client) Gender() general.Gender {
	return c.gender
}

func (c *client) Passport() passport.Passport {
	return c.passport
}

func (c *client) Pin() tax.Pin {
	return c.pin
}

func (c *client) Comments() string {
	return c.comments
}

func (c *client) Contacts() []Contact {
	return c.contacts
}

func (c *client) CreatedAt() time.Time {
	return c.createdAt
}

func (c *client) UpdatedAt() time.Time {
	return c.updatedAt
}

func (c *client) SetContacts(contacts []Contact) Client {
	result := *c
	result.contacts = contacts
	result.updatedAt = time.Now()
	return &result
}

func (c *client) AddContact(contact Contact) Client {
	result := *c
	result.contacts = append(result.contacts, contact)
	result.updatedAt = time.Now()
	return &result
}

func (c *client) RemoveContact(contactID uint) Client {
	var filteredContacts []Contact
	for _, contact := range c.contacts {
		if contact.ID() != contactID {
			filteredContacts = append(filteredContacts, contact)
		}
	}

	result := *c
	result.contacts = filteredContacts
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetPhone(number phone.Phone) Client {
	result := *c
	result.phone = number
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetName(firstName, lastName, middleName string) Client {
	result := *c
	result.firstName = firstName
	result.lastName = lastName
	result.middleName = middleName
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetAddress(address string) Client {
	result := *c
	result.address = address
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetEmail(email internet.Email) Client {
	result := *c
	result.email = email
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetDateOfBirth(dob *time.Time) Client {
	result := *c
	result.dateOfBirth = dob
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetGender(gender general.Gender) Client {
	result := *c
	result.gender = gender
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetPassport(p passport.Passport) Client {
	result := *c
	result.passport = p
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetPIN(pin tax.Pin) Client {
	result := *c
	result.pin = pin
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetComments(comments string) Client {
	result := *c
	result.comments = comments
	result.updatedAt = time.Now()
	return &result
}

// ContactOption is a function that configures a contact
type ContactOption func(*contact)

// WithContactID sets the ID of the contact
func WithContactID(id uint) ContactOption {
	return func(c *contact) {
		c.id = id
	}
}

// WithContactCreatedAt sets the created time of the contact
func WithContactCreatedAt(createdAt time.Time) ContactOption {
	return func(c *contact) {
		c.createdAt = createdAt
	}
}

// WithContactUpdatedAt sets the updated time of the contact
func WithContactUpdatedAt(updatedAt time.Time) ContactOption {
	return func(c *contact) {
		c.updatedAt = updatedAt
	}
}

// contact implements the Contact interface
type contact struct {
	id          uint
	contactType ContactType
	value       string
	createdAt   time.Time
	updatedAt   time.Time
}

// NewContact creates a new contact instance with options pattern
func NewContact(contactType ContactType, value string, opts ...ContactOption) Contact {
	now := time.Now()
	c := &contact{
		contactType: contactType,
		value:       value,
		createdAt:   now,
		updatedAt:   now,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *contact) ID() uint {
	return c.id
}

func (c *contact) Type() ContactType {
	return c.contactType
}

func (c *contact) Value() string {
	return c.value
}

func (c *contact) CreatedAt() time.Time {
	return c.createdAt
}

func (c *contact) UpdatedAt() time.Time {
	return c.updatedAt
}
