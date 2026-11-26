package notifications

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// subjectAccessRule is one contribution of ContributeSubjectAccess.
type subjectAccessRule struct {
	entity string
	access SubjectAccess
}

// ContributeSubjectAccess registers who may see a notice whose subject is a record of entity: the
// module that owns the entity knows its access rules. The rule decides alone for its entity (the
// application's NotificationSubjectAccess is not asked for it); a subject of an entity without a rule
// goes to the application's check. The factory runs once when the application is composed; two rules
// of one entity fail the first delivery that needs them.
func ContributeSubjectAccess(b *composition.Builder, entity string, factory func(*composition.Container) (SubjectAccess, error)) {
	if factory == nil || entity == "" {
		return
	}
	composition.Contribute(b, func(c *composition.Container) ([]subjectAccessRule, error) {
		access, err := factory(c)
		if err != nil || access == nil {
			return nil, err
		}
		return []subjectAccessRule{{entity: entity, access: access}}, nil
	})
}

// EntitySubjectAccess routes the check of a subject to the rule of its entity, and of any other
// entity to fallback (nil: allowed).
type EntitySubjectAccess struct {
	load     func() ([]subjectAccessRule, error)
	fallback SubjectAccess

	once  sync.Once
	rules map[string]SubjectAccess
	err   error
}

// NewEntitySubjectAccess is the access of the rules contributed to the container, with fallback for
// the other entities. The rules are read on the first check.
func NewEntitySubjectAccess(c *composition.Container, fallback SubjectAccess) *EntitySubjectAccess {
	return &EntitySubjectAccess{load: func() ([]subjectAccessRule, error) {
		if c == nil {
			return nil, nil
		}
		return composition.Contributions[subjectAccessRule](c)
	}, fallback: fallback}
}

// CanAccess implements SubjectAccess.
func (e *EntitySubjectAccess) CanAccess(ctx context.Context, tenantID uuid.UUID, recipient user.User, subject ObjectRef) (bool, error) {
	e.once.Do(func() {
		list, err := e.load()
		if err != nil {
			e.err = err
			return
		}
		e.rules = map[string]SubjectAccess{}
		for _, r := range list {
			if _, dup := e.rules[r.entity]; dup {
				e.err = fmt.Errorf("notification subject access: entity %s has two rules", r.entity)
				return
			}
			e.rules[r.entity] = r.access
		}
	})
	if e.err != nil {
		return false, e.err
	}
	if rule, ok := e.rules[subject.Entity]; ok {
		return rule.CanAccess(ctx, tenantID, recipient, subject)
	}
	if e.fallback == nil {
		return true, nil
	}
	return e.fallback.CanAccess(ctx, tenantID, recipient, subject)
}
