package controllers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/testkit/domain/schemas"
	"github.com/iota-uz/iota-sdk/modules/testkit/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	tf "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"golang.org/x/crypto/bcrypt"
)

// OTPCache stores plaintext OTP codes temporarily for testing
type OTPCache struct {
	mu    sync.RWMutex
	codes map[string]otpCacheEntry // key: userID or identifier
}

type otpCacheEntry struct {
	code      string
	expiresAt time.Time
}

func newOTPCache() *OTPCache {
	cache := &OTPCache{
		codes: make(map[string]otpCacheEntry),
	}
	// Start cleanup goroutine
	go cache.cleanupExpired()
	return cache
}

func (c *OTPCache) set(key string, code string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.codes[key] = otpCacheEntry{
		code:      code,
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *OTPCache) get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, exists := c.codes[key]
	if !exists || entry.expiresAt.Before(time.Now()) {
		return "", false
	}
	return entry.code, true
}

func (c *OTPCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.codes {
			if entry.expiresAt.Before(now) {
				delete(c.codes, key)
			}
		}
		c.mu.Unlock()
	}
}

type TestEndpointsController struct {
	app         application.Application
	testService *services.TestDataService
	otpCache    *OTPCache
	mutationMu  sync.Mutex
}

func NewTestEndpointsController(app application.Application) application.Controller {
	return &TestEndpointsController{
		app:         app,
		testService: services.NewTestDataService(app),
		otpCache:    newOTPCache(),
	}
}

func (c *TestEndpointsController) Key() string {
	return "/__test__"
}

func (c *TestEndpointsController) Register(r *mux.Router) {
	// Additional safety check in middleware
	r.Use(c.testEndpointsMiddleware)

	// Reset endpoint - truncates all data
	r.HandleFunc("/__test__/reset", c.handleReset).Methods(http.MethodPost)

	// Populate endpoint - accepts JSON data specification
	r.HandleFunc("/__test__/populate", c.handlePopulate).Methods(http.MethodPost)

	// Seed endpoint - applies preset scenarios
	r.HandleFunc("/__test__/seed", c.handleSeed).Methods(http.MethodPost)
	r.HandleFunc("/__test__/seed", c.handleListSeedScenarios).Methods(http.MethodGet)

	// OTP endpoints for testing - generate and retrieve plaintext OTP codes
	r.HandleFunc("/__test__/otp/{userID}", c.handleGenerateOTP).Methods(http.MethodPost)
	r.HandleFunc("/__test__/otp/{userID}", c.handleGetOTP).Methods(http.MethodGet)
	r.HandleFunc("/__test__/otp", c.handleGetOTPByIdentifier).Methods(http.MethodGet)

	// Health check for test endpoints
	r.HandleFunc("/__test__/health", c.handleHealth).Methods(http.MethodGet)
}

func (c *TestEndpointsController) testEndpointsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conf := configuration.Use()
		logger := composables.UseLogger(r.Context())

		if !conf.EnableTestEndpoints {
			logger.Warn("Test endpoints accessed but not enabled")
			http.Error(w, "Test endpoints not enabled", http.StatusNotFound)
			return
		}

		logger.Debug("Test endpoint accessed: " + r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (c *TestEndpointsController) handleReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)
	c.mutationMu.Lock()
	defer c.mutationMu.Unlock()

	type resetRequest struct {
		ReseedMinimal bool `json:"reseedMinimal,omitempty"`
	}

	var req resetRequest
	if r.Body != http.NoBody {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
		}
	}

	logger.Warn("Resetting test database")

	c.mutationMu.Lock()
	err := c.testService.ResetDatabase(ctx, req.ReseedMinimal)
	c.mutationMu.Unlock()
	if err != nil {
		logger.WithError(err).Error("Failed to reset database")
		http.Error(w, "Failed to reset database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success":       true,
		"message":       "Database reset successfully",
		"reseedMinimal": req.ReseedMinimal,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode response")
	}
}

func (c *TestEndpointsController) handlePopulate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)
	c.mutationMu.Lock()
	defer c.mutationMu.Unlock()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	req, err := schemas.ParsePopulateRequest(body)
	if err != nil {
		logger.WithError(err).Error("Failed to parse populate request")
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	logger.WithField("version", req.Version).Info("Populating test data")

	c.mutationMu.Lock()
	result, err := c.testService.PopulateData(ctx, req)
	c.mutationMu.Unlock()
	if err != nil {
		logger.WithError(err).Error("Failed to populate data")
		response := schemas.PopulateResponse{
			Success: false,
			Error:   err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
			logger.WithError(encodeErr).Error("Failed to encode error response")
		}
		return
	}

	response := schemas.PopulateResponse{
		Success: true,
		Message: "Data populated successfully",
		Data:    result,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode response")
	}
}

