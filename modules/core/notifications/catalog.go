// Package notifications defines events and routing rules for in-app delivery.
package notifications

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const TestEventKey = "core.notification.test.v1"

var eventKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+\.v[1-9][0-9]*$`)

type SubjectReference struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
type Event struct {
	Version     int               `json:"version"`
	ActorUserID uint              `json:"actor_user_id"`
	OccurredAt  time.Time         `json:"occurred_at"`
	Subject     SubjectReference  `json:"subject"`
	DedupeKey   string            `json:"dedupe_key"`
	Key         string            `json:"key"`
	ID          string            `json:"id"`
	TenantID    uuid.UUID         `json:"tenant_id"`
	Data        map[string]string `json:"data"`
}

type Content struct{ Title, Body, ActionURL string }

type PayloadField struct {
	Key         string
	Description map[string]string
	Required    bool
}
type RecipientDefinition struct {
	Key     string
	Name    map[string]string
	Resolve func(context.Context, Event) ([]uint, error)
}
type Definition struct {
	ActionURL            func(Event) (string, error)
	DefaultLevel         notification.Level
	PayloadFields        []PayloadField
	RecipientKeys        []RecipientDefinition
	DefaultRecipientKeys []string
	ValidatePayload      func(Event) error
	RecipientGuard       func(context.Context, Event, user.User) (bool, error)
	Module               string
	Key                  string
	Name                 map[string]string
	Description          map[string]string
	RequiredPermission   permission.Permission
	Render               func(Event, string) (Content, error)
}

type Catalog struct {
	mu          sync.RWMutex
	definitions map[string]Definition
}

func NewCatalog() *Catalog { return &Catalog{definitions: make(map[string]Definition)} }

func (c *Catalog) Register(d Definition) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !eventKeyPattern.MatchString(d.Key) || d.Name["en"] == "" {
		return fmt.Errorf("invalid notification definition")
	}
	if d.DefaultLevel == "" {
		d.DefaultLevel = notification.LevelInfo
	}
	if !d.DefaultLevel.Valid() {
		return fmt.Errorf("invalid default notification level")
	}
	keys := map[string]bool{}
	for _, key := range d.RecipientKeys {
		if key.Key == "" || keys[key.Key] || key.Resolve == nil || key.Name["en"] == "" {
			return fmt.Errorf("invalid recipient definition")
		}
		keys[key.Key] = true
	}
	fields := map[string]bool{}
	for _, field := range d.PayloadFields {
		if field.Key == "" || fields[field.Key] {
			return fmt.Errorf("invalid notification payload field")
		}
		fields[field.Key] = true
	}
	defaults := map[string]bool{}
	for _, key := range d.DefaultRecipientKeys {
		if !keys[key] || defaults[key] {
			return fmt.Errorf("unknown default recipient key: %s", key)
		}
		defaults[key] = true
	}
	if _, exists := c.definitions[d.Key]; exists {
		return fmt.Errorf("notification event already registered: %s", d.Key)
	}
	if d.Render == nil {
		names := cloneDefinition(d).Name
		descriptions := cloneDefinition(d).Description
		d.Render = func(_ Event, language string) (Content, error) {
			return Content{Title: Localized(names, language), Body: Localized(descriptions, language)}, nil
		}
	}
	c.definitions[d.Key] = cloneDefinition(d)
	return nil
}

func (c *Catalog) Get(key string) (Definition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.definitions[key]
	return cloneDefinition(d), ok
}
func (c *Catalog) Definitions() []Definition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Definition, 0, len(c.definitions))
	for _, d := range c.definitions {
		result = append(result, cloneDefinition(d))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}
func Localized(values map[string]string, language string) string {
	if v := values[language]; v != "" {
		return v
	}
	return values["en"]
}

type Rule struct {
	Configured    bool               `json:"configured"`
	Level         notification.Level `json:"level"`
	RecipientKeys []string           `json:"recipient_keys"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	EventKey      string             `json:"event_key"`
	Enabled       bool               `json:"enabled"`
	UserIDs       []uint             `json:"user_ids"`
	GroupIDs      []uuid.UUID        `json:"group_ids"`
	RoleIDs       []uint             `json:"role_ids"`
}
type RuleRepository interface {
	Get(context.Context, string) (Rule, error)
	Save(context.Context, Rule) error
}

