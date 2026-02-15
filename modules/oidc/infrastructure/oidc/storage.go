package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/crypto/bcrypt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/authrequest"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/token"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Storage implements op.Storage interface from zitadel/oidc
// This is the bridge between zitadel's OIDC library and IOTA SDK's infrastructure
type Storage struct {
	clientRepo           client.Repository
	authRequestRepo      authrequest.Repository
	tokenRepo            token.Repository
	userRepo             user.Repository
	db                   *pgxpool.Pool
	cryptoKey            string
	issuerURL            string
	accessTokenLifetime  time.Duration
	refreshTokenLifetime time.Duration
}

// NewStorage creates a new OIDC storage adapter
func NewStorage(
	clientRepo client.Repository,
	authRequestRepo authrequest.Repository,
	tokenRepo token.Repository,
	userRepo user.Repository,
	db *pgxpool.Pool,
	cryptoKey string,
	issuerURL string,
	accessTokenLifetime time.Duration,
	refreshTokenLifetime time.Duration,
) *Storage {
	// Apply defaults if zero values provided
	if accessTokenLifetime == 0 {
		accessTokenLifetime = time.Hour // Default 1 hour
	}
	if refreshTokenLifetime == 0 {
		refreshTokenLifetime = 30 * 24 * time.Hour // Default 30 days
	}

	return &Storage{
		clientRepo:           clientRepo,
		authRequestRepo:      authRequestRepo,
		tokenRepo:            tokenRepo,
		userRepo:             userRepo,
		db:                   db,
		cryptoKey:            cryptoKey,
		issuerURL:            issuerURL,
		accessTokenLifetime:  accessTokenLifetime,
		refreshTokenLifetime: refreshTokenLifetime,
	}
}

// GetClientByClientID retrieves a client by its client_id
func (s *Storage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	const op serrors.Op = "Storage.GetClientByClientID"

	// Note: Transaction context is handled by repository via composables.UseTx(ctx)
	// if the caller wraps the context with a transaction
	c, err := s.clientRepo.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Return domain client which implements op.Client interface
	return &oidcClient{Client: c}, nil
}

// AuthorizeClientIDSecret validates client credentials
func (s *Storage) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	const op serrors.Op = "Storage.AuthorizeClientIDSecret"

	// Get client by client_id
	c, err := s.clientRepo.GetByClientID(ctx, clientID)
	if err != nil {
		return serrors.E(op, err)
	}

	// Check if client is active
	if !c.IsActive() {
		return serrors.E(op, serrors.PermissionDenied, "client is not active")
	}

	// Verify client secret
	// Public clients (PKCE-only) have no secret
	if c.ClientSecretHash() == nil {
		// Public client - no secret required
		return nil
	}

	// Compare provided secret with bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(*c.ClientSecretHash()), []byte(clientSecret)); err != nil {
		return serrors.E(op, serrors.PermissionDenied, "invalid client secret")
	}

	return nil
}

// CreateAuthRequest stores a new authorization request
func (s *Storage) CreateAuthRequest(ctx context.Context, authReq *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	const operation serrors.Op = "Storage.CreateAuthRequest"

	// Parse userID if provided (empty for unauthenticated requests)
	var parsedUserID *int
	var tenantID *uuid.UUID
	if userID != "" {
		uid, err := parseUserID(userID)
		if err != nil {
			return nil, serrors.E(operation, serrors.KindValidation, "invalid user ID", err)
		}
		intUID := int(uid)
		parsedUserID = &intUID

		// Fetch user to get tenant_id
		u, err := s.userRepo.GetByID(ctx, uid)
		if err != nil {
			return nil, serrors.E(operation, err)
		}
		tid := u.TenantID()
		tenantID = &tid
	}

	// Create domain auth request
	opts := []authrequest.Option{}

	if authReq.State != "" {
		opts = append(opts, authrequest.WithState(authReq.State))
	}

	if authReq.Nonce != "" {
		opts = append(opts, authrequest.WithNonce(authReq.Nonce))
	}

	if authReq.CodeChallenge != "" {
		opts = append(opts, authrequest.WithCodeChallenge(
			authReq.CodeChallenge,
			string(authReq.CodeChallengeMethod),
		))
	}

	if parsedUserID != nil {
		opts = append(opts, authrequest.WithUserID(*parsedUserID))
	}

	if tenantID != nil {
		opts = append(opts, authrequest.WithTenantID(*tenantID))
		opts = append(opts, authrequest.WithAuthTime(time.Now()))
	}

	domainAuthReq := authrequest.New(
		authReq.ClientID,
		authReq.RedirectURI,
		authReq.Scopes,
		string(authReq.ResponseType),
		opts...,
	)

	// Store in repository
	if err := s.authRequestRepo.Create(ctx, domainAuthReq); err != nil {
		return nil, serrors.E(operation, err)
	}

	// Return as op.AuthRequest wrapper
	return &oidcAuthRequest{authRequest: domainAuthReq}, nil
}

