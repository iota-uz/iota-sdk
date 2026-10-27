package middleware

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/agentsession"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
)

func isAgentSession(_ context.Context, container *composition.Container, sess session.Session) bool {
	cfg, err := composition.Resolve[*appconfig.Config](container)
	return err == nil && agentsession.Is(sess, cfg.Environment)
}
