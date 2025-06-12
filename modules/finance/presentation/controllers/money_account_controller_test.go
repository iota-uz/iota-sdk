package controllers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/antchfx/htmlquery"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	i18n "github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	moneyAccountEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

var (
	MoneyAccountBasePath = "/finance/accounts"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// testFixtures contains common test dependencies
type moneyAccountTestFixtures struct {
	ctx                 context.Context
	tx                  pgx.Tx
	app                 application.Application
	router              *mux.Router
	tenant              *composables.Tenant
	moneyAccountService *services.MoneyAccountService
	adminUser           user.User
}

// setupMoneyAccountTest initializes test dependencies
func setupMoneyAccountTest(t *testing.T) *moneyAccountTestFixtures {
	t.Helper()

	// Create test database
	testutils.CreateDB(t.Name())
	pool := testutils.NewPool(testutils.DbOpts(t.Name()))

	// Setup real application with required modules
	app, err := testutils.SetupApplication(pool, modules.BuiltInModules...)
	require.NoError(t, err, "Failed to setup application")

	ctx := context.Background()
	// Create tenant first before starting transaction
	tenant, err := testutils.CreateTestTenant(ctx, pool)
	require.NoError(t, err, "Failed to create test tenant")

	// Create required currencies before starting transaction (currencies are global)
	currencyCtx := composables.WithPool(ctx, pool)
	currencyRepository := persistence.NewCurrencyRepository()
	err = currencyRepository.Create(currencyCtx, &currency.USD)
	require.NoError(t, err, "Failed to create USD currency")
	err = currencyRepository.Create(currencyCtx, &currency.EUR)
	require.NoError(t, err, "Failed to create EUR currency")

	// Begin transaction after tenant and currency creation
	tx, err := pool.Begin(ctx)
	require.NoError(t, err, "Failed to begin transaction")

	// Add cleanup to commit transaction
	t.Cleanup(func() {
		err := tx.Commit(ctx)
		if err != nil && err != pgx.ErrTxClosed {
			t.Logf("Warning: failed to commit transaction: %v", err)
		}
	})

	// Create context with transaction and tenant
	ctx = composables.WithPool(ctx, pool)
	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithTenantID(ctx, tenant.ID)
	ctx = composables.WithParams(ctx, testutils.DefaultParams())

	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
		user.WithTenantID(tenant.ID),
	)

	ctx = composables.WithUser(ctx, adminUser)
	ctx = composables.WithSession(ctx, &session.Session{})

	// Get service from application
	moneyAccountService := app.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	// Create controller with real application
	controller := controllers.NewMoneyAccountController(app)

	// Create router
	router := mux.NewRouter()

	// Apply test middleware to simulate auth and context
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add test context values that middleware would normally add
			reqCtx := r.Context()
			reqCtx = composables.WithUser(reqCtx, adminUser)
			reqCtx = composables.WithPool(reqCtx, pool)
			reqCtx = composables.WithTx(reqCtx, tx)
			reqCtx = composables.WithSession(reqCtx, &session.Session{})
			reqCtx = context.WithValue(reqCtx, constants.AppKey, app)
			reqCtx = context.WithValue(reqCtx, constants.HeadKey, templ.NopComponent)
			reqCtx = context.WithValue(reqCtx, constants.LogoKey, templ.NopComponent)

			// Important: Add tenant to context
			reqCtx = composables.WithTenantID(reqCtx, tenant.ID)

			// Add logger to context
			logger := logrus.New()
			fieldsLogger := logger.WithFields(logrus.Fields{
				"test": true,
				"path": r.URL.Path,
			})
			reqCtx = context.WithValue(reqCtx, constants.LoggerKey, fieldsLogger)

			// Add params to context (required for auth middleware)
			params := &composables.Params{
				IP:            "127.0.0.1",
				UserAgent:     "test-agent",
				Authenticated: true, // Important for RedirectNotAuthenticated middleware
				Request:       r,
				Writer:        w,
			}
			reqCtx = composables.WithParams(reqCtx, params)

			// Mock localizer context
			localizer := i18n.NewLocalizer(app.Bundle(), "en")

			// Add PageContext
			parsedURL, _ := url.Parse(MoneyAccountBasePath)
			reqCtx = composables.WithPageCtx(reqCtx, &types.PageContext{
				Locale:    language.English,
				URL:       parsedURL,
				Localizer: localizer,
			})

			next.ServeHTTP(w, r.WithContext(reqCtx))
		})
	})

	controller.Register(router)

	return &moneyAccountTestFixtures{
		ctx:                 ctx,
		tx:                  tx,
		app:                 app,
		tenant:              tenant,
		router:              router,
		moneyAccountService: moneyAccountService,
		adminUser:           adminUser,
	}
}

