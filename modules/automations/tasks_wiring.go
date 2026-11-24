package automations

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/automations/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/graphs"
	"github.com/iota-uz/iota-sdk/modules/automations/presentation/inbox"
	"github.com/iota-uz/iota-sdk/modules/automations/tasks"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// Natives is the module's native registration: automation.task.assigned.v1, the event of a human
// node's task assigned to people (its notification is the module's). Applications compose it into
// their native.Set (modules.Natives).
func Natives(s *native.Set) error {
	return s.Add(engine.TaskAssigned)
}

// WithSubjectNames is how the inbox names the record a task is about (its label, and its link for a
// reader who may open it). Default: the label of the native entity and the engine's subject links
// (engine.Config.Links) for a reader with the entity's read permission.
func WithSubjectNames(r tasks.SubjectResolver) Option {
	return func(c *component) { c.subjectNames = r }
}

// inboxHolder is the composition's task inbox; nil without an engine.
type inboxHolder struct{ svc *tasks.Service }

// inbox is the task inbox of the composition, nil when it runs no engine.
func (c *component) inbox(container *composition.Container) (*tasks.Service, error) {
	if c.noEngine || !composition.Has[*inboxHolder](container) {
		return nil, nil //nolint:nilnil // no engine: no inbox
	}
	h, err := composition.Resolve[*inboxHolder](container)
	if err != nil {
		return nil, err
	}
	return h.svc, nil
}

// buildTasks provides the task forms and the inbox, and contributes the task observer that publishes
// automation.task.assigned.v1 and the notification of that event.
func (c *component) buildTasks(b *composition.Builder) {
	composition.Provide[*tasks.Forms](b, func(container *composition.Container) (*tasks.Forms, error) {
		return tasks.FormsOf(container), nil
	})
	if c.noEngine {
		return
	}
	composition.Provide[*inboxHolder](b, func(container *composition.Container) (*inboxHolder, error) {
		h, err := composition.Resolve[*engineHolder](container)
		if err != nil || h.rt == nil {
			return &inboxHolder{}, err
		}
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		wf, err := composition.Resolve[*workflow.Engine](container)
		if err != nil {
			return nil, err
		}
		subjects, err := composition.Resolve[host.SubjectLoader](container)
		if err != nil {
			return nil, err
		}
		var audience workflow.TaskAudience
		if composition.Has[workflow.TaskAudience](container) {
			if audience, err = composition.Resolve[workflow.TaskAudience](container); err != nil {
				return nil, err
			}
		}
		forms, err := composition.Resolve[*tasks.Forms](container)
		if err != nil {
			return nil, err
		}
		names := c.subjectNames
		if names == nil {
			var natives *native.Set
			if composition.Has[*native.Set](container) {
				if natives, err = composition.Resolve[*native.Set](container); err != nil {
					return nil, err
				}
			}
			names = entitySubjects{natives: natives, links: h.links}
		}
		svc, err := tasks.NewService(tasks.Config{
			Tasks: wf, DB: db, Audience: audience, Subjects: subjects, Users: userDirectory{names: persistence.NewUserNames(db)},
			Forms: forms, SubjectNames: names,
		})
		if err != nil {
			return nil, err
		}
		return &inboxHolder{svc: svc}, nil
	})
	wiring.ContributeTaskObservers(b, func(container *composition.Container) ([]workflow.TaskObserver, error) {
		rt, err := c.runtime(container)
		if err != nil || rt == nil {
			return nil, err
		}
		var audience workflow.TaskAudience
		if composition.Has[workflow.TaskAudience](container) {
			if audience, err = composition.Resolve[workflow.TaskAudience](container); err != nil {
				return nil, err
			}
		}
		return []workflow.TaskObserver{engine.NewTaskAssignedObserver(audience, slog.Default())}, nil
	})
	// The definition is registered once the container is built, as notifications.ContributeDefinitions
	// does; its guard reads the inbox lazily.
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		if !composition.Has[*notifications.Catalog](container) {
			return nil, nil
		}
		cat, err := composition.Resolve[*notifications.Catalog](container)
		if err != nil {
			return nil, err
		}
		reader := &lazyReader{load: func() (*tasks.Service, error) { return c.inbox(container) }}
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		users := persistence.NewUserNames(db)
		def := tasks.NotificationDefinition(tasks.NotificationConfig{Reader: reader, Names: users.Names, Path: c.tasksPath()})
		if err := cat.Register(def); err != nil {
			return nil, err
		}
		return nil, nil
	})
	// A second form of one kind fails the start of the application.
	bc := b.Context()
	if !bc.HasCapability(composition.CapabilityAPI) && !bc.HasCapability(composition.CapabilityWorker) {
		return
	}
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		return []composition.Hook{{Name: "automations.task-forms", Start: func(context.Context) (composition.StopFn, error) {
			forms, err := composition.Resolve[*tasks.Forms](container)
			if err != nil {
				return nil, err
			}
			if err := forms.Check(); err != nil {
				return nil, err
			}
			return func(context.Context) error { return nil }, nil
		}}}, nil
	})
}

