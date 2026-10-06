// Package notifications defines events and routing rules for in-app delivery.
package notifications

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const TestEventKey = "core.notification.test.v1"

type Event struct {
	Key      string
	ID       string
	TenantID uuid.UUID
	Data     map[string]string
}

type Content struct{ Title, Body, ActionURL string }

type Definition struct {
	Module             string
	Key                string
	Name               map[string]string
	Description        map[string]string
	RequiredPermission permission.Permission
	Render             func(Event, string) (Content, error)
}

type Catalog struct {
	mu          sync.RWMutex
	definitions map[string]Definition
}

func NewCatalog() *Catalog { return &Catalog{definitions: make(map[string]Definition)} }

func (c *Catalog) Register(d Definition) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d.Key == "" || d.Render == nil || d.Name["en"] == "" {
		return fmt.Errorf("invalid notification definition")
	}
	if _, exists := c.definitions[d.Key]; exists {
		return fmt.Errorf("notification event already registered: %s", d.Key)
	}
	c.definitions[d.Key] = d
	return nil
}

func (c *Catalog) Get(key string) (Definition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.definitions[key]
	return d, ok
}
func (c *Catalog) Definitions() []Definition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Definition, 0, len(c.definitions))
	for _, d := range c.definitions {
		result = append(result, d)
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
	EventKey string
	Enabled  bool
	UserIDs  []uint
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