// AuthRequestByID retrieves an auth request by its ID
func (s *Storage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	const operation serrors.Op = "Storage.AuthRequestByID"

	// Parse UUID
	authID, err := uuid.Parse(id)
	if err != nil {
		return nil, serrors.E(operation, serrors.KindValidation, "invalid auth request ID", err)
	}

	// Retrieve from repository
	authReq, err := s.authRequestRepo.GetByID(ctx, authID)
	if err != nil {
		return nil, serrors.E(operation, err)
	}

	// Check if expired
	if authReq.IsExpired() {
		return nil, serrors.E(operation, serrors.KindValidation, "auth request has expired")
	}

	// Return as op.AuthRequest wrapper
	return &oidcAuthRequest{authRequest: authReq}, nil
}

// AuthRequestByCode retrieves an auth request by its authorization code
// The code is cryptographically random and generated by the zitadel library
func (s *Storage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	const operation serrors.Op = "Storage.AuthRequestByCode"

	// Retrieve auth request by code
	authReq, err := s.authRequestRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, serrors.E(operation, err)
	}

	// Check if expired
	if authReq.IsExpired() {
		return nil, serrors.E(operation, serrors.KindValidation, "auth request has expired")
	}

	// Check if code was already used (replay attack prevention)
	if authReq.IsCodeUsed() {
		return nil, serrors.E(operation, serrors.KindValidation, "authorization code already used")
	}

	// Atomically mark code as used (one-time use per RFC 6749)
	if err := s.authRequestRepo.MarkCodeUsed(ctx, code); err != nil {
		return nil, serrors.E(operation, err)
	}

	// Return as op.AuthRequest wrapper
	return &oidcAuthRequest{authRequest: authReq}, nil
}

// SaveAuthCode saves the cryptographic authorization code for an auth request
// The code is generated by the zitadel library and passed to this method for storage
func (s *Storage) SaveAuthCode(ctx context.Context, id, code string) error {
	const operation serrors.Op = "Storage.SaveAuthCode"

	// Validate auth request ID
	authID, err := uuid.Parse(id)
	if err != nil {
		return serrors.E(operation, serrors.KindValidation, "invalid auth request ID", err)
	}

	// Store the cryptographic code in the database
	if err := s.authRequestRepo.SaveCode(ctx, authID, code); err != nil {
		return serrors.E(operation, err)
	}

	return nil
}

// DeleteAuthRequest removes an auth request
func (s *Storage) DeleteAuthRequest(ctx context.Context, id string) error {
	const operation serrors.Op = "Storage.DeleteAuthRequest"

	authID, err := uuid.Parse(id)
	if err != nil {
		return serrors.E(operation, serrors.KindValidation, "invalid auth request ID", err)
	}

	// Delete from repository (transaction is handled by repository composables.UseTx)
	return s.authRequestRepo.Delete(ctx, authID)
}

