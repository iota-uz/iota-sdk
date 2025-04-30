package chat

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

// ---- Errors ----

var (
	ErrEmptyMessage = errors.New("message is empty")
)

// ---- Interfaces ----

type SenderType string

const (
	UnknownSenderType SenderType = "unknown"
	UserSenderType    SenderType = "user"
	ClientSenderType  SenderType = "client"
)

type Transport string

const (
	TelegramTransport  Transport = "telegram"
	WhatsAppTransport  Transport = "whatsapp"
	InstagramTransport Transport = "instagram"
	SMSTransport       Transport = "sms"
	EmailTransport     Transport = "email"
	PhoneTransport     Transport = "phone"
	WebsiteTransport   Transport = "website"
	OtherTransport     Transport = "other"
)

type Provider interface {
	Transport() Transport
	Send(ctx context.Context, msg Message) error
	OnReceived(callback func(msg Message) error) error
}

type Chat interface {
	ID() uint
	ClientID() uint
	Messages() []Message
	SendMessage(content string, senderID uuid.UUID, attachments ...upload.Upload) (Message, error)
	AddMessage(msg Message) Chat
	UnreadMessages() int
	MarkAllAsRead()
	Members() []Member
	MapUserToMemberID(userID uint) (uuid.UUID, error)
	MapClientToMemberID(clientID uint) (uuid.UUID, error)
	LastMessage() (Message, error)
	LastMessageAt() *time.Time
	CreatedAt() time.Time
}

type Message interface {
	ID() uint
	// TODO: rename
	SenderID() uuid.UUID
	Message() string
	IsRead() bool
	MarkAsRead()
	ReadAt() *time.Time
	Attachments() []upload.Upload
	CreatedAt() time.Time
}

type Member interface {
	ID() uuid.UUID
	Transport() Transport
	Sender() Sender
	CreatedAt() time.Time
	UpdatedAt() time.Time
}

type Sender interface {
	Transport() Transport
}

type UserSender interface {
	Sender
	UserID() uint
	FirstName() string
	LastName() string
}

type ClientSender interface {
	Sender
	ClientID() uint
	ContactID() uint
	FirstName() string
	LastName() string
}

type TelegramSender interface {
	Sender
	ChatID() int64
	Username() string
	Phone() phone.Phone
}

type WhatsAppSender interface {
	Sender
	Phone() phone.Phone
}

type InstagramSender interface {
	Sender
	Username() string
}

type SMSSender interface {
	Sender
	Phone() phone.Phone
}

type EmailSender interface {
	Sender
	Email() internet.Email
}

type PhoneSender interface {
	Sender
	Phone() phone.Phone
}

type WebsiteSender interface {
	Sender
	Phone() phone.Phone
	Email() internet.Email
}

type OtherSender interface {
	Sender
}

// ---- Chat Implementation ----

type ChatOption func(c *chat)

func WithChatID(id uint) ChatOption {
	return func(c *chat) {
		c.id = id
	}
}

func WithMessages(messages []Message) ChatOption {
	return func(c *chat) {
		c.messages = messages
	}
}

func WithLastMessageAt(lastMessageAt *time.Time) ChatOption {
	return func(c *chat) {
		c.lastMessageAt = lastMessageAt
	}
}

func WithMembers(members []Member) ChatOption {
	return func(c *chat) {
		c.members = members
	}
}

func WithCreatedAt(createdAt time.Time) ChatOption {
	return func(c *chat) {
		c.createdAt = createdAt
	}
}

