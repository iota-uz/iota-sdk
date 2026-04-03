package templ

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// RenderPanelFragment renders a single panel body using the fully prepared dashboard result.
// It returns false when either the panel result or the panel spec cannot be resolved.
func RenderPanelFragment(w http.ResponseWriter, r *http.Request, result *runtime.Result, panelID string) bool {
	if result == nil {
		return false
	}
	if result.Panel(panelID) == nil {
		return false
	}
	panelSpec, ok := lens.FindPanel(result.Spec, panelID)
	if !ok {
		return false
	}
	templ.Handler(PanelFragment(FragmentProps{
		Panel:  panelSpec,
		Result: result,
	})).ServeHTTP(w, r)
	return true
}
