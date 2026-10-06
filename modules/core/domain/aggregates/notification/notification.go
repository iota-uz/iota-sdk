// Package notification defines persistent notifications for back-office users.
package notification

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("notification not found")

type Notification interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	UserID() uint
	EventKey() string
	Title() string
	Body() string
	ActionURL() string
	DedupeKey() string
	CreatedAt() time.Time
	ReadAt() *time.Time
}

type notification struct {
	id, tenantID                                uuid.UUID
	userID                                      uint
	eventKey, title, body, actionURL, dedupeKey string
	createdAt                                   time.Time
	readAt                                      *time.Time
}

type Option func(*notification)

func WithID(v uuid.UUID) Option        { return func(n *notification) { n.id = v } }
func WithTenantID(v uuid.UUID) Option  { return func(n *notification) { n.tenantID = v } }
func WithEventKey(v string) Option     { return func(n *notification) { n.eventKey = v } }
func WithActionURL(v string) Option    { return func(n *notification) { n.actionURL = v } }
func WithDedupeKey(v string) Option    { return func(n *notification) { n.dedupeKey = v } }
func WithCreatedAt(v time.Time) Option { return func(n *notification) { n.createdAt = v } }
func WithReadAt(v *time.Time) Option {
	return func(n *notification) {
		if v != nil {
			readAt := *v
			n.readAt = &readAt
		}
	}
}

func New(userID uint, title, body string, opts ...Option) (Notification, error) {
	n := &notification{
		id: uuid.New(), tenantID: uuid.Nil, userID: userID,
		title: strings.TrimSpace(title), body: body,
		eventKey: "", actionURL: "", dedupeKey: "",
		createdAt: time.Now().UTC(), readAt: nil,
	}
	for _, opt := range opts {
		opt(n)
	}
	if err := Validate(n); err != nil {
		return nil, err
	}
	return n, nil
}

func Validate(n Notification) error {
	if n == nil || n.ID() == uuid.Nil || n.UserID() == 0 || strings.TrimSpace(n.Title()) == "" {
		return errors.New("notification requires id, recipient and title")
	}
	if len(n.Title()) > 300 || len(n.Body()) > 10000 || len(n.EventKey()) > 200 || len(n.DedupeKey()) > 300 || len(n.ActionURL()) > 2000 {
		return errors.New("notification exceeds maximum field length")
	}
	if n.ActionURL() == "" {
		return nil
	}
	raw := n.ActionURL()
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return fmt.Errorf("invalid notification action URL: %w", err)
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(decoded, "/") || strings.HasPrefix(decoded, "//") || strings.Contains(decoded, "\\") || strings.IndexFunc(decoded, unicode.IsControl) >= 0 {
		return errors.New("notification action URL must be an internal absolute path")
	}
	return nil
}
func (n *notification) ID() uuid.UUID        { return n.id }
func (n *notification) TenantID() uuid.UUID  { return n.tenantID }
func (n *notification) UserID() uint         { return n.userID }
func (n *notification) EventKey() string     { return n.eventKey }
func (n *notification) Title() string        { return n.title }
func (n *notification) Body() string         { return n.body }
func (n *notification) ActionURL() string    { return n.actionURL }
func (n *notification) DedupeKey() string    { return n.dedupeKey }
func (n *notification) CreatedAt() time.Time { return n.createdAt }
func (n *notification) ReadAt() *time.Time {
	if n.readAt == nil {
		return nil
	}
	v := *n.readAt
	return &v
}

type FindParams struct {
	Limit, Offset int
	UnreadOnly    bool
}

func (p FindParams) Bounded() FindParams {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	if p.Limit > 100 {
		p.Limit = 100
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}

type Repository interface {
	Create(context.Context, Notification) (Notification, error)
	List(context.Context, uint, FindParams) ([]Notification, error)
	UnreadCount(context.Context, uint) (int64, error)
	MarkRead(context.Context, uint, uuid.UUID) error
	MarkAllRead(context.Context, uint) error
}
