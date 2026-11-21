package hrm

import (
	"github.com/iota-uz/iota-sdk/modules/hrm/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// Natives is the HRM module's native registration for the extension runtime:
// the hrm.employee entity and its events (services.Natives). Applications
// compose it into their native.Set (modules.Natives).
func Natives(s *native.Set) error { return services.Natives(s) }
