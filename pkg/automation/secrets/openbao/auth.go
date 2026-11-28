package openbao

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
)

// Token is a token OpenBao issued to the service and how long it lives.
type Token struct {
	Value     string
	TTL       time.Duration // 0: it does not expire
	Renewable bool
}

// Login is what an Auth may ask of OpenBao while logging in: the requests carry no token of the client.
type Login interface {
	// Login posts body to an auth method's login path ("auth/approle/login") and returns the issued token.
	Login(ctx context.Context, authPath string, body map[string]any) (Token, error)
	// LookupSelf tells the TTL of a token the service was given rather than issued.
	LookupSelf(ctx context.Context, token string) (Token, error)
}

// Auth logs the service in. The client calls it on the first request, when its token can no longer be
// renewed, and when OpenBao refuses the token. An Auth reads its credential (a file) on every login, so a
// rotated secret_id, JWT or agent-written token is picked up without a restart. An error never quotes the
// credential.
type Auth interface {
	Login(ctx context.Context, l Login) (Token, error)
	// CanRelogin reports whether a refused token may be replaced by logging in again.
	CanRelogin() bool
}

// Credential reads a credential when a login needs it.
type Credential func() (string, error)

// Literal is a credential given as it is.
func Literal(v string) Credential { return func() (string, error) { return v, nil } }

// File is a credential read from a file on every login (an OpenBao agent's token sink, a projected
// Kubernetes service-account token, a secret_id a deploy rotates); surrounding whitespace is trimmed.
func File(path string) Credential {
	return func() (string, error) {
		raw, err := os.ReadFile(path)
		if err != nil {
			// The path is configuration, not a secret; the error never reads the content.
			return "", errors.New("the credential file " + path + " cannot be read")
		}
		return strings.TrimSpace(string(raw)), nil
	}
}

func read(c Credential, what string) (string, error) {
	if c == nil {
		return "", errors.New(what + " is not configured")
	}
	v, err := c()
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New(what + " is empty")
	}
	return v, nil
}

func authPath(mount, def string) string {
	mount = strings.Trim(strings.TrimSpace(mount), "/")
	if mount == "" {
		mount = def
	}
	return "auth/" + mount + "/login"
}

// TokenAuth uses a token the service is given (an agent's sink file, a periodic token): its TTL is
// looked up and it is renewed while renewable; once it cannot be, the credential is read again.
func TokenAuth(token Credential) Auth { return tokenAuth{token} }

type tokenAuth struct{ token Credential }

func (a tokenAuth) Login(ctx context.Context, l Login) (Token, error) {
	tok, err := read(a.token, "the token")
	if err != nil {
		return Token{}, err
	}
	return l.LookupSelf(ctx, tok)
}

// CanRelogin is true: the credential is read again (an agent may have written a new token).
func (tokenAuth) CanRelogin() bool { return true }

// AppRoleAuth logs in with AppRole (mount default "approle").
func AppRoleAuth(mount string, roleID, secretID Credential) Auth {
	return appRoleAuth{path: authPath(mount, "approle"), roleID: roleID, secretID: secretID}
}

type appRoleAuth struct {
	path             string
	roleID, secretID Credential
}

func (a appRoleAuth) Login(ctx context.Context, l Login) (Token, error) {
	role, err := read(a.roleID, "the role_id")
	if err != nil {
		return Token{}, err
	}
	body := map[string]any{"role_id": role}
	if a.secretID != nil {
		secret, err := read(a.secretID, "the secret_id")
		if err != nil {
			return Token{}, err
		}
		body["secret_id"] = secret
	}
	return l.Login(ctx, a.path, body)
}

func (appRoleAuth) CanRelogin() bool { return true }

// KubernetesAuth logs in with a service-account JWT (mount default "kubernetes"); jwt is usually
// File("/var/run/secrets/kubernetes.io/serviceaccount/token").
func KubernetesAuth(mount, role string, jwt Credential) Auth {
	return jwtAuth{path: authPath(mount, "kubernetes"), role: role, jwt: jwt}
}

// JWTAuth logs in with the JWT/OIDC method (mount default "jwt").
func JWTAuth(mount, role string, jwt Credential) Auth {
	return jwtAuth{path: authPath(mount, "jwt"), role: role, jwt: jwt}
}

type jwtAuth struct {
	path, role string
	jwt        Credential
}

func (a jwtAuth) Login(ctx context.Context, l Login) (Token, error) {
	if strings.TrimSpace(a.role) == "" {
		return Token{}, errors.New("the role is not configured")
	}
	jwt, err := read(a.jwt, "the JWT")
	if err != nil {
		return Token{}, err
	}
	return l.Login(ctx, a.path, map[string]any{"role": a.role, "jwt": jwt})
}

func (jwtAuth) CanRelogin() bool { return true }