func TestDefinition() Definition {
	names := map[string]string{"en": "Test notification", "ru": "Тестовое уведомление", "uz": "Sinov bildirishnomasi", "pt-BR": "Notificação de teste", "zh": "测试通知", "uz-Cyrl": "Синов билдиришномаси"}
	bodies := map[string]string{"en": "Notifications are working. This message was sent using the configured event rule.", "ru": "Уведомления работают. Сообщение отправлено по настроенному правилу события.", "uz": "Bildirishnomalar ishlayapti. Xabar sozlangan voqea qoidasi orqali yuborildi.", "pt-BR": "As notificações estão funcionando. Esta mensagem foi enviada pela regra configurada.", "zh": "通知正常工作。此消息已按配置的事件规则发送。", "uz-Cyrl": "Билдиришномалар ишлаяпти. Хабар созланган воқеа қоидаси орқали юборилди."}
	return Definition{Module: "core", Key: TestEventKey, Name: names, Description: bodies, Render: func(_ Event, language string) (Content, error) {
		return Content{Title: Localized(names, language), Body: Localized(bodies, language), ActionURL: "/notifications"}, nil
	}}
}
func (c *Catalog) Normalize(event Event) (Event, error) {
	d, ok := c.Get(event.Key)
	if !ok {
		return event, fmt.Errorf("unknown notification event")
	}
	version, err := strconv.Atoi(event.Key[strings.LastIndex(event.Key, ".v")+2:])
	if err != nil {
		return event, err
	}
	if event.Version == 0 {
		event.Version = version
	}
	if event.Version != version || event.TenantID == uuid.Nil || strings.TrimSpace(event.ID) == "" || len(event.ID) > 200 || len(event.DedupeKey) > 200 {
		return event, fmt.Errorf("invalid notification event envelope")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if (event.Subject.Type == "") != (event.Subject.ID == "") {
		return event, fmt.Errorf("invalid notification subject")
	}
	for _, field := range d.PayloadFields {
		if field.Required && strings.TrimSpace(event.Data[field.Key]) == "" {
			return event, fmt.Errorf("missing notification payload field: %s", field.Key)
		}
	}
	if d.ActionURL != nil {
		action, err := d.ActionURL(event)
		if err != nil {
			return event, err
		}
		if _, err := notification.New(1, "validation", "", notification.WithActionURL(action)); err != nil {
			return event, err
		}
	}
	if d.ValidatePayload != nil {
		if err := d.ValidatePayload(event); err != nil {
			return event, err
		}
	}
	return event, nil
}
func (c *Catalog) ByModule(module string) []Definition {
	all := c.Definitions()
	result := make([]Definition, 0)
	for _, d := range all {
		if d.Module == module {
			result = append(result, d)
		}
	}
	return result
}
func cloneDefinition(d Definition) Definition {
	cloneMap := func(input map[string]string) map[string]string {
		output := make(map[string]string, len(input))
		for key, value := range input {
			output[key] = value
		}
		return output
	}
	d.Name = cloneMap(d.Name)
	d.Description = cloneMap(d.Description)
	d.DefaultRecipientKeys = append([]string{}, d.DefaultRecipientKeys...)
	d.PayloadFields = append([]PayloadField{}, d.PayloadFields...)
	for i := range d.PayloadFields {
		d.PayloadFields[i].Description = cloneMap(d.PayloadFields[i].Description)
	}
	d.RecipientKeys = append([]RecipientDefinition{}, d.RecipientKeys...)
	for i := range d.RecipientKeys {
		d.RecipientKeys[i].Name = cloneMap(d.RecipientKeys[i].Name)
	}
	return d
}
