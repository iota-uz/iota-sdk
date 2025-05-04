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

var (
	ErrEmptyMessage    = errors.New("message content and attachments cannot both be empty")
	ErrNoMessages      = errors.New("chat has no messages")
	ErrSenderNotMember = errors.New("sender is not a member of this chat")
	ErrMemberNotFound  = errors.New("member not found")
)

type ChatOption func(c *chat)

func WithChatID(id uint) ChatOption {
	return func(c *chat) {
		c.id = id
	}
}

func WithMessages(messages []Message) ChatOption {
	return func(c *chat) {
		if messages != nil {
			c.messages = make([]Message, len(messages))
			copy(c.messages, messages)
		} else {
			c.messages = []Message{}
		}
	}
}

func WithMembers(members []Member) ChatOption {
	return func(c *chat) {
		for _, m := range members {
			c.members[m.ID()] = m
		}
	}
}

func WithCreatedAt(createdAt time.Time) ChatOption {
	return func(c *chat) {
		c.createdAt = createdAt
	}
}

func New(clientID uint, opts ...ChatOption) Chat {
	c := &chat{
		id:        0,
		clientID:  clientID,
		messages:  []Message{},
		members:   make(map[uuid.UUID]Member),
		createdAt: time.Now(),
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

type chat struct {
	id        uint
	clientID  uint
	messages  []Message
	members   map[uuid.UUID]Member
	createdAt time.Time
}

func (c *chat) ensureMembersMap() {
	if c.members == nil {
		c.members = make(map[uuid.UUID]Member)
	}
}

func (c *chat) copy() *chat {
	newChat := *c

	newChat.messages = make([]Message, len(c.messages))
	copy(newChat.messages, c.messages)

	newChat.members = make(map[uuid.UUID]Member, len(c.members))
	for id, member := range c.members {
		newChat.members[id] = member
	}

	return &newChat
}

func (c *chat) ID() uint {
	return c.id
}

func (c *chat) ClientID() uint {
	return c.clientID
}

func (c *chat) Messages() []Message {
	msgs := make([]Message, len(c.messages))
	copy(msgs, c.messages)
	return msgs
}

func (c *chat) UnreadMessages() int {
	count := 0
	for _, msg := range c.messages {
		if !msg.IsRead() {
			count++
		}
	}
	return count
}

func (c *chat) MarkAllAsRead() {
	for _, msg := range c.messages {
		if !msg.IsRead() {
			msg.MarkAsRead()
		}
	}
}

func (c *chat) AddMessage(msg Message) Chat {
	if msg == nil || msg.Sender() == nil {
		return c
	}

	msgSender := msg.Sender()
	currentState := c

	if !c.hasMemberByID(msgSender.ID()) {
		currentState = c.AddMember(msgSender).(*chat)
	}

	res := currentState.copy()
	res.messages = append(res.messages, msg)

	return res
}

func (c *chat) LastMessage() (Message, error) {
	if len(c.messages) == 0 {
		return nil, ErrNoMessages
	}
	return c.messages[len(c.messages)-1], nil
}

func (c *chat) LastMessageAt() *time.Time {
	if len(c.messages) > 0 {
		return c.messages[len(c.messages)-1].SentAt()
	}
	return nil
}

func (c *chat) CreatedAt() time.Time {
	return c.createdAt
}

func (c *chat) Members() []Member {
	c.ensureMembersMap()
	members := make([]Member, 0, len(c.members))
	for _, member := range c.members {
		members = append(members, member)
	}
	return members
}

func (c *chat) hasMemberByID(memberID uuid.UUID) bool {
	c.ensureMembersMap()
	_, exists := c.members[memberID]
	return exists
}

func (c *chat) AddMember(member Member) Chat {
	if member == nil {
		return c
	}
	c.ensureMembersMap()

	if _, exists := c.members[member.ID()]; exists {
		return c
	}

	res := c.copy()
	res.members[member.ID()] = member
	return res
}

func (c *chat) RemoveMember(memberID uuid.UUID) Chat {
	c.ensureMembersMap()

	if _, exists := c.members[memberID]; !exists {
		return c
	}

	res := c.copy()
	delete(res.members, memberID)
	return res
}

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
	if sender == nil {
		panic("sender cannot be nil when creating a new Member")
	}
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

func (m *member) ID() uuid.UUID        { return m.id }
func (m *member) Transport() Transport { return m.transport }
func (m *member) Sender() Sender       { return m.sender }
func (m *member) CreatedAt() time.Time { return m.createdAt }
func (m *member) UpdatedAt() time.Time { return m.updatedAt }

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
		if readAt != nil {
			ts := *readAt
			m.readAt = &ts
			m.isRead = true
		} else {
			m.readAt = nil
			m.isRead = false
		}
	}
}

