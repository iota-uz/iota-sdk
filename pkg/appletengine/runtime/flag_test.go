package runtime

import (
	"testing"

	appletsconfig "github.com/iota-uz/applets/config"
	"github.com/stretchr/testify/assert"
)

func TestEnabledForEngineConfig(t *testing.T) {
	assert.True(t, EnabledForEngineConfig(appletsconfig.AppletEngineConfig{
		Runtime: appletsconfig.EngineRuntimeBun,
	}))
	assert.True(t, EnabledForEngineConfig(appletsconfig.AppletEngineConfig{
		Runtime: "BUN",
	}))
	assert.False(t, EnabledForEngineConfig(appletsconfig.AppletEngineConfig{
		Runtime: appletsconfig.EngineRuntimeOff,
	}))
	assert.False(t, EnabledForEngineConfig(appletsconfig.AppletEngineConfig{}))
}