// CreateAccessToken generates a new access token
func (s *Storage) CreateAccessToken(ctx context.Context, req op.TokenRequest) (string, time.Time, error) {
	const operation serrors.Op = "Storage.CreateAccessToken"

	// Get signing key
	privateKey, keyID, err := GetActiveSigningKey(ctx, s.db, s.cryptoKey)
	if err != nil {
		return "", time.Time{}, serrors.E(operation, fmt.Errorf("failed to get signing key: %w", err))
	}

	// Calculate expiration using configured lifetime
	now := time.Now()
	expiresAt := now.Add(s.accessTokenLifetime)

	// Create JWT claims
	claims := jwt.MapClaims{
		"iss":   s.issuerURL,
		"sub":   req.GetSubject(),
		"aud":   req.GetAudience(),
		"exp":   expiresAt.Unix(),
		"iat":   now.Unix(),
		"scope": req.GetScopes(),
	}

	// Create and sign token
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	jwtToken.Header["kid"] = keyID

	tokenString, err := jwtToken.SignedString(privateKey)
	if err != nil {
		return "", time.Time{}, serrors.E(operation, fmt.Errorf("failed to sign token: %w", err))
	}

	return tokenString, expiresAt, nil
}

// CreateAccessAndRefreshTokens generates both access and refresh tokens
func (s *Storage) CreateAccessAndRefreshTokens(
	ctx context.Context,
	req op.TokenRequest,
	refreshToken string,
) (string, string, time.Time, error) {
	const operation serrors.Op = "Storage.CreateAccessAndRefreshTokens"

	// Generate access token JWT
	accessToken, expiresAt, err := s.CreateAccessToken(ctx, req)
	if err != nil {
		return "", "", time.Time{}, serrors.E(operation, err)
	}

	// Hash refresh token with SHA-256
	hash := sha256.Sum256([]byte(refreshToken))
	tokenHash := hex.EncodeToString(hash[:])

	// Parse user ID from subject
	userID, err := parseUserID(req.GetSubject())
	if err != nil {
		return "", "", time.Time{}, serrors.E(operation, err)
	}

	// Get user to fetch tenant_id
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", "", time.Time{}, serrors.E(operation, err)
	}

	// Create domain refresh token using configured lifetime

	// Get auth time and AMR from request
	// The zitadel library passes extended request objects with these methods
	authTime := time.Now()
	amr := []string{"pwd"} // Default authentication method
	clientID := ""         // Will be extracted from audience if available

	// Type assertion to get additional claims if available
	if authReq, ok := req.(interface {
		GetAuthTime() time.Time
		GetAMR() []string
	}); ok {
		authTime = authReq.GetAuthTime()
		if len(authReq.GetAMR()) > 0 {
			amr = authReq.GetAMR()
		}
	}

	// Extract client ID from audience (first element is typically the client ID)
	if len(req.GetAudience()) > 0 {
		clientID = req.GetAudience()[0]
	}

	domainToken := token.New(
		tokenHash,
		clientID,
		int(userID),
		u.TenantID(),
		req.GetScopes(),
		authTime,
		s.refreshTokenLifetime,
		token.WithAudience(req.GetAudience()),
		token.WithAMR(amr),
	)

	// Store refresh token in repository
	if err := s.tokenRepo.Create(ctx, domainToken); err != nil {
		return "", "", time.Time{}, serrors.E(operation, err)
	}

	return accessToken, refreshToken, expiresAt, nil
}

// TokenRequestByRefreshToken retrieves token request data by refresh token
func (s *Storage) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	const operation serrors.Op = "Storage.TokenRequestByRefreshToken"

	// Hash the refresh token to look up in database
	hash := sha256.Sum256([]byte(refreshToken))
	tokenHash := hex.EncodeToString(hash[:])

	// Get token from repository (transaction handled by composables.UseTx in repository)
	t, err := s.tokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, serrors.E(operation, err)
	}

	// Check if token is expired
	if t.IsExpired() {
		return nil, serrors.E(operation, serrors.KindValidation, "refresh token has expired")
	}

	// Map to op.RefreshTokenRequest interface
	return &refreshTokenRequest{token: t}, nil
}

