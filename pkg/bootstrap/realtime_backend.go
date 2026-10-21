package bootstrap

import (
	"context"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/redisconfig"
	"github.com/iota-uz/iota-sdk/pkg/realtime"
	"github.com/iota-uz/iota-sdk/pkg/realtime/memory"
	"github.com/iota-uz/iota-sdk/pkg/realtime/redisfanout"
	"github.com/sirupsen/logrus"
)

func newRealtimeBackend(ctx context.Context, cfg *redisconfig.Config, logger *logrus.Logger) (realtime.Backend, error) {
	if !cfg.IsConfigured() {
		return memory.New(), nil
	}
	redisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return redisfanout.New(redisCtx, cfg.URL, logger)
}
