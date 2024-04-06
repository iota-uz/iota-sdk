package users

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/pkg/server/helpers"
	"github.com/iota-agency/iota-erp/pkg/server/routes"
	"github.com/jmoiron/sqlx"
	"net/http"
)

type ApiRoute struct {
	Db *sqlx.DB
}

func (u *ApiRoute) Prefix() string {
	return "/rpc"
}

type RpcCall struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type RpcResponse struct {
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

func (u *ApiRoute) Setup(router *mux.Router, opts *routes.Options) {
	u.Db = opts.Db
	router.HandleFunc("/", u.Post).Methods(http.MethodPost)
}

func (u *ApiRoute) Post(w http.ResponseWriter, r *http.Request) {
	data := RpcCall{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		helpers.BadRequest(w, err)
		return
	}

	switch data.Method {
	case "DoSomething":
		// Do something
		helpers.RespondWithJson(w, http.StatusOK, RpcResponse{Result: "Something done"})
	default:
		helpers.RespondWithJson(w, http.StatusMethodNotAllowed, RpcResponse{
			Error: "Method not found",
		})
	}
}
