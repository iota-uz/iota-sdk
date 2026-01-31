package controllers

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/interop"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/templates/pages/bichat"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
)

type BiChatController struct {
	basePath        string
	app             application.Application
	dialogueService *services.DialogueService
}

func NewBiChatController(app application.Application) application.Controller {
	return &BiChatController{
		basePath:        "/bi-chat",
		app:             app,
		dialogueService: app.Service(services.DialogueService{}).(*services.DialogueService),
	}
}

func (c *BiChatController) Key() string {
	return c.basePath
}

func (c *BiChatController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.Index).Methods(http.MethodGet)
	getRouter.HandleFunc("/app", c.RenderChatApp).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("/new", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *BiChatController) Index(w http.ResponseWriter, r *http.Request) {
	props := &bichat.ChatPageProps{
		Suggestions: []string{"Hello", "World", "IOTA", "UZ"},
	}
	templ.Handler(bichat.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *BiChatController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.MessageDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = c.dialogueService.StartDialogue(r.Context(), dto.Message, "gpt-4o")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *BiChatController) Delete(w http.ResponseWriter, r *http.Request) {
	//id, err := shared.ParseID(r)
	//if err != nil {
	//	http.Error(w, "Error parsing id", http.StatusInternalServerError)
	//	return
	//}

	shared.Redirect(w, r, c.basePath)
}

// RenderChatApp renders the React SPA with injected server context
func (c *BiChatController) RenderChatApp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Build initial context for React
	initialContext, err := interop.BuildInitialContext(ctx)
	if err != nil {
		http.Error(w, "Failed to build context", http.StatusInternalServerError)
		return
	}

	// Get CSRF token from context
	csrfToken, ok := ctx.Value("gorilla.csrf.Token").(string)
	if !ok {
		csrfToken = ""
	}

	// Serialize context to JSON
	contextJSON, err := json.Marshal(initialContext)
	if err != nil {
		http.Error(w, "Failed to serialize context", http.StatusInternalServerError)
		return
	}

	// Render HTML template
	tmpl := template.Must(template.New("app").Parse(appTemplate))
	data := map[string]interface{}{
		"ContextJSON": template.JS(contextJSON),
		"CSRFToken":   csrfToken,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

const appTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>BI Chat</title>
    <script type="module" src="/bichat/assets/index.js"></script>
</head>
<body>
    <div id="app"></div>
    <script>
        window.__BICHAT_CONTEXT__ = {{.ContextJSON}};
        window.__CSRF_TOKEN__ = "{{.CSRFToken}}";
    </script>
</body>
</html>`
