package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/automations/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// RecipientAssignees is the semantic recipient of the task notification: the users the task was just
// assigned to (user_ids of automation.task.assigned.v1).
const RecipientAssignees = "assignees"

// DefaultPath is where the inbox is mounted; a task is DefaultPath + "/" + its ID.
const DefaultPath = "/automations/tasks"

// TaskURL is the page of a task under the inbox at base (DefaultPath when empty).
func TaskURL(base string, id string) string {
	if base == "" {
		base = DefaultPath
	}
	return strings.TrimRight(base, "/") + "/" + id
}

// Reader answers whether a user may read a task: its current audience (a delegate included) and the
// holders of Automations.Read may.
type Reader interface {
	CanRead(ctx context.Context, tenantID uuid.UUID, u user.User, taskID uuid.UUID) (bool, error)
}

// NotificationConfig configures the notification of assigned tasks.
type NotificationConfig struct {
	// Reader checks, at delivery, that the recipient may still read the task; nil: no check.
	Reader Reader
	// Names names the user who delegated a task; nil: the notice says "a colleague".
	Names func(ctx context.Context, tenantID uuid.UUID, ids []int64) (map[int64]string, error)
	// Path is the inbox path (default DefaultPath).
	Path string
}

var assignedText = map[string]map[string]string{
	workflow.TaskChangeCreated: {
		"en": "You have a new task: %s", "ru": "Вам назначена задача: %s",
		"uz": "Sizga vazifa tayinlandi: %s", "uz-Cyrl": "Сизга вазифа тайинланди: %s",
	},
	workflow.TaskChangeDelegated: {
		"en": "%s passed a task to you: %s", "ru": "%s передал вам задачу: %s",
		"uz": "%s sizga vazifani topshirdi: %s", "uz-Cyrl": "%s сизга вазифани топширди: %s",
	},
	workflow.TaskChangeEscalated: {
		"en": "A task is overdue and passed to you: %s", "ru": "Задача просрочена и передана вам: %s",
		"uz": "Vazifa muddati o‘tdi va sizga topshirildi: %s", "uz-Cyrl": "Вазифа муддати ўтди ва сизга топширилди: %s",
	},
}

var colleague = map[string]string{"en": "A colleague", "ru": "Коллега", "uz": "Hamkasb", "uz-Cyrl": "Ҳамкасб"}

var dueText = map[string]string{"en": "Due %s", "ru": "Срок: %s", "uz": "Muddat: %s", "uz-Cyrl": "Муддат: %s"}

// NotificationDefinition is the notification of automation.task.assigned.v1: to the users the task was
// assigned to (by default; channels and level follow the tenant's rules and personal settings), with a
// link to the task. The text says who passed a delegated task and that an escalated one is overdue.
func NotificationDefinition(cfg NotificationConfig) notifications.Definition {
	ev := engine.TaskAssigned
	opts := []notifications.DefinitionOption{
		notifications.WithModule("automations"),
		notifications.WithLevel(notification.LevelInfo),
		notifications.WithRecipients(notifications.RecipientDefinition{
			Key: RecipientAssignees,
			Name: map[string]string{
				"en": "Task assignees", "ru": "Исполнители задачи", "uz": "Vazifa ijrochilari", "uz-Cyrl": "Вазифа ижрочилари",
			},
			Resolve: func(_ context.Context, e notifications.Event) ([]uint, error) { return assigneesOf(e) },
		}),
		notifications.WithDefaultRecipients(RecipientAssignees),
		notifications.WithRender(func(e notifications.Event, language string) (notifications.Content, error) {
			return renderAssigned(cfg, e, language), nil
		}),
	}
	if cfg.Reader != nil {
		opts = append(opts, notifications.WithGuard(func(ctx context.Context, e notifications.Event, u user.User) (bool, error) {
			id, err := uuid.Parse(e.Data["task_id"])
			if err != nil {
				return false, nil
			}
			return cfg.Reader.CanRead(ctx, e.TenantID, u, id)
		}))
	}
	return notifications.FromEvent(ev, opts...)
}

// assigneesOf reads user_ids of the payload.
func assigneesOf(e notifications.Event) ([]uint, error) {
	var ids []int64
	if raw := strings.TrimSpace(e.Data["user_ids"]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			return nil, fmt.Errorf("notification %s: invalid user_ids", e.Key)
		}
	}
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			out = append(out, uint(id))
		}
	}
	return out, nil
}

func localized(m map[string]string, language string) string {
	if s, ok := m[language]; ok {
		return s
	}
	if base, _, ok := strings.Cut(language, "-"); ok {
		if s, ok := m[base]; ok {
			return s
		}
	}
	return m["en"]
}

func renderAssigned(cfg NotificationConfig, e notifications.Event, language string) notifications.Content {
	title := strings.TrimSpace(e.Data["title"])
	reason := e.Data["reason"]
	texts, ok := assignedText[reason]
	if !ok {
		texts = assignedText[workflow.TaskChangeCreated]
	}
	var head string
	if reason == workflow.TaskChangeDelegated {
		who := localized(colleague, language)
		if id, err := strconv.ParseInt(e.Data["delegated_by"], 10, 64); err == nil && id > 0 && cfg.Names != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			names, err := cfg.Names(ctx, e.TenantID, []int64{id})
			cancel()
			if err == nil && strings.TrimSpace(names[id]) != "" {
				who = strings.TrimSpace(names[id])
			}
		}
		head = fmt.Sprintf(localized(texts, language), who, title)
	} else {
		head = fmt.Sprintf(localized(texts, language), title)
	}
	content := notifications.Content{Title: head}
	if due := strings.TrimSpace(e.Data["due_at"]); due != "" {
		if t, err := time.Parse(time.RFC3339Nano, due); err == nil {
			content.Body = fmt.Sprintf(localized(dueText, language), t.UTC().Format("2006-01-02 15:04 UTC"))
		}
	}
	if id := e.Data["task_id"]; id != "" {
		content.ActionURL = TaskURL(cfg.Path, id)
	}
	return content
}

// CanRead implements Reader: the task's current audience (the user a delegation passed it to, but not
// the one who passed it on), a user who decided it, and the holders of Automations.Read may read it.
func (s *Service) CanRead(ctx context.Context, tenantID uuid.UUID, u user.User, taskID uuid.UUID) (bool, error) {
	if u == nil {
		return false, nil
	}
	sub, err := s.cfg.Subjects.LoadSubject(ctx, tenantID, int64(u.ID()), spi.SubjectInitiator)
	if err != nil {
		return false, nil //nolint:nilerr // a user who cannot be loaded (blocked) reads nothing
	}
	if sub.Has(permissions.AutomationsRead.Name()) {
		return true, nil
	}
	if _, err := s.get(ctx, actor{user: u, subject: sub}, taskID); err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// SubjectAccess is notifications.SubjectAccess for the subject automation.task (Reader.CanRead); other
// subjects are left to next (nil: allowed). An application that sets the core's
// NotificationSubjectAccess wraps its own with it.
func SubjectAccess(r Reader, next notifications.SubjectAccess) notifications.SubjectAccess {
	return notifications.SubjectAccessFunc(func(ctx context.Context, tenantID uuid.UUID, u user.User, subject notifications.ObjectRef) (bool, error) {
		if subject.Entity == engine.SubjectTask {
			id, err := uuid.Parse(subject.Key)
			if err != nil {
				return false, nil
			}
			return r.CanRead(ctx, tenantID, u, id)
		}
		if next == nil {
			return true, nil
		}
		return next.CanAccess(ctx, tenantID, u, subject)
	})
}
