package mcpgateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

// AccountResolver maps a verified token subject to its account and tenant.
// It runs on every request, so blocked or deleted accounts lose access on an
// existing connection; it must reject them.
type AccountResolver interface {
	Principal(ctx context.Context, subject string) (mcp.Principal, error)
}

type AccountResolverFunc func(ctx context.Context, subject string) (mcp.Principal, error)

func (f AccountResolverFunc) Principal(ctx context.Context, subject string) (mcp.Principal, error) {
	return f(ctx, subject)
}

// OAuth configures the gateway as an OAuth 2.1 resource server for access
// tokens issued by an authorization server such as the SDK oidc module.
type OAuth struct {
	// Resource is the canonical URL of the MCP endpoint.
	Resource string
	// Issuer is the authorization server; tokens from any other issuer fail.
	Issuer string
	// Audiences are accepted aud values besides Resource, e.g. the client id
	// of an authorization server that does not support resource indicators.
	Audiences []string
	// KeySet returns the issuer's signing keys, e.g. oidc Storage.GetKeySet.
	KeySet          func(ctx context.Context) (*jose.JSONWebKeySet, error)
	Accounts        AccountResolver
	ScopesSupported []string
	// RequiredScopes are needed to connect at all; tools add their own.
	RequiredScopes []string
	ClockSkew      time.Duration
}

var signingMethods = []string{"RS256", "RS384", "RS512", "ES256", "ES384", "PS256"}

// Verify checks signature, issuer, audience/resource and expiry, then binds
// the account and tenant from trusted server state, never from the token's
// free-form claims.
func (o OAuth) Verify(ctx context.Context, raw string, _ *http.Request) (*auth.TokenInfo, error) {
	keys, err := o.KeySet(ctx)
	if err != nil {
		return nil, fmt.Errorf("load signing keys: %w", err)
	}
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		found := keys.Key(kid)
		if len(found) == 0 {
			return nil, errors.New("unknown signing key")
		}
		return found[0].Public().Key, nil
	},
		jwt.WithIssuer(o.Issuer),
		jwt.WithValidMethods(signingMethods),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(o.ClockSkew),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", auth.ErrInvalidToken, err)
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	audience, _ := claims.GetAudience()
	if !oauthex.MatchesResource(audience, o.Resource) && !slices.ContainsFunc(audience, func(a string) bool {
		return slices.Contains(o.Audiences, a)
	}) {
		return nil, fmt.Errorf("%w: token is not issued for this resource", auth.ErrInvalidToken)
	}
	subject, _ := claims.GetSubject()
	if subject == "" {
		return nil, fmt.Errorf("%w: token has no subject", auth.ErrInvalidToken)
	}
	p, err := o.Accounts.Principal(ctx, subject)
	if err != nil || !p.Valid() {
		return nil, fmt.Errorf("%w: account is not available", auth.ErrInvalidToken)
	}
	scopes := scopesOf(claims)
	p.Scopes = mcp.GrantedScopes(scopes...)
	if clientID, ok := claims["client_id"].(string); ok {
		p.ClientID = clientID
	} else if azp, ok := claims["azp"].(string); ok {
		p.ClientID = azp
	}
	expires, _ := claims.GetExpirationTime()
	return &auth.TokenInfo{
		Scopes:     scopes,
		Expiration: expires.Time,
		UserID:     p.TenantID.String() + "/" + p.Subject(),
		Extra:      map[string]any{principalKey: p},
	}, nil
}

func scopesOf(claims jwt.MapClaims) []string {
	switch v := claims["scope"].(type) {
	case string:
		return strings.Fields(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, s := range v {
			if str, ok := s.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

// MetadataPath is where RFC 9728 places the protected resource metadata for
// Resource; mount MetadataHandler there.
func (o OAuth) MetadataPath() string {
	u, err := url.Parse(o.Resource)
	if err != nil {
		return "/.well-known/oauth-protected-resource"
	}
	return "/.well-known/oauth-protected-resource" + strings.TrimSuffix(u.Path, "/")
}

func (o OAuth) metadataURL() string {
	u, err := url.Parse(o.Resource)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host + o.MetadataPath()
}

// MetadataHandler serves the protected resource metadata that lets clients
// discover the authorization server.
func (o OAuth) MetadataHandler() http.Handler {
	return auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               o.Resource,
		AuthorizationServers:   []string{o.Issuer},
		ScopesSupported:        o.ScopesSupported,
		BearerMethodsSupported: []string{"header"},
	})
}

// Handler serves Streamable HTTP behind bearer authentication; a missing or
// invalid token gets 401 with the metadata location for discovery.
func (o OAuth) Handler(server *sdk.Server, opts *sdk.StreamableHTTPOptions) http.Handler {
	streamable := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, opts)
	return auth.RequireBearerToken(o.Verify, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: o.metadataURL(),
		Scopes:              o.RequiredScopes,
		ClockSkew:           o.ClockSkew,
	})(streamable)
}
