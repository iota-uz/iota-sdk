// Package subjects names the records automations are about (a run's subject, a human task's
// subject) for the people who see them. The module that owns an entity contributes a Namer for it
// (Contribute); a screen asks the Names of the composition (Of).
package subjects

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const op = serrors.Op("automation.subjects")

// Namer names the records of one entity for a viewer. Label is the record's name ("" when the tenant
// has no such record); URL is the page of the record, set only when the viewer may read it.
type Namer interface {
	Name(ctx context.Context, viewer spi.Subject, key string) (label, url string, err error)
}

// NamerFunc adapts a function to Namer.
type NamerFunc func(ctx context.Context, viewer spi.Subject, key string) (string, string, error)

func (f NamerFunc) Name(ctx context.Context, viewer spi.Subject, key string) (string, string, error) {
	return f(ctx, viewer, key)
}

type contribution struct {
	entity string
	namer  Namer
}

// Contribute registers the namer of an entity; the factory runs once when the application is composed.
// Two namers of one entity fail the first lookup.
func Contribute(b *composition.Builder, entity string, factory func(*composition.Container) (Namer, error)) {
	if factory == nil || entity == "" {
		return
	}
	composition.Contribute(b, func(c *composition.Container) ([]contribution, error) {
		n, err := factory(c)
		if err != nil || n == nil {
			return nil, err
		}
		return []contribution{{entity: entity, namer: n}}, nil
	})
}

// Names are the namers of a composition, collected on first use.
type Names struct {
	load func() ([]contribution, error)

	once sync.Once
	by   map[string]Namer
	err  error
}

// Of is the namers contributed to the container.
func Of(c *composition.Container) *Names {
	return &Names{load: func() ([]contribution, error) { return composition.Contributions[contribution](c) }}
}

// Static is a Names of the given namers (tests, applications without a container).
func Static(by map[string]Namer) *Names {
	n := &Names{load: func() ([]contribution, error) { return nil, nil }}
	n.once.Do(func() { n.by = by })
	return n
}

// Namer is the namer of an entity.
func (n *Names) Namer(entity string) (Namer, bool, error) {
	if n == nil {
		return nil, false, nil
	}
	n.once.Do(func() {
		list, err := n.load()
		if err != nil {
			n.err = err
			return
		}
		n.by = map[string]Namer{}
		for _, c := range list {
			if _, dup := n.by[c.entity]; dup {
				n.err = fmt.Errorf("automation subjects: entity %s has two namers", c.entity)
				return
			}
			n.by[c.entity] = c.namer
		}
	})
	if n.err != nil {
		return nil, false, n.err
	}
	namer, ok := n.by[entity]
	return namer, ok, nil
}

// TableNamer names the records of a tenant table: Label is an SQL expression over the row (alias r),
// Path the page of a record. The URL is given only to a viewer of the record's tenant who holds
// ReadPermission.
type TableNamer struct {
	DB *pgxpool.Pool
	// Table is "schema.table" or "table"; KeyColumn and TenantColumn are its key and tenant.
	Table, KeyColumn, TenantColumn string
	Label                          string
	ReadPermission                 string
	Path                           func(key string) string
}

var _ Namer = TableNamer{}

// Name implements Namer.
func (t TableNamer) Name(ctx context.Context, viewer spi.Subject, key string) (string, string, error) {
	if t.DB == nil || key == "" || viewer.TenantID == uuid.Nil {
		return "", "", nil
	}
	var label string
	err := t.DB.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM %s r WHERE r.%s::text = $1 AND r.%s = $2`,
		t.Label, t.Table, t.KeyColumn, t.TenantColumn), key, viewer.TenantID).Scan(&label)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", serrors.FromDB(op, err)
	}
	label = strings.TrimSpace(label)
	if t.Path == nil || t.ReadPermission == "" || !viewer.Has(t.ReadPermission) {
		return label, "", nil
	}
	return label, t.Path(key), nil
}