func TestMoneyAccountController_List_Success(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Create test accounts
	balance1 := money.NewFromFloat(1000.50, string(currency.UsdCode))
	account1 := moneyAccountEntity.New(
		"Test Account 1",
		balance1,
		moneyAccountEntity.WithTenantID(fixtures.tenant.ID),
		moneyAccountEntity.WithAccountNumber("ACC001"),
		moneyAccountEntity.WithDescription("Test account 1 description"),
	)

	balance2 := money.NewFromFloat(2500.75, string(currency.EurCode))
	account2 := moneyAccountEntity.New(
		"Test Account 2",
		balance2,
		moneyAccountEntity.WithTenantID(fixtures.tenant.ID),
		moneyAccountEntity.WithAccountNumber("ACC002"),
		moneyAccountEntity.WithDescription("Test account 2 description"),
	)

	// Save accounts
	_, err := fixtures.moneyAccountService.Create(fixtures.ctx, account1)
	require.NoError(t, err)
	_, err = fixtures.moneyAccountService.Create(fixtures.ctx, account2)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodGet, MoneyAccountBasePath, nil)
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	// Assert response
	require.Equal(t, http.StatusOK, rr.Code)

	// Parse HTML and validate structure
	doc, err := htmlquery.Parse(strings.NewReader(rr.Body.String()))
	require.NoError(t, err)

	// Check that accounts are displayed in the table
	accountRows := htmlquery.Find(doc, "//table//tbody//tr")
	assert.GreaterOrEqual(t, len(accountRows), 2, "Should have at least 2 account rows")

	// Check for account names in the response
	pageContent := rr.Body.String()
	assert.Contains(t, pageContent, "Test Account 1")
	assert.Contains(t, pageContent, "Test Account 2")
}

func TestMoneyAccountController_List_HTMX_Request(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Create test account
	balance := money.NewFromFloat(500.00, "USD")
	account := moneyAccountEntity.New(
		"HTMX Test Account",
		balance,
		moneyAccountEntity.WithTenantID(fixtures.tenant.ID),
	)

	_, err := fixtures.moneyAccountService.Create(fixtures.ctx, account)
	require.NoError(t, err)

	// Create HTMX request
	req := httptest.NewRequest(http.MethodGet, MoneyAccountBasePath, nil)
	req.Header.Set("Hx-Request", "true")
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	// Assert response
	require.Equal(t, http.StatusOK, rr.Code)

	// For HTMX requests, should return partial content (table only)
	pageContent := rr.Body.String()
	assert.Contains(t, pageContent, "HTMX Test Account")
}

func TestMoneyAccountController_GetNew_Success(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Create request
	req := httptest.NewRequest(http.MethodGet, MoneyAccountBasePath+"/new", nil)
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	// Assert response
	require.Equal(t, http.StatusOK, rr.Code)

	// Parse HTML and validate form structure
	doc, err := htmlquery.Parse(strings.NewReader(rr.Body.String()))
	require.NoError(t, err)

	// Check for form elements
	nameInput := htmlquery.FindOne(doc, "//input[@name='Name']")
	assert.NotNil(t, nameInput, "Should have Name input field")

	balanceInput := htmlquery.FindOne(doc, "//input[@name='Balance']")
	assert.NotNil(t, balanceInput, "Should have Balance input field")

	currencySelect := htmlquery.FindOne(doc, "//select[@name='CurrencyCode']")
	assert.NotNil(t, currencySelect, "Should have CurrencyCode select field")

	accountNumberInput := htmlquery.FindOne(doc, "//input[@name='AccountNumber']")
	assert.NotNil(t, accountNumberInput, "Should have AccountNumber input field")

	descriptionInput := htmlquery.FindOne(doc, "//textarea[@name='Description']")
	assert.NotNil(t, descriptionInput, "Should have Description textarea field")
}

