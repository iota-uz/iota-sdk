package server

import (
	"context"
	"errors"
	"github.com/apollos-studio/sso/pkg/authentication"
	"github.com/apollos-studio/sso/pkg/server/helpers"
	"github.com/apollos-studio/sso/pkg/server/routes"
	"github.com/apollos-studio/sso/pkg/server/routes/resources"
	"github.com/apollos-studio/sso/pkg/server/routes/roles"
	"github.com/apollos-studio/sso/pkg/server/routes/users"
	"github.com/apollos-studio/sso/pkg/utils"
	"github.com/apollos-studio/sso/templates/pages"
	"github.com/apollos-studio/sso/templates/pages/login"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"log"
	"net/http"
	"net/url"
	"strings"
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

func (s *Server) Authenticate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	cookie, err := r.Cookie("sso-token")
	if cookie != nil && err == nil {
		s.handleRedirects(w, r, cookie.Value)
		return
	}
	authErrors := &login.AuthenticationErrors{}
	ctx := &login.AuthenticationContext{
		Email:    r.FormValue("email"),
		Password: r.FormValue("password"),
		Errors:   authErrors,
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := strings.TrimSpace(r.FormValue("password"))
	if email == "" {
		authErrors.Email = "Email is required"
	}
	if password == "" {
		authErrors.Password = "Password is required"
	}
	if authErrors.Email != "" || authErrors.Password != "" {
		if err := login.Index(ctx).Render(context.Background(), w); err != nil {
			helpers.ServerError(w, err)
			return
		}
		return
	}
	_, token, err := s.Auth.Authenticate(email, password, r.RemoteAddr, r.UserAgent())
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	s.handleRedirects(w, r, token)
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	authenticated, ok := r.Context().Value("authenticated").(bool)
	if ok && authenticated {
		s.handleRedirects(w, r, r.Context().Value("token").(string))
		return
	}
	ctx := &login.AuthenticationContext{
		Errors: &login.AuthenticationErrors{},
	}
	if err := login.Index(ctx).Render(context.Background(), w); err != nil {
		helpers.ServerError(w, err)
		return
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

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	auth, ok := r.Context().Value("authenticated").(bool)
	if !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := pages.Index().Render(context.Background(), w); err != nil {
		helpers.ServerError(w, err)
		return
	}
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

func (s *Server) Start() {
	handlers := []routes.Route{
		&users.ApiRoute{},
		&users.Route{},
		&roles.ApiRoute{},
		&roles.Route{},
		&resources.Route{},
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
	r.Use(loggingMiddleware)
	r.Use(AuthMiddleware(s.Db))
	r.HandleFunc("/", s.Index).Methods(http.MethodGet)
	r.HandleFunc("/login", s.Login).Methods(http.MethodGet)
	r.HandleFunc("/login", s.Authenticate).Methods(http.MethodPost)
	r.HandleFunc("/authorize", s.Authorize).Methods(http.MethodGet)
	r.HandleFunc("/logout", s.Logout).Methods(http.MethodGet)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))
	log.Println("Listening on port :3200")
	err := http.ListenAndServe(":3200", r)
	log.Fatal(err)
}
