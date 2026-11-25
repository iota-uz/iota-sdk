// Package httpapi is the generic HTTP/API connector of automation graphs: a connection names an https API
// and how requests to it are authorized (a bearer token, a key in a header, or a login and a password), and
// Code and HTTP nodes then call the API as env.connections.<name>.get(path), .post(path, body) and
// .fetch(path, init). The SDK adds the authorization, so a token never reaches the code; every request stays
// on the API's origin.
//
// The connector has no node and no trigger: it exists for code access (connectors.CodeAccessor).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// Kind is the connector kind.
const Kind = catalog.KindHTTPAPI

// Credentials of a connection.
const (
	KeyBaseURL    = "base_url"
	KeyAuth       = "auth"
	KeyToken      = "token"
	KeyHeaderName = "header_name"
	KeyUsername   = "username"
	KeyPassword   = "password"
)

// Ways a connection authorizes its requests.
const (
	AuthNone   = "none"
	AuthBearer = "bearer"
	AuthHeader = "header"
	AuthBasic  = "basic"
)

// maxValue bounds a secret or a header value of a connection.
const maxValue = 4096

const credentialsSchema = `{
  "type": "object",
  "required": ["base_url"],
  "properties": {
    "base_url": {"type": "string", "title": "API address", "x-label": {"en":"API address","ru":"Адрес API","uz":"API manzili","oz":"API манзили"}, "description": "https only, without a query: https://api.example.com/v1", "pattern": "^https://[^\\s]+$", "maxLength": 300, "examples": ["https://api.example.com/v1"]},
    "auth": {"type": "string", "title": "Authorization", "x-label": {"en":"Authorization","ru":"Авторизация","uz":"Avtorizatsiya","oz":"Авторизация"}, "enum": ["none", "bearer", "header", "basic"], "default": "none", "description": "none; bearer: Authorization: Bearer <token>; header: a header with the token; basic: a login and a password"},
    "token": {"type": "string", "title": "Token", "x-label": {"en":"Token","ru":"Токен","uz":"Token","oz":"Токен"}, "description": "For bearer and header", "x-secret": true, "x-authorization-only": true, "maxLength": 4096},
    "header_name": {"type": "string", "title": "Header name", "x-label": {"en":"Header name","ru":"Имя заголовка","uz":"Sarlavha nomi","oz":"Сарлавҳа номи"}, "description": "For header, for example X-API-Key", "maxLength": 100, "pattern": "^[A-Za-z][A-Za-z0-9-]*$"},
    "username": {"type": "string", "title": "Login", "x-label": {"en":"Login","ru":"Логин","uz":"Login","oz":"Логин"}, "description": "For basic", "maxLength": 500},
    "password": {"type": "string", "title": "Password", "x-label": {"en":"Password","ru":"Пароль","uz":"Parol","oz":"Парол"}, "description": "For basic", "x-secret": true, "x-authorization-only": true, "maxLength": 4096}
  },
  "additionalProperties": false
}`

// New is the connector "http_api".
func New() connectors.Connector { return &connector{} }

type connector struct{}

var (
	_ connectors.Connector           = (*connector)(nil)
	_ connectors.CodeAccessor        = (*connector)(nil)
	_ connectors.CodeBaseURLResolver = (*connector)(nil)
)

func (c *connector) Kind() catalog.ConnectorKind       { return Kind }
func (c *connector) Nodes() []catalog.NodeType         { return nil }
func (c *connector) Triggers() []catalog.TriggerSource { return nil }

func (c *connector) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind:        Kind,
		Title:       map[string]string{"en": "HTTP API", "ru": "HTTP API", "uz": "HTTP API", "oz": "HTTP API"},
		Description: map[string]string{"en": "Call any https API from code and HTTP nodes without exposing its token", "ru": "Вызовы любого https API из кода и HTTP-узлов без раскрытия токена", "uz": "Istalgan https API ni token oshkor qilinmasdan kod va HTTP tugunlaridan chaqirish", "oz": "Исталган https API ни токен ошкор қилинмасдан код ва HTTP тугунларидан чақириш"},
		Category:    connectors.CategoryOther,
		Icon:        "plugs-connected",
		Auth:        connectors.AuthCustom,
		Credentials: json.RawMessage(credentialsSchema),
		Platform:    connectors.PlatformNone,
	}
}

