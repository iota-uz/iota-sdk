package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

// The OAuth metadata documents of the MCP authorization specification, read with the guarded client.
// (The SDK keeps its own helpers behind a build tag, so the connector reads them itself.)

// resourceMetadata is OAuth 2.0 Protected Resource Metadata (RFC 9728).
type resourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

// serverMetadata is OAuth 2.0 Authorization Server Metadata (RFC 8414) or OpenID provider metadata.
type serverMetadata struct {
	Issuer                        string   `json:"issuer"`
	AuthorizationEndpoint         string   `json:"authorization_endpoint"`
	TokenEndpoint                 string   `json:"token_endpoint"`
	RegistrationEndpoint          string   `json:"registration_endpoint"`
	ScopesSupported               []string `json:"scopes_supported"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
}

// clientRegistration is the answer of Dynamic Client Registration (RFC 7591).
type clientRegistration struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

const maxMetadataBytes = 1 << 20

// getJSON reads a JSON document; found=false for a 4xx answer.
func getJSON(ctx context.Context, httpc *http.Client, u string, v any) (bool, error) {
	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, u, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := httpc.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 && res.StatusCode < 500 {
		return false, nil
	}
	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("mcp: %s answered HTTP %d", u, res.StatusCode)
	}
	if mt, _, err := mime.ParseMediaType(res.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return false, fmt.Errorf("mcp: %s is not JSON", u)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxMetadataBytes)).Decode(v); err != nil {
		return false, fmt.Errorf("mcp: %s is not readable", u)
	}
	return true, nil
}

var resourceMetadataRe = regexp.MustCompile(`resource_metadata\s*=\s*"([^"]+)"`)

// resourceMetadataURL is the resource_metadata of a Bearer challenge.
func resourceMetadataURL(h http.Header) string {
	for _, v := range h.Values("WWW-Authenticate") {
		if m := resourceMetadataRe.FindStringSubmatch(v); m != nil {
			return m[1]
		}
	}
	return ""
}

// protectedResource reads the protected resource metadata of server at metaURL; its resource must be
// the server.
func protectedResource(ctx context.Context, httpc *http.Client, metaURL, server string) (*resourceMetadata, error) {
	var prm resourceMetadata
	found, err := getJSON(ctx, httpc, metaURL, &prm)
	if err != nil || !found {
		return nil, err
	}
	if strings.TrimSuffix(prm.Resource, "/") != strings.TrimSuffix(server, "/") {
		return nil, fmt.Errorf("mcp: the resource metadata is of %q, not of the server", prm.Resource)
	}
	return &prm, nil
}

// resourceCandidates are the well-known URIs of a server's resource metadata: with its path, then at
// the root.
func resourceCandidates(server *url.URL) []string {
	origin := server.Scheme + "://" + server.Host
	out := []string{}
	if p := strings.TrimSuffix(server.Path, "/"); p != "" {
		out = append(out, origin+path.Join("/.well-known/oauth-protected-resource", p))
	}
	return append(out, origin+"/.well-known/oauth-protected-resource")
}

// authorizationServer reads an issuer's metadata at metaURL: its issuer must be the one asked for and
// it must support PKCE S256.
func authorizationServer(ctx context.Context, httpc *http.Client, metaURL, issuer string) (*serverMetadata, error) {
	var m serverMetadata
	found, err := getJSON(ctx, httpc, metaURL, &m)
	if err != nil || !found {
		return nil, err
	}
	if strings.TrimSuffix(m.Issuer, "/") != strings.TrimSuffix(issuer, "/") {
		return nil, fmt.Errorf("mcp: the metadata is of issuer %q", m.Issuer)
	}
	if len(m.CodeChallengeMethodsSupported) > 0 && !slices.Contains(m.CodeChallengeMethodsSupported, "S256") {
		return nil, fmt.Errorf("mcp: the authorization server does not support PKCE S256")
	}
	return &m, nil
}

// registerClient is Dynamic Client Registration of a public client.
func registerClient(ctx context.Context, httpc *http.Client, endpoint string, meta map[string]any) (*clientRegistration, error) {
	body, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp: the registration endpoint answered HTTP %d", res.StatusCode)
	}
	var reg clientRegistration
	if err := json.NewDecoder(io.LimitReader(res.Body, maxMetadataBytes)).Decode(&reg); err != nil || reg.ClientID == "" {
		return nil, fmt.Errorf("mcp: the registration has no client_id")
	}
	return &reg, nil
}
