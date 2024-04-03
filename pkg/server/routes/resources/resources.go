package resources

import (
	"context"
	"database/sql"
	"github.com/apollos-studio/sso/models"
	"github.com/apollos-studio/sso/pkg/server/helpers"
	"github.com/apollos-studio/sso/pkg/server/routes"
	"github.com/apollos-studio/sso/templates/pages/resources"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"net/http"
)

type Route struct {
	Db *sqlx.DB
}

func (i *Route) Prefix() string {
	return "/resources"
}

func (i *Route) Setup(router *mux.Router, opts *routes.Options) {
	i.Db = opts.Db
	router.Use(helpers.RequireAuthMiddleware)
	router.HandleFunc("/", i.Get).Methods(http.MethodGet)
	router.HandleFunc("/{id}", i.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/{id}", i.FormSubmit).Methods(http.MethodPost)
}

func (i *Route) Get(w http.ResponseWriter, r *http.Request) {
	var resources2 []*models.Resource
	err := i.Db.Select(&resources2, "SELECT * FROM resources")
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	data := &resources.IndexData{
		Resources: resources2,
	}
	if err := resources.Index(data).Render(context.Background(), w); err != nil {
		helpers.ServerError(w, err)
		return
	}
}

func (i *Route) GetOne(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	resource := &models.Resource{}
	if id != "new" {
		err := i.Db.Get(resource, "SELECT * FROM resources WHERE id = $1", id)
		if err != nil {
			helpers.ServerError(w, err)
			return
		}
	}
	data := &resources.EditData{
		Resource: resource,
	}
	if err := resources.EditPage(data).Render(context.Background(), w); err != nil {
		helpers.ServerError(w, err)
		return
	}
}

func (i *Route) FormSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	vars := mux.Vars(r)
	id := vars["id"]
	resource := &models.Resource{}
	if id != "new" {
		err := i.Db.Get(resource, "SELECT * FROM resources WHERE id = $1", id)
		if err != nil {
			helpers.ServerError(w, err)
			return
		}
	}
	resource.Id = r.FormValue("id")
	resource.Description = models.JsonNullString{
		NullString: sql.NullString{
			String: r.FormValue("description"),
			Valid:  true,
		},
	}
	if errs := resource.Validate(); len(errs) != 0 {
		response := map[string][]*models.ValidationError{"errors": errs}
		helpers.RespondWithJson(w, http.StatusUnprocessableEntity, response)
		return
	}
	if err := resource.Save(i.Db); err != nil {
		helpers.ServerError(w, err)
		return
	}
	http.Redirect(w, r, "/resources", http.StatusFound)
}
