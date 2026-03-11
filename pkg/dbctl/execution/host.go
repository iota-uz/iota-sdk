package execution

import (
	"context"
	"strings"

	commande2e "github.com/iota-uz/iota-sdk/pkg/commands/e2e"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/ops"
	"github.com/iota-uz/iota-sdk/pkg/dbctl/policy"
)

type Host interface {
	LookupOperation(name string) (ops.OperationSpec, error)
	ResolveTarget(ctx context.Context, operation string) (policy.Target, error)
	ControlDatabaseName(operation string) string
}

type DefaultHost struct{}

func (DefaultHost) LookupOperation(name string) (ops.OperationSpec, error) {
	return ops.Get(name)
}

func (DefaultHost) ResolveTarget(_ context.Context, operation string) (policy.Target, error) {
	conf := configuration.Use()
	target := policy.Target{
		Environment: strings.TrimSpace(conf.GoAppEnvironment),
		Host:        strings.TrimSpace(conf.Database.Host),
		Port:        strings.TrimSpace(conf.Database.Port),
		Name:        strings.TrimSpace(conf.Database.Name),
		User:        strings.TrimSpace(conf.Database.User),
	}
	switch operation {
	case "db.e2e.create", "db.e2e.drop", "db.e2e.reset", "db.e2e.migrate", "seed.e2e":
		target.Name = commande2e.E2EDBName
	}
	return target, nil
}

func (DefaultHost) ControlDatabaseName(operation string) string {
	switch operation {
	case "db.e2e.create", "db.e2e.drop", "db.e2e.reset":
		return "postgres"
	default:
		return ""
	}
}

func resolveHost(host Host) Host {
	if host != nil {
		return host
	}
	return DefaultHost{}
}