func WithAttachments(attachments []upload.Upload) MessageOption {
	return func(m *message) {
		if attachments != nil {
			m.attachments = make([]upload.Upload, len(attachments))
			copy(m.attachments, attachments)
		} else {
			m.attachments = []upload.Upload{}
		}
	}
}

func WithMessageCreatedAt(createdAt time.Time) MessageOption {
	return func(m *message) {
		m.createdAt = createdAt
	}
}

func NewMessage(
	msgContent string,
	sender Member,
	opts ...MessageOption,
) Message {
	if sender == nil {
		panic("sender cannot be nil when creating a new Message")
	}
	m := &message{
		id:          0,
		message:     msgContent,
		sender:      sender,
		isRead:      false,
		readAt:      nil,
		attachments: []upload.Upload{},
		createdAt:   time.Now(),
		sentAt:      nil,
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

func (m *message) ID() uint        { return m.id }
func (m *message) ChatID() uint    { return m.chatID }
func (m *message) Sender() Member  { return m.sender }
func (m *message) Message() string { return m.message }
func (m *message) IsRead() bool    { return m.isRead }
func (m *message) MarkAsRead() {
	if !m.isRead {
		m.isRead = true
		m.readAt = mapping.Pointer(time.Now())
	}
}
func (m *message) ReadAt() *time.Time {
	if m.readAt == nil {
		return nil
	}
	t := *m.readAt
	return &t
}
func (m *message) SentAt() *time.Time {
	if m.sentAt == nil {
		return nil
	}
	t := *m.sentAt
	return &t
}
func (m *message) Attachments() []upload.Upload {
	atts := make([]upload.Upload, len(m.attachments))
	copy(atts, m.attachments)
	return atts
}
func (m *message) CreatedAt() time.Time { return m.createdAt }

type userSender struct {
	transport Transport
	userID    uint
	firstName string
	lastName  string
}

func (s *userSender) Transport() Transport { return s.transport }
func (s *userSender) Type() SenderType     { return UserSenderType }
func (s *userSender) UserID() uint         { return s.userID }
func (s *userSender) FirstName() string    { return s.firstName }
func (s *userSender) LastName() string     { return s.lastName }

type clientSender struct {
	transport Transport
	clientID  uint
	contactID uint
	firstName string
	lastName  string
}

func (s *clientSender) Transport() Transport { return s.transport }
func (s *clientSender) Type() SenderType     { return ClientSenderType }
func (s *clientSender) ClientID() uint       { return s.clientID }
func (s *clientSender) ContactID() uint      { return s.contactID }
func (s *clientSender) FirstName() string    { return s.firstName }
func (s *clientSender) LastName() string     { return s.lastName }

func NewUserSender(transport Transport, userID uint, firstName, lastName string) UserSender {
	return &userSender{
		transport: transport,
		userID:    userID,
		firstName: firstName,
		lastName:  lastName,
	}
}

func NewClientSender(transport Transport, clientID, contactID uint, firstName, lastName string) ClientSender {
	return &clientSender{
		transport: transport,
		clientID:  clientID,
		contactID: contactID,
		firstName: firstName,
		lastName:  lastName,
	}
}

type telegramSender struct {
	base     Sender
	chatID   int64
	username string
	phone    phone.Phone
}

func NewTelegramSender(base Sender, chatID int64, username string, phone phone.Phone) TelegramSender {
	if base == nil {
		panic("base sender cannot be nil for TelegramSender")
	}
	return &telegramSender{base: base, chatID: chatID, username: username, phone: phone}
}
func (s *telegramSender) Transport() Transport { return TelegramTransport }
func (s *telegramSender) Type() SenderType     { return s.base.Type() }
func (s *telegramSender) ChatID() int64        { return s.chatID }
func (s *telegramSender) Username() string     { return s.username }
func (s *telegramSender) Phone() phone.Phone   { return s.phone }

type websiteSender struct {
	base  Sender
	phone phone.Phone
	email internet.Email
}

func NewWebsiteSender(base Sender, phone phone.Phone, email internet.Email) WebsiteSender {
	if base == nil {
		panic("base sender cannot be nil for WebsiteSender")
	}
	return &websiteSender{base: base, phone: phone, email: email}
}
func (s *websiteSender) Transport() Transport  { return WebsiteTransport }
func (s *websiteSender) Type() SenderType      { return s.base.Type() }
func (s *websiteSender) Phone() phone.Phone    { return s.phone }
func (s *websiteSender) Email() internet.Email { return s.email }

type whatsAppSender struct {
	base  Sender
	phone phone.Phone
}

func NewWhatsAppSender(base Sender, phone phone.Phone) WhatsAppSender {
	if base == nil {
		panic("base sender cannot be nil for WhatsAppSender")
	}
	return &whatsAppSender{base: base, phone: phone}
}
func (s *whatsAppSender) Transport() Transport { return WhatsAppTransport }
func (s *whatsAppSender) Type() SenderType     { return s.base.Type() }
func (s *whatsAppSender) Phone() phone.Phone   { return s.phone }

type instagramSender struct {
	base     Sender
	username string
}

func NewInstagramSender(base Sender, username string) InstagramSender {
	if base == nil {
		panic("base sender cannot be nil for InstagramSender")
	}
	return &instagramSender{base: base, username: username}
}
func (s *instagramSender) Transport() Transport { return InstagramTransport }
func (s *instagramSender) Type() SenderType     { return s.base.Type() }
func (s *instagramSender) Username() string     { return s.username }

type smsSender struct {
	base  Sender
	phone phone.Phone
}

func NewSMSSender(base Sender, phone phone.Phone) SMSSender {
	if base == nil {
		panic("base sender cannot be nil for SMSSender")
	}
	return &smsSender{base: base, phone: phone}
}
func (s *smsSender) Transport() Transport { return SMSTransport }
func (s *smsSender) Type() SenderType     { return s.base.Type() }
func (s *smsSender) Phone() phone.Phone   { return s.phone }

type emailSender struct {
	base  Sender
	email internet.Email
}

func NewEmailSender(base Sender, email internet.Email) EmailSender {
	if base == nil {
		panic("base sender cannot be nil for EmailSender")
	}
	return &emailSender{base: base, email: email}
}
func (s *emailSender) Transport() Transport  { return EmailTransport }
func (s *emailSender) Type() SenderType      { return s.base.Type() }
func (s *emailSender) Email() internet.Email { return s.email }

type phoneSender struct {
	base  Sender
	phone phone.Phone
}

func NewPhoneSender(base Sender, phone phone.Phone) PhoneSender {
	if base == nil {
		panic("base sender cannot be nil for PhoneSender")
	}
	return &phoneSender{base: base, phone: phone}
}
func (s *phoneSender) Transport() Transport { return PhoneTransport }
func (s *phoneSender) Type() SenderType     { return s.base.Type() }
func (s *phoneSender) Phone() phone.Phone   { return s.phone }

type otherSender struct {
	base Sender
}

func NewOtherSender(base Sender) OtherSender {
	if base == nil {
		base = &userSender{transport: OtherTransport, userID: 0, firstName: "Unknown", lastName: "Sender"}
	}
	return &otherSender{base: base}
}
func (s *otherSender) Transport() Transport { return OtherTransport }
func (s *otherSender) Type() SenderType     { return s.base.Type() }
