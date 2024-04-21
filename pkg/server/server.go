package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/pkg/authentication"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/routes"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/routes/auth"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/routes/employees"
	expenseCategories "github.com/iota-agency/iota-erp/pkg/server/graphql/routes/expense-categories"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/routes/expenses"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/routes/positions"
	taskTypes "github.com/iota-agency/iota-erp/pkg/server/graphql/routes/task-types"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/routes/users"
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
	queryConstructors := []routes.GraphQLConstructor{
		//bichat.GraphQL,
		users.Queries,
		expenses.Queries,
		expenseCategories.Queries,
		employees.Queries,
		positions.Queries,
		taskTypes.Queries,
	}
	mutationConstructors := []routes.GraphQLConstructor{
		auth.Mutations,
		users.Mutations,
		expenses.Mutations,
		expenseCategories.Mutations,
		employees.Mutations,
		positions.Mutations,
		taskTypes.Mutations,
	}
	combinedQueries := graphql.Fields{}
	combineMutations := graphql.Fields{}
	for _, constructor := range queryConstructors {
		for _, field := range constructor(s.Db) {
			if field.Name == "" {
				panic(fmt.Sprintf("field.Name is missing on %v", field))
			}
			combinedQueries[field.Name] = field
		}
	}
	for _, constructor := range mutationConstructors {
		for _, field := range constructor(s.Db) {
			combineMutations[field.Name] = field
		}
	}
	return graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(
			graphql.ObjectConfig{
				Name:   "Query",
				Fields: combinedQueries,
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
