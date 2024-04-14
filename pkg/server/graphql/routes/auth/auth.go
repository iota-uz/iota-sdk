package auth

import (
	"errors"
	"github.com/graphql-go/graphql"
	"github.com/iota-agency/iota-erp/pkg/authentication"
	"github.com/iota-agency/iota-erp/pkg/utils"
	"github.com/jmoiron/sqlx"
	"net/http"
	"time"
)

func Authenticate(db *sqlx.DB) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		email, ok := p.Args["email"].(string)
		if !ok {
			return nil, errors.New("email is required")
		}
		password, ok := p.Args["password"].(string)
		if !ok {
			return nil, errors.New("password is required")
		}
		ip, ok := p.Context.Value("ip").(string)
		if !ok {
			ip = "-"
		}
		userAgent, ok := p.Context.Value("userAgent").(string)
		if !ok {
			userAgent = "-"
		}
		auth := &authentication.Authentication{Db: db}
		_, token, err := auth.Authenticate(email, password, ip, userAgent)
		if err != nil {
			return nil, err
		}
		writer := p.Context.Value("writer").(http.ResponseWriter)
		cookie := &http.Cookie{
			Name:     "token",
			Value:    token,
			Expires:  time.Now().Add(utils.SessionDuration()),
			HttpOnly: false,
			SameSite: http.SameSiteNoneMode,
			Secure:   false,
			Domain:   utils.GetEnv("DOMAIN", "localhost"),
		}
		http.SetCookie(writer, cookie)
		return map[string]interface{}{"token": token}, nil
	}
}

func GraphQL(db *sqlx.DB) (*graphql.Object, *graphql.Object) {
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "Sessions",
		Description: "A",
		Fields: graphql.Fields{
			"ip": &graphql.Field{
				Name: "ip",
				Type: graphql.String,
			},
		},
	})
	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Authentication",
		Fields: graphql.Fields{
			"authenticate": &graphql.Field{
				Args: graphql.FieldConfigArgument{
					"email": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"password": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "AuthPayload",
					Fields: graphql.Fields{
						"token": &graphql.Field{
							Type: graphql.String,
						},
					},
				}),
				Resolve: Authenticate(db),
			},
		},
	})
	return queryType, mutationType
}