// GetRefreshTokenInfo retrieves user and token IDs from a refresh token
// This method is used for token revocation and validation
func (s *Storage) GetRefreshTokenInfo(ctx context.Context, clientID string, tokenValue string) (string, string, error) {
	const operation serrors.Op = "Storage.GetRefreshTokenInfo"

	// Hash the refresh token to look up in database
	hash := sha256.Sum256([]byte(tokenValue))
	tokenHash := hex.EncodeToString(hash[:])

	// Get token from repository (transaction handled by composables.UseTx)
	t, err := s.tokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return "", "", serrors.E(operation, err)
	}

	// Verify client ID matches
	if t.ClientID() != clientID {
		return "", "", serrors.E(operation, serrors.PermissionDenied, "client ID mismatch")
	}

	// Return user ID and token ID
	return fmt.Sprintf("%d", t.UserID()), t.ID().String(), nil
}

// RevokeToken revokes a token (access or refresh)
func (s *Storage) RevokeToken(ctx context.Context, tokenOrTokenID string, userID string, clientID string) *oidc.Error {
	// If userID is provided, this is an access token ID
	// If userID is empty, this is a refresh token value
	if userID != "" {
		// Access token revocation - tokenOrTokenID is the token ID
		// JWTs cannot be revoked server-side without a blacklist
		// This would require implementing a token blacklist in the database
		// For now, we rely on short token lifetimes
		return nil
	}

	// Refresh token revocation - tokenOrTokenID is the token value
	// Hash the refresh token to look up in database
	hash := sha256.Sum256([]byte(tokenOrTokenID))
	tokenHash := hex.EncodeToString(hash[:])

	// Get token from repository (transaction handled by composables.UseTx)
	t, err := s.tokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		// Return OIDC error for invalid token
		return oidc.ErrInvalidGrant().WithDescription("invalid refresh token")
	}

	// Verify client ID matches
	if t.ClientID() != clientID {
		return oidc.ErrInvalidGrant().WithDescription("client ID mismatch")
	}

	// Delete the refresh token
	if err := s.tokenRepo.Delete(ctx, t.ID()); err != nil {
		// Return generic server error without exposing internal details
		return oidc.ErrServerError()
	}

	return nil
}

// TerminateSession terminates a user's session for a specific client
func (s *Storage) TerminateSession(ctx context.Context, userID string, clientID string) error {
	const operation serrors.Op = "Storage.TerminateSession"

	// Parse user ID
	uid, err := parseUserID(userID)
	if err != nil {
		return serrors.E(operation, serrors.KindValidation, "invalid user ID", err)
	}

	// Delete all refresh tokens for this user + client combination
	if err := s.tokenRepo.DeleteByUserAndClient(ctx, int(uid), clientID); err != nil {
		return serrors.E(operation, err)
	}

	return nil
}

// GetSigningKey returns the active signing key for JWT signing
// Deprecated: use KeySet instead
func (s *Storage) GetSigningKey(ctx context.Context, keyCh chan<- jose.SigningKey) {
	// Retrieve active signing key
	privateKey, keyID, err := GetActiveSigningKey(ctx, s.db, s.cryptoKey)
	if err != nil {
		// Log error and close channel to signal failure
		log.Printf("OIDC: Failed to get signing key: %v", err)
		close(keyCh)
		return
	}

	// Create signing key
	signingKey := jose.SigningKey{
		Algorithm: jose.RS256,
		Key: &jose.JSONWebKey{
			Key:       privateKey,
			KeyID:     keyID,
			Algorithm: string(jose.RS256),
			Use:       "sig",
		},
	}

	keyCh <- signingKey
	close(keyCh)
}

