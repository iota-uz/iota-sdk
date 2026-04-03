package templ

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

func RenderTableChunk(w http.ResponseWriter, r *http.Request, result *runtime.Result, panelID string) bool {
	panelResult := result.Panel(panelID)
	if panelResult == nil {
		return false
	}
	templ.Handler(TablePanelChunk(panelResult)).ServeHTTP(w, r)
	return true
}
