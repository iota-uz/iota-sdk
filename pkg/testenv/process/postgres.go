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
	owned    map[string]string
}

func (p *Postgres) Prepare(ctx context.Context, id string) ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Template == "" {
		return nil, fmt.Errorf("process: a sealed template is required")
	}
	if p.owned == nil {
		p.owned = map[string]string{}
	}
	prefix := p.Prefix
	if prefix == "" {
		prefix = "te_"
	}
	name := prefix + strings.ReplaceAll(id, "-", "")
	if err := testdb.Clone(ctx, name, p.Template, p.Config); err != nil {
		return nil, err
	}
	p.owned[id] = name
	conn, err := sql.Open("postgres", testdb.ConnectionString(name, p.Config))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
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
	defer p.mu.Unlock()
	name, ok := p.owned[id]
	if !ok {
		return nil
	}
	if err := testdb.Drop(ctx, name, p.Config); err != nil {
		return err
	}
	delete(p.owned, id)
	return nil
}
