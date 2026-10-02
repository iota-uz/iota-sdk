package process

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/testdb"
	"github.com/iota-uz/iota-sdk/pkg/testenv"
)

type Postgres struct {
	Config   dbconfig.Config
	Template string
	Prefix   string
	Manifest testdb.Manifest
	mu       sync.Mutex
	owned    map[string]*database
}

type database struct {
	mu      sync.Mutex
	name    string
	created bool
}

func (p *Postgres) Prepare(ctx context.Context, id string) ([]string, error) {
	if p.Template == "" {
		return nil, fmt.Errorf("process: a sealed template is required")
	}
	prefix := p.Prefix
	if prefix == "" {
		prefix = "te_"
	}
	name := prefix + strings.ReplaceAll(id, "-", "")
	p.mu.Lock()
	if p.owned == nil {
		p.owned = map[string]*database{}
	}
	if _, exists := p.owned[id]; exists {
		p.mu.Unlock()
		return nil, fmt.Errorf("process: database already owned")
	}
	db := &database{name: name}
	db.mu.Lock()
	p.owned[id] = db
	p.mu.Unlock()
	defer db.mu.Unlock()
	if err := testdb.Clone(ctx, name, p.Template, p.Config); err != nil {
		p.mu.Lock()
		delete(p.owned, id)
		p.mu.Unlock()
		return nil, err
	}
	db.created = true
	conn, err := sql.Open("postgres", testdb.ConnectionString(name, p.Config))
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	actual, err := testdb.ReadManifest(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("baseline manifest: %w", err)
	}
	if actual != p.Manifest {
		return nil, &testenv.Error{Code: "baseline_mismatch", Message: "baseline manifest differs from required fingerprints or revision"}
	}
	return []string{"DB_NAME=" + name, "DB_HOST=" + p.Config.Host, "DB_PORT=" + p.Config.Port, "DB_USER=" + p.Config.User, "DB_PASSWORD=" + p.Config.Password}, nil
}

func (p *Postgres) Dispose(ctx context.Context, id string) error {
	p.mu.Lock()
	db, ok := p.owned[id]
	p.mu.Unlock()
	if !ok {
		return nil
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.created {
		if err := testdb.Drop(ctx, db.name, p.Config); err != nil {
			return err
		}
		db.created = false
	}
	p.mu.Lock()
	if p.owned[id] == db {
		delete(p.owned, id)
	}
	p.mu.Unlock()
	return nil
}
