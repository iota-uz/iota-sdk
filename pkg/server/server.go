package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/pkg/authentication"
	"github.com/iota-agency/iota-erp/pkg/server/helpers"
	"github.com/iota-agency/iota-erp/pkg/server/routes"
	"github.com/iota-agency/iota-erp/pkg/server/routes/users"
	"github.com/iota-agency/iota-erp/pkg/server/service"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/jmoiron/sqlx"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"time"
)

type Server struct {
	Db   *sqlx.DB
	Auth *authentication.Authentication
}

func New(db *sqlx.DB) *Server {
	return &Server{
		Db:   db,
		Auth: authentication.New(db),
	}
}

func (s *Server) handleRedirects(w http.ResponseWriter, r *http.Request, token string) {
	dest := r.URL.Query().Get("dest")
	redirectUri := r.URL.Query().Get("redirect_uri")
	cookie := http.Cookie{
		Name:     "sso-token",
		Value:    token,
		Expires:  time.Now().Add(utils.SessionDuration()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
		Domain:   utils.GetEnv("DOMAIN", "localhost"),
	}
	w.Header().Set("Set-Cookie", cookie.String())
	if dest == "cookie" {
		http.Redirect(w, r, redirectUri, http.StatusFound)
	} else if dest == "url" {
		destUrl, err := url.Parse(redirectUri)
		if err != nil {
			helpers.ServerError(w, err)
			return
		}
		q := destUrl.Query()
		q.Set("token", token)
		destUrl.RawQuery = q.Encode()
		http.Redirect(w, r, destUrl.String(), http.StatusFound)
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func (s *Server) Authorize(w http.ResponseWriter, r *http.Request) {
	token, err := r.Cookie("sso-token")
	if err != nil {
		helpers.NotAuthorized(w, err)
		return
	}
	if token == nil {
		helpers.NotAuthorized(w, errors.New("no token"))
		return
	}
	if _, err := s.Auth.Authorize(token.Value); err != nil {
		helpers.NotAuthorized(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	if err := s.Auth.Logout(r.Context().Value("token").(string)); err != nil {
		log.Printf("Error logging out: %s", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   "",
		Expires: time.Now().Add(-1 * time.Hour * 24),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func AuthMiddleware(db *sqlx.DB) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := r.Cookie("sso-token")
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			user, err := authentication.New(db).Authorize(token.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), "user", user)
			ctx = context.WithValue(ctx, "token", token.Value)
			ctx = context.WithValue(ctx, "authenticated", true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (s *Server) GraphQL(schema graphql.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			helpers.BadRequest(w, err)
			return
		}
		result := graphql.Do(graphql.Params{
			Schema:         schema,
			RequestString:  data.Query,
			VariableValues: data.Variables,
			Context:        r.Context(),
		})
		if len(result.Errors) > 0 {
			helpers.RespondWithJson(w, http.StatusBadRequest, result)
			return
		}
		helpers.RespondWithJson(w, http.StatusOK, result)
	}
}

func (s *Server) Start() {
	handlers := []routes.Route{
		&users.ApiRoute{},
	}
	r := mux.NewRouter().StrictSlash(true)
	opts := &routes.Options{
		Db: s.Db,
	}
	for _, route := range handlers {
		prefix := route.Prefix()
		var subRouter *mux.Router
		if prefix == "/" || prefix == "" {
			subRouter = r
		} else {
			subRouter = r.PathPrefix(prefix).Subrouter()
		}
		route.Setup(subRouter, opts)
	}
	models := []*service.Model{
		{
			Pk:    "id",
			Table: "companies",
			Fields: []*service.Field{
				{
					Name:     "name",
					Type:     reflect.String,
					Nullable: false,
				},
				{
					Name:     "address",
					Type:     reflect.String,
					Nullable: true,
				},
			},
		},
		{
			Pk:    "id",
			Table: "employees",
			Fields: []*service.Field{
				{
					Name:     "first_name",
					Type:     reflect.String,
					Nullable: false,
				},
				{
					Name:     "last_name",
					Type:     reflect.String,
					Nullable: false,
				},
				{
					Name:     "company_id",
					Type:     reflect.Int,
					Nullable: false,
				},
				{
					Name:     "email",
					Type:     reflect.String,
					Nullable: false,
				},
				{
					Name:     "salary",
					Type:     reflect.Float64,
					Nullable: true,
				},
			},
		},
	}
	combinedQueryFields := graphql.Fields{}
	combineMutations := graphql.Fields{}
	for _, model := range models {
		q, m := service.GraphQLAdapter(&service.GraphQLAdapterOptions{
			Service: service.New(s.Db, model),
			Name:    model.Table,
		})
		combinedQueryFields[model.Table] = &graphql.Field{
			Type: q,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return q, nil
			},
		}
		combineMutations[model.Table] = &graphql.Field{
			Type: m,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return m, nil
			},
		}
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(
			graphql.ObjectConfig{
				Name:   "Query",
				Fields: combinedQueryFields,
			},
		),
		Mutation: graphql.NewObject(
			graphql.ObjectConfig{
				Name:   "Mutation",
				Fields: combineMutations,
			},
		),
	})
	if err != nil {
		log.Fatal(err)
		return
	}
	r.Use(loggingMiddleware)
	r.Use(AuthMiddleware(s.Db))
	r.HandleFunc("/graphql", s.GraphQL(schema)).Methods(http.MethodPost)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))
	log.Println("Listening on port :3200")
	if utils.GetEnv("GO_APP_ENV", "development") == "production" {
		err = http.ListenAndServe(":3200", r)
	} else {
		err = http.ListenAndServe("localhost:3200", r)
	}
	log.Fatal(err)
}