// SigningKey returns the active signing key for JWT signing
func (s *Storage) SigningKey(ctx context.Context) (op.SigningKey, error) {
	const operation serrors.Op = "Storage.SigningKey"

	// Retrieve active signing key
	privateKey, keyID, err := GetActiveSigningKey(ctx, s.db, s.cryptoKey)
	if err != nil {
		return nil, serrors.E(operation, err)
	}

	// Return signing key wrapper
	return &signingKeyAdapter{
		id:        keyID,
		algorithm: jose.RS256,
		key:       privateKey,
	}, nil
}

// SignatureAlgorithms returns the supported signature algorithms
func (s *Storage) SignatureAlgorithms(ctx context.Context) ([]jose.SignatureAlgorithm, error) {
	// We only support RS256 for now
	return []jose.SignatureAlgorithm{jose.RS256}, nil
}

// KeySet returns all active signing keys
func (s *Storage) KeySet(ctx context.Context) ([]op.Key, error) {
	const operation serrors.Op = "Storage.KeySet"

	// Retrieve active signing key
	privateKey, keyID, err := GetActiveSigningKey(ctx, s.db, s.cryptoKey)
	if err != nil {
		return nil, serrors.E(operation, err)
	}

	// Create key wrapper
	key := &signingKey{
		id:        keyID,
		algorithm: jose.RS256,
		use:       "sig",
		key:       privateKey,
	}

	return []op.Key{key}, nil
}

// GetKeySet returns the JSON Web Key Set for public key verification
func (s *Storage) GetKeySet(ctx context.Context) (*jose.JSONWebKeySet, error) {
	const op serrors.Op = "Storage.GetKeySet"

	// Get public keys with their key IDs from database
	keysWithIDs, err := GetPublicKeysWithIDs(ctx, s.db)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	keySet := &jose.JSONWebKeySet{
		Keys: make([]jose.JSONWebKey, len(keysWithIDs)),
	}

	for i, keyWithID := range keysWithIDs {
		keySet.Keys[i] = jose.JSONWebKey{
			Key:       keyWithID.PublicKey,
			KeyID:     keyWithID.KeyID, // Use actual key_id from database
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}
	}

	return keySet, nil
}

// SaveNewKeyPair generates and stores a new signing key pair
func (s *Storage) SaveNewKeyPair(ctx context.Context) error {
	// Phase 2: Implement proper key rotation (generate new key, mark old as inactive)
	// For now, use BootstrapKeys which only generates if no keys exist
	return BootstrapKeys(ctx, s.db, s.cryptoKey)
}

// Health performs a health check on the storage
func (s *Storage) Health(ctx context.Context) error {
	const op serrors.Op = "Storage.Health"

	// Ping the database
	if err := s.db.Ping(ctx); err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// SetUserinfoFromScopes maps IOTA SDK User entity to OIDC claims
// Deprecated: use SetUserinfoFromToken instead
func (s *Storage) SetUserinfoFromScopes(
	ctx context.Context,
	userinfo *oidc.UserInfo,
	userID, clientID string,
	scopes []string,
) error {
	const op serrors.Op = "Storage.SetUserinfoFromScopes"

	// Parse user ID
	uid, err := parseUserID(userID)
	if err != nil {
		return serrors.E(op, err)
	}

	// Note: Transaction context is handled by repository via composables.UseTx(ctx)
	// if the caller wraps the context with a transaction
	u, err := s.userRepo.GetByID(ctx, uid)
	if err != nil {
		return serrors.E(op, err)
	}

	// Set standard claims
	userinfo.Subject = userID

	// Map scopes to claims
	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeEmail:
			userinfo.Email = u.Email().Value()
			// SECURITY: Set to false until proper email verification is implemented
			// Do not use this claim for authorization decisions in Phase 1
			userinfo.EmailVerified = oidc.Bool(false)

		case oidc.ScopeProfile:
			userinfo.GivenName = u.FirstName()
			userinfo.FamilyName = u.LastName()
			if u.MiddleName() != "" {
				userinfo.MiddleName = u.MiddleName()
			}
			// Phase 2: Add extended profile fields (locale, picture, timezone, etc.)

		case oidc.ScopePhone:
			if u.Phone() != nil {
				userinfo.PhoneNumber = u.Phone().Value()
				// SECURITY: Set to false until proper phone verification is implemented
				// Do not use this claim for authorization decisions in Phase 1
				userinfo.PhoneNumberVerified = oidc.Bool(false)
			}

		// Custom scopes
		case "tenant_id":
			// Add tenant_id as custom claim
			userinfo.AppendClaims("tenant_id", u.TenantID().String())

		case "roles":
			// Add roles as custom claim
			roleNames := make([]string, len(u.Roles()))
			for i, role := range u.Roles() {
				roleNames[i] = role.Name()
			}
			userinfo.AppendClaims("roles", roleNames)
		}
	}

	return nil
}