func TestMoneyAccountController_Create_Success(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Prepare form data
	formData := url.Values{}
	formData.Set("Name", "New Test Account")
	formData.Set("Balance", "1500.25")
	formData.Set("CurrencyCode", "USD")
	formData.Set("AccountNumber", "ACC123")
	formData.Set("Description", "New account description")

	// Create request
	req := httptest.NewRequest(http.MethodPost, MoneyAccountBasePath, strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Logf("Response body: %s", rr.Body.String())
		t.Logf("Response headers: %v", rr.Header())
	}

	// Should redirect after successful creation
	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Equal(t, MoneyAccountBasePath, rr.Header().Get("Location"))

	// Verify account was created
	accounts, err := fixtures.moneyAccountService.GetAll(fixtures.ctx)
	require.NoError(t, err)
	require.Len(t, accounts, 1, "One account should be created")

	savedAccount := accounts[0]
	assert.Equal(t, "New Test Account", savedAccount.Name())
	assert.Equal(t, "ACC123", savedAccount.AccountNumber())
	assert.Equal(t, "New account description", savedAccount.Description())
	assert.Equal(t, "USD", savedAccount.Balance().Currency().Code)
	assert.Equal(t, int64(150025), savedAccount.Balance().Amount())
}

func TestMoneyAccountController_Create_ValidationError(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Prepare invalid form data (missing required fields)
	formData := url.Values{}
	formData.Set("Name", "")                // Required field is empty
	formData.Set("Balance", "-100")         // Negative balance
	formData.Set("CurrencyCode", "INVALID") // Invalid currency code
	formData.Set("AccountNumber", "ACC123")
	formData.Set("Description", "Test description")

	// Create request
	req := httptest.NewRequest(http.MethodPost, MoneyAccountBasePath, strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	// Should return form with errors
	assert.Equal(t, http.StatusOK, rr.Code)

	// Debug: output the response body to see what's actually returned
	t.Logf("Response body: %s", rr.Body.String())

	// Parse HTML and check for error messages
	doc, err := htmlquery.Parse(strings.NewReader(rr.Body.String()))
	require.NoError(t, err)

	// Should contain validation error messages
	errorElements := htmlquery.Find(doc, "//small[@data-testid='field-error']")
	assert.Greater(t, len(errorElements), 0, "Should have validation error indicators")

	// Verify no account was created
	accounts, err := fixtures.moneyAccountService.GetAll(fixtures.ctx)
	require.NoError(t, err)
	assert.Empty(t, accounts, "No accounts should be created with validation errors")
}

func TestMoneyAccountController_GetEdit_Success(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Create test account
	balance := money.NewFromFloat(1000.00, "USD")
	account := moneyAccountEntity.New(
		"Edit Test Account",
		balance,
		moneyAccountEntity.WithTenantID(fixtures.tenant.ID),
		moneyAccountEntity.WithAccountNumber("EDIT001"),
		moneyAccountEntity.WithDescription("Account to edit"),
	)

	createdAccount, err := fixtures.moneyAccountService.Create(fixtures.ctx, account)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String()), nil)
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	// Assert response
	require.Equal(t, http.StatusOK, rr.Code)

	// Parse HTML and validate form pre-population
	doc, err := htmlquery.Parse(strings.NewReader(rr.Body.String()))
	require.NoError(t, err)

	// Check that form is pre-populated with existing values
	nameInput := htmlquery.FindOne(doc, "//input[@name='Name']")
	assert.NotNil(t, nameInput)
	assert.Equal(t, "Edit Test Account", htmlquery.SelectAttr(nameInput, "value"))

	accountNumberInput := htmlquery.FindOne(doc, "//input[@name='AccountNumber']")
	assert.NotNil(t, accountNumberInput)
	assert.Equal(t, "EDIT001", htmlquery.SelectAttr(accountNumberInput, "value"))

	descriptionTextarea := htmlquery.FindOne(doc, "//textarea[@name='Description']")
	assert.NotNil(t, descriptionTextarea)
	assert.Equal(t, "Account to edit", htmlquery.InnerText(descriptionTextarea))
}