func New(clientID uint, opts ...ChatOption) Chat {
	c := &chat{
		id:            0,
		clientID:      clientID,
		messages:      []Message{},
		lastMessageAt: nil,
		members:       []Member{},
		createdAt:     time.Now(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

type chat struct {
	id            uint
	clientID      uint
	messages      []Message
	lastMessageAt *time.Time
	members       []Member
	createdAt     time.Time
}

func (c *chat) ID() uint {
	return c.id
}

func (c *chat) Messages() []Message {
	return c.messages
}

func (c *chat) ClientID() uint {
	return c.clientID
}

// UnreadMessages returns the number of unread messages in the chat
func (c *chat) UnreadMessages() int {
	count := 0
	for _, msg := range c.messages {
		if !msg.IsRead() {
			count++
		}
	}
	return count
}

// MarkAllAsRead marks all messages in the chat as read
func (c *chat) MarkAllAsRead() {
	for _, msg := range c.messages {
		msg.MarkAsRead()
	}
}

func (c *chat) AddMessage(msg Message) Chat {
	res := *c
	res.messages = append(c.messages, msg)
	res.lastMessageAt = mapping.Pointer(time.Now())
	return &res
}

// MapUserToMemberID maps a user ID to a member ID in the chat
func (c *chat) MapUserToMemberID(userID uint) (uuid.UUID, error) {
	for _, member := range c.members {
		if user, ok := member.Sender().(UserSender); ok && user.UserID() == userID {
			return member.ID(), nil
		}
	}
	return uuid.Nil, errors.New("user not found in chat")
}

// MapClientToMemberID maps a client ID to a member ID in the chat
func (c *chat) MapClientToMemberID(clientID uint) (uuid.UUID, error) {
	for _, member := range c.members {
		if client, ok := member.Sender().(ClientSender); ok && client.ClientID() == clientID {
			return member.ID(), nil
		}
	}
	return uuid.Nil, errors.New("client not found in chat")
}

// AddMessage adds a new message to the chat
func (c *chat) SendMessage(
	content string,
	senderID uuid.UUID,
	attachments ...upload.Upload,
) (Message, error) {
	if content == "" && len(attachments) == 0 {
		return nil, ErrEmptyMessage
	}

	msg := NewMessage(
		content,
		senderID,
		WithAttachments(attachments),
	)

	c.messages = append(c.messages, msg)
	c.lastMessageAt = mapping.Pointer(time.Now())

	return msg, nil
}

func (c *chat) LastMessage() (Message, error) {
	if len(c.messages) == 0 {
		return nil, errors.New("no messages")
	}

	return c.messages[len(c.messages)-1], nil
}

func (c *chat) LastMessageAt() *time.Time {
	return c.lastMessageAt
}

func (c *chat) CreatedAt() time.Time {
	return c.createdAt
}

func (c *chat) Members() []Member {
	return c.members
}

// -------
// Member
// -------

type MemberOption func(m *member)

func WithMemberID(id uuid.UUID) MemberOption {
	return func(m *member) {
		m.id = id
	}
}

func WithMemberCreatedAt(createdAt time.Time) MemberOption {
	return func(m *member) {
		m.createdAt = createdAt
	}
}

func WithMemberUpdatedAt(updatedAt time.Time) MemberOption {
	return func(m *member) {
		m.updatedAt = updatedAt
	}
}

func NewMember(
	transport Transport,
	sender Sender,
	opts ...MemberOption,
) Member {
	m := &member{
		id:        uuid.New(),
		transport: transport,
		sender:    sender,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

type member struct {
	id        uuid.UUID
	transport Transport
	sender    Sender
	createdAt time.Time
	updatedAt time.Time
}

func (m *member) ID() uuid.UUID {
	return m.id
}

func (m *member) Transport() Transport {
	return m.transport
}

func (m *member) Sender() Sender {
	return m.sender
}

func (m *member) CreatedAt() time.Time {
	return m.createdAt
}

func (m *member) UpdatedAt() time.Time {
	return m.updatedAt
}

// -------
// Message
// -------

type MessageOption func(m *message)

func WithMessageID(id uint) MessageOption {
	return func(m *message) {
		m.id = id
	}
}

func WithReadAt(readAt *time.Time) MessageOption {
	return func(m *message) {
		m.readAt = readAt
	}
}

func WithAttachments(attachments []upload.Upload) MessageOption {
	return func(m *message) {
		m.attachments = attachments
	}
}

func WithMessageCreatedAt(createdAt time.Time) MessageOption {
	return func(m *message) {
		m.createdAt = createdAt
	}
}

func NewMessage(
	msg string,
	senderID uuid.UUID,
	opts ...MessageOption,
) Message {
	m := &message{
		id:          0,
		message:     msg,
		senderID:    senderID,
		isRead:      false,
		readAt:      nil,
		attachments: []upload.Upload{},
		createdAt:   time.Now(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

type message struct {
	id          uint
	chatID      uint
	message     string
	senderID    uuid.UUID
	isRead      bool
	readAt      *time.Time
	attachments []upload.Upload
	createdAt   time.Time
}

func (m *message) ID() uint {
	return m.id
}

func (m *message) ChatID() uint {
	return m.chatID
}

func (m *message) Message() string {
	return m.message
}

func (m *message) SenderID() uuid.UUID {
	return m.senderID
}

func (m *message) IsRead() bool {
	return m.isRead
}

func (m *message) MarkAsRead() {
	m.isRead = true
	m.readAt = mapping.Pointer(time.Now())
}

func (m *message) ReadAt() *time.Time {
	return m.readAt
}

func (m *message) Attachments() []upload.Upload {
	return m.attachments
}

func (m *message) CreatedAt() time.Time {
	return m.createdAt
}

// --------
// Senders
// --------

type sender struct {
	transport Transport
	type_     SenderType
	senderID  uint
	firstName string
	lastName  string
}

func (s *sender) Transport() Transport {
	return s.transport
}

func (s *sender) Type() SenderType {
	return s.type_
}

func (s *sender) SenderID() uint {
	return s.senderID
}

func (s *sender) FirstName() string {
	return s.firstName
}

func (s *sender) LastName() string {
	return s.lastName
}

// UserSender represents a message sent by a user

func NewUserSender(transport Transport, id uint, firstName, lastName string) Sender {
	return &sender{
		senderID:  id,
		type_:     UserSenderType,
		firstName: firstName,
		lastName:  lastName,
		transport: transport,
	}
}

func NewClientSender(transport Transport, id uint, firstName, lastName string) Sender {
	return &sender{
		senderID:  id,
		type_:     ClientSenderType,
		firstName: firstName,
		lastName:  lastName,
		transport: transport,
	}
}

func NewTelegramSender(base Sender, chatID int64, username string, phone phone.Phone) TelegramSender {
	return &telegramSender{
		Sender:   base,
		chatID:   chatID,
		username: username,
		phone:    phone,
	}
}

type telegramSender struct {
	Sender
	chatID   int64
	username string
	phone    phone.Phone
}

func (s *telegramSender) ChatID() int64 {
	return s.chatID
}

func (s *telegramSender) Username() string {
	return s.username
}

func (s *telegramSender) Phone() phone.Phone {
	return s.phone
}

func NewWebsiteSender(base Sender, phone phone.Phone, email internet.Email) WebsiteSender {
	return &websiteSender{
		Sender: base,
		phone:  phone,
		email:  email,
	}
}

type websiteSender struct {
	Sender
	phone phone.Phone
	email internet.Email
}

func (s *websiteSender) Phone() phone.Phone {
	return s.phone
}

func (s *websiteSender) Email() internet.Email {
	return s.email
}

// WhatsAppSender represents a message from WhatsApp
func NewWhatsAppSender(base Sender, phone phone.Phone) WhatsAppSender {
	return &whatsAppSender{
		Sender: base,
		phone:  phone,
	}
}

type whatsAppSender struct {
	Sender
	phone phone.Phone
}

func (s *whatsAppSender) Phone() phone.Phone {
	return s.phone
}

// InstagramSender represents a message from Instagram
func NewInstagramSender(base Sender, username string) InstagramSender {
	return &instagramSender{
		Sender:   base,
		username: username,
	}
}

type instagramSender struct {
	Sender
	username string
}

func (s *instagramSender) Username() string {
	return s.username
}

// SMSSender represents a message from SMS
func NewSMSSender(base Sender, phone phone.Phone) SMSSender {
	return &smsSender{
		Sender: base,
		phone:  phone,
	}
}

type smsSender struct {
	Sender
	phone phone.Phone
}

func (s *smsSender) Phone() phone.Phone {
	return s.phone
}

// EmailSender represents a message from Email
func NewEmailSender(base Sender, email internet.Email) EmailSender {
	return &emailSender{
		Sender: base,
		email:  email,
	}
}

type emailSender struct {
	Sender
	email internet.Email
}

func (s *emailSender) Email() internet.Email {
	return s.email
}

// PhoneSender represents a message from Phone call
func NewPhoneSender(base Sender, phone phone.Phone) PhoneSender {
	return &phoneSender{
		Sender: base,
		phone:  phone,
	}
}

type phoneSender struct {
	Sender
	phone phone.Phone
}

func (s *phoneSender) Phone() phone.Phone {
	return s.phone
}

// OtherSender represents a message from other sources
func NewOtherSender(base Sender) OtherSender {
	return &otherSender{
		Sender: base,
	}
}

type otherSender struct {
	Sender
}
