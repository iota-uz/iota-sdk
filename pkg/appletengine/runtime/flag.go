package runtime

import (
	"strings"

	appletsconfig "github.com/iota-uz/applets/config"
)

func EnabledForEngineConfig(engineCfg appletsconfig.AppletEngineConfig) bool {
	return strings.EqualFold(strings.TrimSpace(engineCfg.Runtime), appletsconfig.EngineRuntimeBun)
}