// SetUserinfoFromToken sets userinfo claims from a token
func (s *Storage) SetUserinfoFromToken(
	ctx context.Context,
	userinfo *oidc.UserInfo,
	tokenID, subject, origin string,
) error {
	const op serrors.Op = "Storage.SetUserinfoFromToken"

	// Parse user ID from subject
	uid, err := parseUserID(subject)
	if err != nil {
		return serrors.E(op, err)
	}

	// Fetch user from repository
	u, err := s.userRepo.GetByID(ctx, uid)
	if err != nil {
		return serrors.E(op, err)
	}

	// Set standard claims
	userinfo.Subject = subject
	userinfo.Email = u.Email().Value()
	// SECURITY: Set to false until proper email verification is implemented
	// Do not use this claim for authorization decisions in Phase 1
	userinfo.EmailVerified = oidc.Bool(false)
	userinfo.GivenName = u.FirstName()
	userinfo.FamilyName = u.LastName()
	if u.MiddleName() != "" {
		userinfo.MiddleName = u.MiddleName()
	}
	if u.Phone() != nil {
		userinfo.PhoneNumber = u.Phone().Value()
		// SECURITY: Set to false until proper phone verification is implemented
		// Do not use this claim for authorization decisions in Phase 1
		userinfo.PhoneNumberVerified = oidc.Bool(false)
	}

	return nil
}

// SetIntrospectionFromToken sets introspection response from a token
func (s *Storage) SetIntrospectionFromToken(
	ctx context.Context,
	introspection *oidc.IntrospectionResponse,
	tokenID, subject, clientID string,
) error {
	const op serrors.Op = "Storage.SetIntrospectionFromToken"

	// Parse user ID from subject
	uid, err := parseUserID(subject)
	if err != nil {
		return serrors.E(op, err)
	}

	// Fetch user from repository
	u, err := s.userRepo.GetByID(ctx, uid)
	if err != nil {
		return serrors.E(op, err)
	}

	// Set introspection response
	introspection.Active = true
	introspection.Subject = subject
	introspection.ClientID = clientID
	introspection.Username = u.Email().Value()

	return nil
}