// CodeAccess offers every HTTP method and a fetch on the connection's API. The base URL is each connection's
// own (CodeBaseURL), so none is declared here.
func (c *connector) CodeAccess() connectors.CodeAccess {
	return connectors.CodeAccess{
		Methods: []connectors.CodeMethod{
			{Name: "get", HTTPMethod: http.MethodGet},
			{Name: "post", HTTPMethod: http.MethodPost},
			{Name: "put", HTTPMethod: http.MethodPut},
			{Name: "patch", HTTPMethod: http.MethodPatch},
			{Name: "delete", HTTPMethod: http.MethodDelete},
		},
		Fetch: true,
	}
}

// errSettings is a connection whose settings do not describe an API it may call. It names the problem, never
// a value.
var errSettings = errors.New("http_api: the connection's settings are invalid")

// settings are the credentials of a connection, checked.
type settings struct {
	base   string
	auth   string
	token  string
	header string
	user   string
	pass   string
}

func read(creds connectors.Credentials) (settings, error) {
	s := settings{
		base: strings.TrimSpace(creds[KeyBaseURL]), auth: strings.TrimSpace(creds[KeyAuth]),
		token: creds[KeyToken], header: strings.TrimSpace(creds[KeyHeaderName]), user: creds[KeyUsername], pass: creds[KeyPassword],
	}
	if err := connectors.CheckCodeBaseURL(s.base); err != nil {
		return settings{}, errors.New("http_api: " + err.Error())
	}
	if s.auth == "" {
		s.auth = AuthNone
	}
	invalid := func(why string) (settings, error) { return settings{}, errors.New(errSettings.Error() + ": " + why) }
	switch s.auth {
	case AuthNone:
	case AuthBearer:
		if s.token == "" {
			return invalid("auth bearer needs a token")
		}
	case AuthHeader:
		if s.token == "" || s.header == "" {
			return invalid("auth header needs a header name and a token")
		}
		if !headerName(s.header) {
			return invalid("the header name is not allowed")
		}
	case AuthBasic:
		if s.user == "" {
			return invalid("auth basic needs a login")
		}
		if strings.Contains(s.user, ":") {
			return invalid("a login of basic has no colon")
		}
	default:
		return invalid("auth is none, bearer, header or basic")
	}
	for _, v := range []string{s.token, s.user, s.pass} {
		if len(v) > maxValue || strings.ContainsAny(v, "\r\n\x00") {
			return invalid("a token, login or password is too long or has a line break")
		}
	}
	return s, nil
}

// headerName reports whether name may carry a token: a plain header name that the transport owns none of.
func headerName(name string) bool {
	if len(name) == 0 || len(name) > 100 {
		return false
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		letter := ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z'
		if !letter && (i == 0 || !(ch >= '0' && ch <= '9' || ch == '-')) {
			return false
		}
	}
	switch strings.ToLower(name) {
	case "host", "content-length", "content-type", "transfer-encoding", "connection", "upgrade", "te", "trailer", "keep-alive",
		"proxy-authorization", "proxy-authenticate", "cookie", "set-cookie", "accept-encoding", "expect":
		return false
	}
	return true
}

// CodeBaseURL is the https base URL of the connection. Its error names the problem, never the URL.
func (c *connector) CodeBaseURL(creds connectors.Credentials) (string, error) {
	s, err := read(creds)
	if err != nil {
		return "", err
	}
	return s.base, nil
}

// Authorize adds the connection's authorization to a request's headers, replacing a header of the same name
// the code set. Its error names the problem, never a credential.
func (c *connector) Authorize(_ context.Context, creds connectors.Credentials, header http.Header) error {
	s, err := read(creds)
	if err != nil {
		return err
	}
	switch s.auth {
	case AuthBearer:
		header.Set("Authorization", "Bearer "+s.token)
	case AuthHeader:
		header.Set(s.header, s.token)
	case AuthBasic:
		r := &http.Request{Header: http.Header{}}
		r.SetBasicAuth(s.user, s.pass)
		header.Set("Authorization", r.Header.Get("Authorization"))
	}
	return nil
}

// Test checks the settings without calling the API: a call would carry the token to an address the tenant
// named. The identity is the API's host.
func (c *connector) Test(_ context.Context, creds connectors.Credentials) (connectors.Identity, error) {
	s, err := read(creds)
	if err != nil {
		return connectors.Identity{}, err
	}
	u, err := url.Parse(s.base)
	if err != nil {
		return connectors.Identity{}, errSettings
	}
	return connectors.Identity{Display: u.Host, External: u.Host}, nil
}