func (c *TestEndpointsController) handleSeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)
	c.mutationMu.Lock()
	defer c.mutationMu.Unlock()

	type seedRequest struct {
		Scenario string `json:"scenario"`
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req seedRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Scenario == "" {
		req.Scenario = "minimal"
	}

	logger.WithField("scenario", req.Scenario).Info("Seeding test data")

	c.mutationMu.Lock()
	err = c.testService.SeedScenario(ctx, req.Scenario)
	c.mutationMu.Unlock()
	if err != nil {
		logger.WithError(err).Error("Failed to seed scenario")
		http.Error(w, "Failed to seed scenario: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success":  true,
		"message":  "Scenario seeded successfully",
		"scenario": req.Scenario,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode response")
	}
}

func (c *TestEndpointsController) handleListSeedScenarios(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)
	scenarios := c.testService.GetAvailableScenarios()

	response := map[string]interface{}{
		"success":   true,
		"scenarios": scenarios,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode response")
	}
}

func (c *TestEndpointsController) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)
	conf := configuration.Use()

	response := map[string]interface{}{
		"success": true,
		"message": "Test endpoints are healthy",
		"config": map[string]interface{}{
			"enableTestEndpoints": conf.EnableTestEndpoints,
			"environment":         conf.GoAppEnvironment,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode response")
	}
}

// generateOTPCode generates a random N-digit OTP code
func (c *TestEndpointsController) generateOTPCode(length int) (string, error) {
	if length < 4 || length > 10 {
		return "", fmt.Errorf("OTP length must be between 4 and 10")
	}

	maxValue := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, maxValue)
	if err != nil {
		return "", err
	}

	// Format with leading zeros
	format := fmt.Sprintf("%%0%dd", length)
	return fmt.Sprintf(format, n), nil
}

// handleGenerateOTP generates an OTP code for a user and stores it in the database
func (c *TestEndpointsController) handleGenerateOTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)
	vars := mux.Vars(r)

	userIDStr := vars["userID"]
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	type generateOTPRequest struct {
		Identifier string `json:"identifier"` // phone or email
		Channel    string `json:"channel"`    // "sms" or "email"
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req generateOTPRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Identifier == "" {
		http.Error(w, "Identifier is required", http.StatusBadRequest)
		return
	}

	if req.Channel != "sms" && req.Channel != "email" {
		http.Error(w, "Channel must be 'sms' or 'email'", http.StatusBadRequest)
		return
	}

	// Get tenant ID from context or use default test tenant
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		// Use default test tenant ID
		tenantID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	}

	// Generate OTP code (6 digits by default)
	conf := configuration.Use()
	codeLength := conf.TwoFactorAuth.OTPCodeLength
	if codeLength == 0 {
		codeLength = 6
	}

	code, err := c.generateOTPCode(codeLength)
	if err != nil {
		logger.WithError(err).Error("Failed to generate OTP code")
		http.Error(w, "Failed to generate OTP", http.StatusInternalServerError)
		return
	}

	// Hash the code with bcrypt
	hashedCode, err := bcrypt.GenerateFromPassword([]byte(code), 10)
	if err != nil {
		logger.WithError(err).Error("Failed to hash OTP code")
		http.Error(w, "Failed to hash OTP", http.StatusInternalServerError)
		return
	}

	// Determine channel
	var channel tf.OTPChannel
	if req.Channel == "sms" {
		channel = tf.ChannelSMS
	} else {
		channel = tf.ChannelEmail
	}

	// Calculate expiration (5 minutes by default)
	ttlSeconds := conf.TwoFactorAuth.OTPTTLSeconds
	if ttlSeconds == 0 {
		ttlSeconds = 300
	}
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	// Create OTP entity
	otpEntity := twofactor.NewOTP(
		req.Identifier,
		string(hashedCode),
		channel,
		expiresAt,
		tenantID,
		uint(userID),
	)

	// Store in database via OTP repository
	otpRepo := persistence.NewOTPRepository()
	if err := otpRepo.Create(ctx, otpEntity); err != nil {
		logger.WithError(err).Error("Failed to store OTP in database")
		http.Error(w, "Failed to store OTP", http.StatusInternalServerError)
		return
	}

	// Store plaintext code in memory cache for test retrieval
	c.otpCache.set(userIDStr, code, time.Duration(ttlSeconds)*time.Second)
	c.otpCache.set(req.Identifier, code, time.Duration(ttlSeconds)*time.Second)

	logger.WithField("userID", userID).WithField("identifier", req.Identifier).Info("Generated OTP for testing")

	response := map[string]interface{}{
		"success": true,
		"code":    code,
		"message": "OTP generated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode response")
	}
}

// handleGetOTP retrieves a plaintext OTP code for a user from the cache
func (c *TestEndpointsController) handleGetOTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)
	vars := mux.Vars(r)

	userIDStr := vars["userID"]

	code, exists := c.otpCache.get(userIDStr)
	if !exists {
		logger.WithField("userID", userIDStr).Warn("OTP not found in cache")
		http.Error(w, "OTP not found or expired", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"code":    code,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode response")
	}
}

// handleGetOTPByIdentifier retrieves a plaintext OTP code by identifier (phone/email) from the cache
func (c *TestEndpointsController) handleGetOTPByIdentifier(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := composables.UseLogger(ctx)

	identifier := r.URL.Query().Get("identifier")
	if identifier == "" {
		http.Error(w, "identifier parameter is required", http.StatusBadRequest)
		return
	}

	code, exists := c.otpCache.get(identifier)
	if !exists {
		logger.WithField("identifier", identifier).Warn("OTP not found in cache")
		http.Error(w, "OTP not found or expired", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"code":    code,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode response")
	}
}
