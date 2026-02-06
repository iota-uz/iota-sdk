package controllers

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	"github.com/iota-uz/iota-sdk/modules/oidc/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// CallbackQueryDTO represents the query parameters for the OIDC callback endpoint
type CallbackQueryDTO struct {
	ID string `schema:"id"`
}

type OIDCController struct {
	app         application.Application
	storage     *oidc.Storage
	config      *configuration.OIDCOptions
	oidcService *services.OIDCService
	provider    op.OpenIDProvider
}

func NewOIDCController(
	app application.Application,
	storage *oidc.Storage,
	config *configuration.OIDCOptions,
	oidcService *services.OIDCService,
) *OIDCController {
	return &OIDCController{
		app:         app,
		storage:     storage,
		config:      config,
		oidcService: oidcService,
	}
}

func (c *OIDCController) Key() string {
	return "/oidc"
}

// Register mounts the OIDC provider router and custom routes
func (c *OIDCController) Register(r *mux.Router) {
	// Convert base64-encoded crypto key to [32]byte array
	var cryptoKey [32]byte
	// Decode base64 crypto key
	decodedKey, err := base64.StdEncoding.DecodeString(c.config.CryptoKey)
	if err != nil {
		panic(fmt.Sprintf("Failed to decode crypto key (must be base64-encoded): %v", err))
	}
	if len(decodedKey) != 32 {
		panic(fmt.Sprintf("Crypto key must be exactly 32 bytes after base64 decoding, got %d bytes", len(decodedKey)))
	}
	copy(cryptoKey[:], decodedKey)

	// Initialize zitadel OIDC provider configuration
	providerConfig := &op.Config{
		CryptoKey:                cryptoKey,
		DefaultLogoutRedirectURI: "/",
		CodeMethodS256:           true, // Enable PKCE with S256
	}

	// Initialize zitadel OIDC provider
	provider, err := op.NewProvider(
		providerConfig,
		c.storage,
		op.StaticIssuer(c.config.IssuerURL),
		// op.WithAllowInsecure() can be added for development without HTTPS
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize OIDC provider: %v", err))
	}

	c.provider = provider

	// Mount OIDC provider routes (all routes are intentionally public)
	//
	// IMPORTANT: No authorization middleware is applied to these routes because:
	// - /.well-known/openid-configuration is a public discovery endpoint (OAuth 2.0 spec)
	// - /oidc/authorize is the public authorization endpoint (requires client_id but not auth)
	// - /oidc/token is the public token endpoint (authenticated via client credentials)
	// - /oidc/userinfo requires Bearer token in Authorization header (self-protected)
	// - /oidc/keys is a public JWKS endpoint for verifying JWT signatures
	// - /oidc/revoke is the public token revocation endpoint (authenticated via client credentials)
	// - /oidc/end_session is the public logout endpoint
	//
	// Security is enforced via:
	// 1. Client authentication (client_id + client_secret or PKCE)
	// 2. Bearer token validation (for /userinfo)
	// 3. Authorization code flow validation
	// 4. Redirect URI validation
	//
	// This provides:
	// - /.well-known/openid-configuration (discovery endpoint)
	// - /authorize (authorization endpoint)
	// - /oauth/token (token endpoint)
	// - /userinfo (userinfo endpoint)
	// - /keys (JWKS endpoint)
	// - /revoke (revocation endpoint)
	// - /end_session (logout endpoint)
	// - /callback (auth callback after login)
	r.PathPrefix("/.well-known/").Handler(provider)

	// Register custom handler for completing login flow BEFORE catch-all /oidc/ handler
	// This is called after user successfully logs in via /login
	// IMPORTANT: Must be registered before PathPrefix("/oidc/") to avoid being shadowed
	r.HandleFunc("/oidc/authorize/callback", c.handleCallback).Methods(http.MethodGet)

	// Register catch-all OIDC provider routes last
	r.PathPrefix("/oidc/").Handler(http.StripPrefix("/oidc", provider))
}

// handleCallback completes the authorization flow after successful login
func (c *OIDCController) handleCallback(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())

	// Parse query parameters using UseQuery pattern
	query, err := composables.UseQuery(&CallbackQueryDTO{}, r)
	if err != nil {
		logger.WithError(err).Error("Failed to parse callback query parameters")
		http.Error(w, "Invalid query parameters", http.StatusBadRequest)
		return
	}

	if query.ID == "" {
		logger.Error("Missing auth request ID in callback")
		http.Error(w, "Missing auth request ID", http.StatusBadRequest)
		return
	}

	// Get auth request to validate it exists and is authenticated
	authReq, err := c.oidcService.GetAuthRequest(r.Context(), query.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to get auth request")
		http.Error(w, "Invalid auth request", http.StatusBadRequest)
		return
	}

	// Check if auth request is authenticated
	if !authReq.IsAuthenticated() {
		logger.Error("Auth request not authenticated")
		http.Error(w, "Auth request not authenticated", http.StatusUnauthorized)
		return
	}

	// Redirect back to the OIDC authorize endpoint to complete the flow
	// The zitadel library will handle generating the authorization code and redirecting to client
	redirectURL := fmt.Sprintf("/oidc/authorize?id=%s", query.ID)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
