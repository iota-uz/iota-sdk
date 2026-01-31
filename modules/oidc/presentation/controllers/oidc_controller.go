package controllers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	"github.com/iota-uz/iota-sdk/modules/oidc/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

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
	// Convert crypto key to [32]byte array
	var cryptoKey [32]byte
	copy(cryptoKey[:], []byte(c.config.CryptoKey))

	// Initialize zitadel OIDC provider configuration
	providerConfig := &op.Config{
		CryptoKey:                cryptoKey,
		DefaultLogoutRedirectURI: "/",
		CodeMethodS256:           true, // Enable PKCE with S256
	}

	// Initialize zitadel OIDC provider
	provider, err := op.NewOpenIDProvider(
		c.config.IssuerURL,
		providerConfig,
		c.storage,
		// op.WithAllowInsecure() can be added for development without HTTPS
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize OIDC provider: %v", err))
	}

	c.provider = provider

	// Mount OIDC provider routes
	// This provides:
	// - /.well-known/openid-configuration (discovery endpoint)
	// - /authorize (authorization endpoint)
	// - /oauth/token (token endpoint)
	// - /userinfo (userinfo endpoint)
	// - /keys (JWKS endpoint)
	// - /revoke (revocation endpoint)
	// - /end_session (logout endpoint)
	// - /callback (auth callback after login)
	r.PathPrefix("/.well-known/").Handler(provider.HttpHandler())
	r.PathPrefix("/oidc/").Handler(http.StripPrefix("/oidc", provider.HttpHandler()))

	// Register custom handler for completing login flow
	// This is called after user successfully logs in via /login
	r.HandleFunc("/oidc/authorize/callback", c.handleCallback).Methods(http.MethodGet)
}

// handleCallback completes the authorization flow after successful login
func (c *OIDCController) handleCallback(w http.ResponseWriter, r *http.Request) {
	const op serrors.Op = "OIDCController.handleCallback"
	logger := composables.UseLogger(r.Context())

	// Get auth request ID from query params
	authRequestID := r.URL.Query().Get("id")
	if authRequestID == "" {
		logger.Error("Missing auth request ID in callback")
		http.Error(w, "Missing auth request ID", http.StatusBadRequest)
		return
	}

	// Get auth request to validate it exists and is authenticated
	authReq, err := c.oidcService.GetAuthRequest(r.Context(), authRequestID)
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
	redirectURL := fmt.Sprintf("/oidc/authorize?id=%s", authRequestID)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
