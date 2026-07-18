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
	return RenderPanelFragmentWithOptions(w, r, result, panelID, FragmentProps{})
}

func RenderPanelFragmentWithOptions(
	w http.ResponseWriter,
	r *http.Request,
	result *runtime.Result,
	panelID string,
	props FragmentProps,
) bool {
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
		Panel:                   panelSpec,
		Result:                  result,
		ResolvePanelErrorAction: props.ResolvePanelErrorAction,
	})).ServeHTTP(w, r)
	return true
}

// RenderExplorationFragment renders the panel returned by
// runtime.ExecuteExploration. Host applications can use it as the response for
// an explore.LoadSpec URL; the fragment wires stable point IDs back into the
// client-side explorer state machine.
func RenderExplorationFragment(w http.ResponseWriter, r *http.Request, result *runtime.ExplorationResult) bool {
	if result == nil || result.Panel == nil {
		return false
	}
	templ.Handler(ExplorationFragment(result)).ServeHTTP(w, r)
	return true
}
