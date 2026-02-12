package runtime

import (
	"os"
	"strings"
)

func EnabledForApplet(appletName string) bool {
	if appletName == "" {
		return false
	}
	key := "IOTA_APPLET_ENGINE_" + strings.ToUpper(strings.ReplaceAll(appletName, "-", "_"))
	return strings.EqualFold(strings.TrimSpace(os.Getenv(key)), "bun")
}
