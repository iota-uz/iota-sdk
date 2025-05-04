package chat

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

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

// AddMessage adds a new message to the chat
func (c *chat) SendMessage(
	content string,
	sender Member,
	attachments ...upload.Upload,
) (Message, error) {
	if content == "" && len(attachments) == 0 {
		return nil, ErrEmptyMessage
	}

	msg := NewMessage(
		content,
		sender,
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

func (c *chat) AddMember(member Member) Chat {
	res := *c
	res.members = append(c.members, member)
	return &res
}

func (c *chat) RemoveMember(memberID uuid.UUID) Chat {
	res := *c
	for i, member := range c.members {
		if member.ID() == memberID {
			res.members = append(res.members[:i], res.members[i+1:]...)
			break
		}
	}
	return &res
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
	sender Sender,
	opts ...MemberOption,
) Member {
	m := &member{
		id:        uuid.New(),
		transport: sender.Transport(),
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

func WithMessageChatID(chatID uint) MessageOption {
	return func(m *message) {
		m.chatID = chatID
	}
}

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
	sender Member,
	opts ...MessageOption,
) Message {
	m := &message{
		id:          0,
		message:     msg,
		sender:      sender,
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
	sender      Member
	isRead      bool
	readAt      *time.Time
	sentAt      *time.Time
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

func (m *message) Sender() Member {
	return m.sender
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

func (m *message) SentAt() *time.Time {
	return m.sentAt
}

// --------
// Senders
// --------

type userSender struct {
	transport Transport
	userID    uint
	firstName string
	lastName  string
}

func (s *userSender) Transport() Transport {
	return s.transport
}

func (s *userSender) Type() SenderType {
	return UserSenderType
}

func (s *userSender) UserID() uint {
	return s.userID
}

func (s *userSender) FirstName() string {
	return s.firstName
}

func (s *userSender) LastName() string {
	return s.lastName
}

type clientSender struct {
	transport Transport
	clientID  uint
	contactID uint
	firstName string
	lastName  string
}

func (s *clientSender) Transport() Transport {
	return s.transport
}

func (s *clientSender) Type() SenderType {
	return ClientSenderType
}

func (s *clientSender) ClientID() uint {
	return s.clientID
}

func (s *clientSender) ContactID() uint {
	return s.contactID
}

func (s *clientSender) FirstName() string {
	return s.firstName
}

func (s *clientSender) LastName() string {
	return s.lastName
}

// UserSender represents a message sent by a user

func NewUserSender(transport Transport, userID uint, firstName, lastName string) UserSender {
	return &userSender{
		userID:    userID,
		firstName: firstName,
		lastName:  lastName,
		transport: transport,
	}
}

func NewClientSender(
	transport Transport,
	clientID, contactID uint,
	firstName,
	lastName string,
) ClientSender {
	return &clientSender{
		clientID:  clientID,
		contactID: contactID,
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
	// If base is nil, create a default sender with OtherTransport
	if base == nil {
		base = &userSender{
			transport: OtherTransport,
		}
	}
	return &otherSender{
		Sender: base,
	}
}

type otherSender struct {
	Sender
}