// tasksPath is the inbox path under the automations path.
func (c *component) tasksPath() string {
	if c.automationsPath == "" {
		return tasks.DefaultPath
	}
	return strings.TrimRight(c.automationsPath, "/") + "/tasks"
}

// inboxController is the inbox's pages, nil without an engine.
func (c *component) inboxController(container *composition.Container) (application.Controller, error) {
	svc, err := c.inbox(container)
	if err != nil || svc == nil {
		return nil, err
	}
	forms, err := composition.Resolve[*tasks.Forms](container)
	if err != nil {
		return nil, err
	}
	path := c.automationsPath
	if path == "" {
		path = graphs.DefaultPath
	}
	return inbox.NewController(inbox.Config{
		Inbox: svc, Forms: forms, Path: c.tasksPath(), AutomationsPath: path, Origin: graphs.SessionOrigin, Logger: slog.Default(),
	})
}

// lazyReader resolves the inbox when a delivery first checks a recipient.
type lazyReader struct {
	load func() (*tasks.Service, error)
	once sync.Once
	svc  *tasks.Service
	err  error
}

func (l *lazyReader) CanRead(ctx context.Context, tenantID uuid.UUID, u user.User, taskID uuid.UUID) (bool, error) {
	l.once.Do(func() { l.svc, l.err = l.load() })
	if l.err != nil {
		return false, l.err
	}
	if l.svc == nil {
		return true, nil
	}
	return l.svc.CanRead(ctx, tenantID, u, taskID)
}

// userDirectory adapts the users table to tasks.Users.
type userDirectory struct{ names *persistence.UserNames }

func (u userDirectory) Names(ctx context.Context, tenantID uuid.UUID, ids []int64) (map[int64]string, error) {
	return u.names.Names(ctx, tenantID, ids)
}

func (u userDirectory) Search(ctx context.Context, tenantID uuid.UUID, query string, exclude []int64, limit int) ([]tasks.UserOption, error) {
	list, err := u.names.SearchActive(ctx, tenantID, query, exclude, limit)
	if err != nil {
		return nil, err
	}
	out := make([]tasks.UserOption, 0, len(list))
	for _, o := range list {
		name := o.Name
		if name == "" {
			name = o.Email
		}
		out = append(out, tasks.UserOption{UserRef: tasks.UserRef{ID: uint(o.ID), Name: name}, Hint: o.Email})
	}
	return out, nil
}

// entitySubjects names a task's record by its native entity: the entity's label (with a short numeric
// key: "Client 42"), and the engine's link to the record for a reader with the entity's read
// permission.
type entitySubjects struct {
	natives *native.Set
	links   engine.Links
}

func (e entitySubjects) Subject(ctx context.Context, viewer spi.Subject, entity, key string) (string, string, error) {
	label, readPerm := entity, ""
	if e.natives != nil {
		for _, n := range e.natives.Entities() {
			if n.ID != entity {
				continue
			}
			readPerm = n.ReadPermission
			if l := localizedLabel(n.Label, viewer.Locale); l != "" {
				label = l
			}
		}
	}
	if key != "" && len(key) <= 12 && strings.Trim(key, "0123456789") == "" {
		label += " " + key
	}
	if e.links == nil || key == "" || (readPerm != "" && !viewer.Has(readPerm)) {
		return label, "", nil
	}
	link, err := e.links.SubjectURL(ctx, viewer.TenantID, engine.SubjectRef{Entity: entity, Key: key})
	if err != nil {
		return label, "", nil //nolint:nilerr // a record without a link is named without one
	}
	return label, link, nil
}

func localizedLabel(l map[string]string, locale string) string {
	if s := l[locale]; s != "" {
		return s
	}
	if base, _, ok := strings.Cut(locale, "-"); ok && l[base] != "" {
		return l[base]
	}
	return l["en"]
}
