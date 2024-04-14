package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/pkg/authentication"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/routes"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/routes/auth"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/routes/bichat"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/routes/users"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/service"
	"github.com/iota-agency/iota-erp/pkg/server/helpers"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/jmoiron/sqlx"
	"github.com/rs/cors"
	"log"
	"net/http"
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

func (s *Server) Authorize(w http.ResponseWriter, r *http.Request) {
	token, err := r.Cookie("token")
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
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

func AuthMiddleware(db *sqlx.DB) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "writer", w)
			token, err := r.Cookie("token")
			if err != nil {
				ctx = context.WithValue(ctx, "authenticated", false)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			user, err := authentication.New(db).Authorize(token.Value)
			if err != nil {
				ctx = context.WithValue(ctx, "authenticated", false)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			ctx = context.WithValue(ctx, "authenticated", true)
			ctx = context.WithValue(ctx, "user", user)
			ctx = context.WithValue(ctx, "token", token.Value)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (s *Server) HandleGraphQL(schema graphql.Schema) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			helpers.BadRequest(w, err)
			return
		}
		ctx := context.WithValue(r.Context(), "ip", r.RemoteAddr)
		ctx = context.WithValue(ctx, "userAgent", r.UserAgent())
		result := graphql.Do(graphql.Params{
			Schema:         schema,
			RequestString:  data.Query,
			VariableValues: data.Variables,
			Context:        ctx,
		})
		if len(result.Errors) > 0 {
			helpers.RespondWithJson(w, http.StatusBadRequest, result)
			return
		}
		helpers.RespondWithJson(w, http.StatusOK, result)
	}
}

func (s *Server) graphQlSchema() (graphql.Schema, error) {
	graphqlConstructors := []routes.GraphQLConstructor{
		bichat.GraphQL,
		auth.GraphQL,
		users.GraphQL,
	}
	combinedQueryFields := graphql.Fields{}
	combineMutations := graphql.Fields{}
	for _, model := range Models {
		q, m := service.GraphQLAdapter(&service.GraphQLAdapterOptions{
			Db:    s.Db,
			Model: model,
			Name:  model.Table,
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
	for _, constructor := range graphqlConstructors {
		query, mutation := constructor(s.Db)
		combinedQueryFields[query.Name()] = &graphql.Field{
			Type: query,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return query, nil
			},
		}
		combineMutations[mutation.Name()] = &graphql.Field{
			Type: mutation,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return mutation, nil
			},
		}
	}
	return graphql.NewSchema(graphql.SchemaConfig{
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
}

func (s *Server) Start() {
	r := mux.NewRouter().StrictSlash(true)
	//handlers := []routes.Route{
	//	&users.ApiRoute{},
	//}
	//opts := &routes.Options{
	//	Db: s.Db,
	//}
	//for _, route := range handlers {
	//	prefix := route.Prefix()
	//	var subRouter *mux.Router
	//	if prefix == "/" || prefix == "" {
	//		subRouter = r
	//	} else {
	//		subRouter = r.PathPrefix(prefix).Subrouter()
	//	}
	//	route.Setup(subRouter, opts)
	//}
	r.Use(loggingMiddleware)
	r.Use(AuthMiddleware(s.Db))
	schema, err := s.graphQlSchema()
	if err != nil {
		log.Fatal(err)
		return
	}
	r.HandleFunc("/graphql", s.HandleGraphQL(schema)).Methods(http.MethodPost)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))
	handler := cors.Default().Handler(r)
	log.Println("Listening on port :3200")
	if utils.GetEnv("GO_APP_ENV", "development") == "production" {
		err = http.ListenAndServe(":3200", handler)
	} else {
		err = http.ListenAndServe("localhost:3200", handler)
	}
	log.Fatal(err)
}