// GetPrivateClaimsFromScopes returns custom private claims based on scopes
func (s *Storage) GetPrivateClaimsFromScopes(
	ctx context.Context,
	userID, clientID string,
	scopes []string,
) (map[string]any, error) {
	const op serrors.Op = "Storage.GetPrivateClaimsFromScopes"

	// Parse user ID
	uid, err := parseUserID(userID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Fetch user from repository
	u, err := s.userRepo.GetByID(ctx, uid)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	claims := make(map[string]any)

	// Add custom claims based on scopes
	for _, scope := range scopes {
		switch scope {
		case "tenant_id":
			claims["tenant_id"] = u.TenantID().String()
		case "roles":
			roleNames := make([]string, len(u.Roles()))
			for i, role := range u.Roles() {
				roleNames[i] = role.Name()
			}
			claims["roles"] = roleNames
		}
	}

	return claims, nil
}

// GetKeyByIDAndClientID retrieves a specific key for a client
func (s *Storage) GetKeyByIDAndClientID(
	ctx context.Context,
	keyID, clientID string,
) (*jose.JSONWebKey, error) {
	const op serrors.Op = "Storage.GetKeyByIDAndClientID"

	// For now, return nil as we use shared signing keys
	// Individual client keys can be implemented later if needed
	return nil, serrors.E(op, fmt.Errorf("client-specific keys not implemented"))
}

// ValidateJWTProfileScopes validates scopes for JWT profile authorization
func (s *Storage) ValidateJWTProfileScopes(
	_ context.Context,
	_ string,
	scopes []string,
) ([]string, error) {
	// For now, return all requested scopes as valid
	// Implement custom validation logic as needed
	return scopes, nil
}

// Helper types to adapt domain entities to op interfaces

// signingKeyAdapter wraps a private key to implement op.SigningKey interface
type signingKeyAdapter struct {
	id        string
	algorithm jose.SignatureAlgorithm
	key       any
}

func (k *signingKeyAdapter) ID() string                                  { return k.id }
func (k *signingKeyAdapter) SignatureAlgorithm() jose.SignatureAlgorithm { return k.algorithm }
func (k *signingKeyAdapter) Key() any                                    { return k.key }

// signingKey wraps a private key to implement op.Key interface
type signingKey struct {
	id        string
	algorithm jose.SignatureAlgorithm
	use       string
	key       any
}

func (k *signingKey) ID() string                         { return k.id }
func (k *signingKey) Algorithm() jose.SignatureAlgorithm { return k.algorithm }
func (k *signingKey) Use() string                        { return k.use }
func (k *signingKey) Key() any                           { return k.key }

// oidcClient wraps client.Client to implement op.Client interface
type oidcClient struct {
	client.Client
}

func (c *oidcClient) GetID() string {
	return c.ClientID()
}

func (c *oidcClient) RedirectURIs() []string {
	return c.Client.RedirectURIs()
}

func (c *oidcClient) PostLogoutRedirectURIs() []string {
	// Phase 2: Add post_logout_redirect_uris field to client entity and validation
	return []string{}
}

func (c *oidcClient) ApplicationType() op.ApplicationType {
	switch c.Client.ApplicationType() {
	case "web":
		return op.ApplicationTypeWeb
	case "native":
		return op.ApplicationTypeNative
	case "user_agent":
		return op.ApplicationTypeUserAgent
	default:
		return op.ApplicationTypeWeb
	}
}

func (c *oidcClient) AuthMethod() oidc.AuthMethod {
	switch c.Client.AuthMethod() {
	case "client_secret_basic":
		return oidc.AuthMethodBasic
	case "client_secret_post":
		return oidc.AuthMethodPost
	case "none":
		return oidc.AuthMethodNone
	default:
		return oidc.AuthMethodBasic
	}
}

func (c *oidcClient) ResponseTypes() []oidc.ResponseType {
	types := make([]oidc.ResponseType, len(c.Client.ResponseTypes()))
	for i, rt := range c.Client.ResponseTypes() {
		types[i] = oidc.ResponseType(rt)
	}
	return types
}

func (c *oidcClient) GrantTypes() []oidc.GrantType {
	types := make([]oidc.GrantType, len(c.Client.GrantTypes()))
	for i, gt := range c.Client.GrantTypes() {
		types[i] = oidc.GrantType(gt)
	}
	return types
}

func (c *oidcClient) LoginURL(id string) string {
	// Note: Returns login URL with auth request ID for OIDC callback flow
	return fmt.Sprintf("/login?auth_request=%s", id)
}

func (c *oidcClient) AccessTokenType() op.AccessTokenType {
	return op.AccessTokenTypeJWT
}

func (c *oidcClient) IDTokenLifetime() time.Duration {
	return c.Client.IDTokenLifetime()
}

func (c *oidcClient) DevMode() bool {
	return false
}

func (c *oidcClient) RestrictAdditionalIdTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string {
		return scopes
	}
}

func (c *oidcClient) RestrictAdditionalAccessTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string {
		return scopes
	}
}

func (c *oidcClient) IsScopeAllowed(scope string) bool {
	return c.ValidateScope(scope)
}

