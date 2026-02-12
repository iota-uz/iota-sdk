package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnabledForApplet(t *testing.T) {
	t.Setenv("IOTA_APPLET_ENGINE_BICHAT", "bun")
	assert.True(t, EnabledForApplet("bichat"))

	t.Setenv("IOTA_APPLET_ENGINE_BICHAT", "BUN")
	assert.True(t, EnabledForApplet("bichat"))

	t.Setenv("IOTA_APPLET_ENGINE_BICHAT", "go")
	assert.False(t, EnabledForApplet("bichat"))

	t.Setenv("IOTA_APPLET_ENGINE_BICHAT", "")
	assert.False(t, EnabledForApplet("bichat"))
	assert.False(t, EnabledForApplet(""))
}
