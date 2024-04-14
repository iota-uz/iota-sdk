package users

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/server/helpers"
	"github.com/iota-agency/iota-erp/pkg/server/http/routes"
	"github.com/jmoiron/sqlx"
	"net/http"
)

type ApiRoute struct {
	Db *sqlx.DB
}

func (u *ApiRoute) Prefix() string {
	return "/api/users"
}

type PostData struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	RoleId    int64  `json:"role_id"`
}

func (u *ApiRoute) Setup(router *mux.Router, opts *routes.Options) {
	u.Db = opts.Db
	router.HandleFunc("/", u.Get).Methods(http.MethodGet)
	router.HandleFunc("/", u.Post).Methods(http.MethodPost)
}

func (u *ApiRoute) Get(w http.ResponseWriter, r *http.Request) {
	var users []*models.User
	err := u.Db.Select(&users, "SELECT * FROM users")
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, users)
}

func (u *ApiRoute) Post(w http.ResponseWriter, r *http.Request) {
	data := PostData{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	user := &models.User{
		FirstName: data.FirstName,
		LastName:  data.LastName,
		Email:     data.Email,
		Password:  data.Password,
	}
	if errs := user.Validate(); len(errs) != 0 {
		response := map[string][]*models.ValidationError{"errors": errs}
		helpers.RespondWithJson(w, http.StatusUnprocessableEntity, response)
		return
	}
	if err := user.SetPassword(user.Password); err != nil {
		helpers.ServerError(w, err)
		return
	}
	if err := user.Save(u.Db); err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, user)
}