func (c *oidcClient) IDTokenUserinfoClaimsAssertion() bool {
	return false
}

func (c *oidcClient) ClockSkew() time.Duration {
	return 0
}

// refreshTokenRequest wraps token.RefreshToken to implement op.RefreshTokenRequest
type refreshTokenRequest struct {
	token token.RefreshToken
}

func (r *refreshTokenRequest) GetAMR() []string {
	return r.token.AMR()
}

func (r *refreshTokenRequest) GetAudience() []string {
	return r.token.Audience()
}

func (r *refreshTokenRequest) GetAuthTime() time.Time {
	return r.token.AuthTime()
}

func (r *refreshTokenRequest) GetClientID() string {
	return r.token.ClientID()
}

func (r *refreshTokenRequest) GetScopes() []string {
	return r.token.Scopes()
}

func (r *refreshTokenRequest) GetSubject() string {
	return fmt.Sprintf("%d", r.token.UserID())
}

func (r *refreshTokenRequest) SetCurrentScopes(scopes []string) {
	// Scopes are immutable in our implementation
	// Token refresh uses original scopes from the refresh token
}

// oidcAuthRequest wraps authrequest.AuthRequest to implement op.AuthRequest
type oidcAuthRequest struct {
	authRequest authrequest.AuthRequest
}

func (a *oidcAuthRequest) GetID() string {
	return a.authRequest.ID().String()
}

func (a *oidcAuthRequest) GetACR() string {
	// Authentication Context Class Reference - not implemented yet
	return ""
}

func (a *oidcAuthRequest) GetAMR() []string {
	// Authentication Methods References - default to password
	return []string{"pwd"}
}

func (a *oidcAuthRequest) GetAudience() []string {
	// Audience is typically the client_id
	return []string{a.authRequest.ClientID()}
}

func (a *oidcAuthRequest) GetAuthTime() time.Time {
	if a.authRequest.AuthTime() != nil {
		return *a.authRequest.AuthTime()
	}
	return time.Time{}
}

func (a *oidcAuthRequest) GetClientID() string {
	return a.authRequest.ClientID()
}

func (a *oidcAuthRequest) GetCodeChallenge() *oidc.CodeChallenge {
	if a.authRequest.CodeChallenge() == nil {
		return nil
	}

	var method oidc.CodeChallengeMethod
	if a.authRequest.CodeChallengeMethod() != nil {
		method = oidc.CodeChallengeMethod(*a.authRequest.CodeChallengeMethod())
	}

	return &oidc.CodeChallenge{
		Challenge: *a.authRequest.CodeChallenge(),
		Method:    method,
	}
}

func (a *oidcAuthRequest) GetNonce() string {
	if a.authRequest.Nonce() != nil {
		return *a.authRequest.Nonce()
	}
	return ""
}

func (a *oidcAuthRequest) GetRedirectURI() string {
	return a.authRequest.RedirectURI()
}

func (a *oidcAuthRequest) GetResponseType() oidc.ResponseType {
	return oidc.ResponseType(a.authRequest.ResponseType())
}

func (a *oidcAuthRequest) GetScopes() []string {
	return a.authRequest.Scopes()
}

func (a *oidcAuthRequest) GetState() string {
	if a.authRequest.State() != nil {
		return *a.authRequest.State()
	}
	return ""
}

func (a *oidcAuthRequest) GetSubject() string {
	if a.authRequest.UserID() != nil {
		return fmt.Sprintf("%d", *a.authRequest.UserID())
	}
	return ""
}

func (a *oidcAuthRequest) Done() bool {
	return a.authRequest.IsAuthenticated()
}

func (a *oidcAuthRequest) GetResponseMode() oidc.ResponseMode {
	// Default response mode for authorization code flow
	return oidc.ResponseModeQuery
}

// Helper functions

func parseUserID(userID string) (uint, error) {
	var uid uint
	_, err := fmt.Sscanf(userID, "%d", &uid)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}
	return uid, nil
}