func TestMoneyAccountController_GetEdit_NotFound(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Create request with non-existent UUID
	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s", MoneyAccountBasePath, nonExistentID.String()), nil)
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	// Should return error
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestMoneyAccountController_Update_Success(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Create test account
	balance := money.NewFromFloat(500.00, "USD")
	account := moneyAccountEntity.New(
		"Original Account",
		balance,
		moneyAccountEntity.WithTenantID(fixtures.tenant.ID),
		moneyAccountEntity.WithAccountNumber("ORIG001"),
		moneyAccountEntity.WithDescription("Original description"),
	)

	createdAccount, err := fixtures.moneyAccountService.Create(fixtures.ctx, account)
	require.NoError(t, err)

	// Prepare update form data
	formData := url.Values{}
	formData.Set("Name", "Updated Account Name")
	formData.Set("Balance", "750.50")
	formData.Set("CurrencyCode", "EUR")
	formData.Set("AccountNumber", "UPD001")
	formData.Set("Description", "Updated description")

	// Create request
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String()), strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Logf("Response body: %s", rr.Body.String())
		t.Logf("Response headers: %v", rr.Header())
	}

	// Should redirect after successful update
	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Equal(t, MoneyAccountBasePath, rr.Header().Get("Location"))

	// Verify account was updated
	updatedAccount, err := fixtures.moneyAccountService.GetByID(fixtures.ctx, createdAccount.ID())
	require.NoError(t, err)

	assert.Equal(t, "Updated Account Name", updatedAccount.Name())
	assert.Equal(t, "UPD001", updatedAccount.AccountNumber())
	assert.Equal(t, "Updated description", updatedAccount.Description())
	assert.Equal(t, "EUR", updatedAccount.Balance().Currency().Code)
	assert.Equal(t, int64(75050), updatedAccount.Balance().Amount())
}

func TestMoneyAccountController_Update_ValidationError(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Create test account
	balance := money.NewFromFloat(500.00, "USD")
	account := moneyAccountEntity.New(
		"Test Account",
		balance,
		moneyAccountEntity.WithTenantID(fixtures.tenant.ID),
	)

	createdAccount, err := fixtures.moneyAccountService.Create(fixtures.ctx, account)
	require.NoError(t, err)

	// Prepare invalid update form data
	formData := url.Values{}
	formData.Set("Name", "")                // Empty name should fail validation
	formData.Set("Balance", "-100")         // Negative balance
	formData.Set("CurrencyCode", "INVALID") // Invalid currency code
	formData.Set("AccountNumber", "")
	formData.Set("Description", "")

	// Create request
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String()), strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	// Should return form with errors
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse HTML and check for error messages
	doc, err := htmlquery.Parse(strings.NewReader(rr.Body.String()))
	require.NoError(t, err)

	// Should contain validation error messages
	errorElements := htmlquery.Find(doc, "//small[@data-testid='field-error']")
	assert.Greater(t, len(errorElements), 0, "Should have validation error indicators")

	// Verify account was not updated
	unchangedAccount, err := fixtures.moneyAccountService.GetByID(fixtures.ctx, createdAccount.ID())
	require.NoError(t, err)
	assert.Equal(t, "Test Account", unchangedAccount.Name()) // Should remain unchanged
}

func TestMoneyAccountController_Delete_Success(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Create test account
	balance := money.NewFromFloat(100.00, "USD")
	account := moneyAccountEntity.New(
		"Account to Delete",
		balance,
		moneyAccountEntity.WithTenantID(fixtures.tenant.ID),
	)

	createdAccount, err := fixtures.moneyAccountService.Create(fixtures.ctx, account)
	require.NoError(t, err)

	// Verify account exists
	existingAccount, err := fixtures.moneyAccountService.GetByID(fixtures.ctx, createdAccount.ID())
	require.NoError(t, err)
	assert.Equal(t, "Account to Delete", existingAccount.Name())

	// Create delete request
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String()), nil)
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Logf("Response body: %s", rr.Body.String())
		t.Logf("Response headers: %v", rr.Header())
	}

	// Should redirect after successful deletion
	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Equal(t, MoneyAccountBasePath, rr.Header().Get("Location"))

	// Verify account was deleted
	_, err = fixtures.moneyAccountService.GetByID(fixtures.ctx, createdAccount.ID())
	assert.Error(t, err, "Account should be deleted and not found")
}

func TestMoneyAccountController_Delete_NotFound(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Create request with non-existent UUID
	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", MoneyAccountBasePath, nonExistentID.String()), nil)
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	// Should return error
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestMoneyAccountController_InvalidUUID(t *testing.T) {
	// Setup test environment
	fixtures := setupMoneyAccountTest(t)

	// Test with invalid UUID format
	req := httptest.NewRequest(http.MethodGet, MoneyAccountBasePath+"/invalid-uuid", nil)
	rr := httptest.NewRecorder()

	// Execute request
	fixtures.router.ServeHTTP(rr, req)

	// Should return not found or error
	assert.NotEqual(t, http.StatusOK, rr.Code)
}
