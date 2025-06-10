package persistence_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/general"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

func createTestPassport() passport.Passport {
	return passport.New(
		"AB",      // series
		"1234567", // number
		passport.WithFullName("John", "Doe", "Smith"),
		passport.WithGender(general.Male),
		passport.WithBirthDetails(time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), "New York"),
		passport.WithNationality("USA"),
		passport.WithPassportType("Regular"),
		passport.WithIssuedAt(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
		passport.WithIssuedBy("Department of State"),
		passport.WithIssuingCountry("USA"),
		passport.WithExpiresAt(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)),
		passport.WithMachineReadableZone("P<USADOE<<JOHN<SMITH<<<<<<<<<<<<<<<<<<<<<<<"),
		passport.WithRemarks("No special remarks"),
	)
}

func createTestClient(t *testing.T, tenantID uuid.UUID, withPassport bool) client.Client {
	t.Helper()
	p, err := phone.NewFromE164("12345678901")
	require.NoError(t, err, "Failed to create phone number")

	birthDate := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	email := internet.MustParseEmail("john.doe@example.com")

	pin, err := tax.NewPin("12345678901234", country.Uzbekistan)
	require.NoError(t, err, "Failed to create tax ID")

	opts := []client.Option{
		client.WithTenantID(tenantID),
		client.WithLastName("Doe"),
		client.WithMiddleName("Smith"),
		client.WithEmail(email),
		client.WithAddress("123 Main St"),
		client.WithDateOfBirth(&birthDate),
		client.WithGender(general.Male),
		client.WithContacts([]client.Contact{
			client.NewContact(client.ContactTypePhone, "12345678901"),
		}),
		client.WithPin(pin),
		client.WithPhone(p),
	}
	if withPassport {
		opts = append(opts, client.WithPassport(createTestPassport()))
	}
	newClient, err := client.New(
		"John",
		opts...,
	)
	require.NoError(t, err, "Failed to create client instance")

	return newClient
}

func TestClientRepository_Create(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	t.Run("Create client without passport", func(t *testing.T) {
		testClient := createTestClient(t, f.tenant.ID, false)

		created, err := clientRepo.Save(f.ctx, testClient)
		require.NoError(t, err, "Failed to create client")

		assert.NotZero(t, created.ID(), "Created client should have a non-zero ID")
		assert.Equal(t, "John", created.FirstName(), "FirstName mismatch")
		assert.Equal(t, "Doe", created.LastName(), "LastName mismatch")
		require.NotNil(t, created.Phone(), "Phone should not be nil")
		assert.Equal(t, "12345678901", created.Phone().Value(), "Phone value mismatch")
		assert.Nil(t, created.Passport(), "Passport should be nil for this test case")
	})

	t.Run("Create client with passport", func(t *testing.T) {
		testClient := createTestClient(t, f.tenant.ID, true)

		created, err := clientRepo.Save(f.ctx, testClient)
		require.NoError(t, err, "Failed to create client with passport")

		assert.NotZero(t, created.ID(), "Created client should have a non-zero ID")
		require.NotNil(t, created.Passport(), "Created client should have a passport")

		assert.Equal(t, "AB", created.Passport().Series(), "Passport series mismatch")
		assert.Equal(t, "1234567", created.Passport().Number(), "Passport number mismatch")
	})
}

func TestClientRepository_GetByID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	testClient := createTestClient(t, f.tenant.ID, true)
	created, err := clientRepo.Save(f.ctx, testClient)
	require.NoError(t, err, "Failed to create test client for GetByID")

	t.Run("Get existing client by ID", func(t *testing.T) {
		clients, err := clientRepo.GetPaginated(f.ctx, &client.FindParams{
			Limit: 1,
			Filters: []client.Filter{
				{
					Column: client.ID,
					Filter: repo.Eq(created.ID()),
				},
			},
		})
		require.NoError(t, err, "Failed to get client by ID")
		require.Len(t, clients, 1, "Should return exactly one client")

		retrieved := clients[0]
		assert.Equal(t, created.ID(), retrieved.ID(), "ID mismatch")
		assert.Equal(t, created.FirstName(), retrieved.FirstName(), "FirstName mismatch")
		require.NotNil(t, retrieved.Passport(), "Retrieved client should have a passport")

		assert.Equal(t, created.Passport().Series(), retrieved.Passport().Series(), "Passport series mismatch")
		assert.Equal(t, created.Passport().Number(), retrieved.Passport().Number(), "Passport number mismatch")
	})

	t.Run("Get non-existent client by ID", func(t *testing.T) {
		_, err := clientRepo.GetByID(f.ctx, 9999, f.tenant.ID)
		require.Error(t, err, "Expected error when getting non-existent client, got nil")
		require.ErrorIs(t, err, persistence.ErrClientNotFound, "Expected ErrClientNotFound")
	})
}

func TestClientRepository_GetByPhone(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	p, err := phone.NewFromE164("98765432109")
	require.NoError(t, err, "Failed to create phone number")

	birthDate := time.Date(1992, 2, 2, 0, 0, 0, 0, time.UTC)
	email, err := internet.NewEmail("jane.smith@example.com")
	require.NoError(t, err, "Failed to create email")

	pin, err := tax.NewPin("98765432109876", country.Uzbekistan)
	require.NoError(t, err, "Failed to create tax ID")

	uniquePhoneClient, err := client.New(
		"Jane",
		client.WithTenantID(f.tenant.ID),
		client.WithLastName("Smith"),
		client.WithMiddleName("Doe"),
		client.WithPhone(p),
		client.WithID(0),
		client.WithAddress("456 Oak St"),
		client.WithEmail(email),
		client.WithDateOfBirth(&birthDate),
		client.WithGender(general.Female),
		client.WithPin(pin),
	)
	require.NoError(t, err, "Failed to create client instance")

	created, err := clientRepo.Save(f.ctx, uniquePhoneClient)
	require.NoError(t, err, "Failed to create test client for GetByPhone")

	t.Run("Get existing client by phone", func(t *testing.T) {
		retrieved, err := clientRepo.GetByPhone(f.ctx, "98765432109")
		require.NoError(t, err, "Failed to get client by phone")

		assert.Equal(t, created.ID(), retrieved.ID(), "ID mismatch")
		assert.Equal(t, "Jane", retrieved.FirstName(), "FirstName mismatch")
		require.NotNil(t, retrieved.Phone(), "Phone should not be nil")
		assert.Equal(t, "98765432109", retrieved.Phone().Value(), "Phone value mismatch")
	})

	t.Run("Get non-existent client by phone", func(t *testing.T) {
		_, err := clientRepo.GetByPhone(f.ctx, "11111111111")
		require.Error(t, err, "Expected error when getting non-existent client, got nil")
		require.ErrorIs(t, err, persistence.ErrClientNotFound, "Expected ErrClientNotFound")
	})
}

func TestClientRepository_GetByContactValue(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	p, err := phone.NewFromE164("55555555555")
	require.NoError(t, err, "Failed to create phone number")

	email, err := internet.NewEmail("contact.test@example.com")
	require.NoError(t, err, "Failed to create email")

	clientWithContacts, err := client.New(
		"Contact",
		client.WithTenantID(f.tenant.ID),
		client.WithLastName("Test"),
		client.WithMiddleName("User"),
		client.WithPhone(p),
		client.WithEmail(email),
		client.WithGender(general.Male),
		client.WithContacts([]client.Contact{
			client.NewContact(client.ContactTypePhone, "55555555555"),
			client.NewContact(client.ContactTypeTelegram, "telegram_user123"),
			client.NewContact(client.ContactTypeWhatsApp, "+1234567890"),
			client.NewContact(client.ContactTypeEmail, "test2@example.com"),
		}),
	)
	require.NoError(t, err, "Failed to create client instance")

	createdClient, err := clientRepo.Save(f.ctx, clientWithContacts)
	require.NoError(t, err, "Failed to create test client")

	t.Run("Get client by phone from clients table", func(t *testing.T) {
		retrieved, err := clientRepo.GetByContactValue(f.ctx, client.ContactTypePhone, "55555555555")
		require.NoError(t, err, "Failed to get client by phone contact")

		assert.Equal(t, createdClient.ID(), retrieved.ID(), "ID mismatch")
		assert.Equal(t, "Contact", retrieved.FirstName(), "FirstName mismatch")
	})

	t.Run("Get client by email from clients table", func(t *testing.T) {
		retrieved, err := clientRepo.GetByContactValue(f.ctx, client.ContactTypeEmail, "contact.test@example.com")
		require.NoError(t, err, "Failed to get client by email contact")

		assert.Equal(t, createdClient.ID(), retrieved.ID(), "ID mismatch")
		assert.Equal(t, "Test", retrieved.LastName(), "LastName mismatch")
	})

	t.Run("Get client by email from client_contacts table", func(t *testing.T) {
		retrieved, err := clientRepo.GetByContactValue(f.ctx, client.ContactTypeEmail, "test2@example.com")
		require.NoError(t, err, "Failed to get client by email contact from client_contacts table")

		assert.Equal(t, createdClient.ID(), retrieved.ID(), "ID mismatch")
	})

	t.Run("Get client by telegram contact from client_contacts table", func(t *testing.T) {
		retrieved, err := clientRepo.GetByContactValue(f.ctx, client.ContactTypeTelegram, "telegram_user123")
		require.NoError(t, err, "Failed to get client by telegram contact")

		assert.Equal(t, createdClient.ID(), retrieved.ID(), "ID mismatch")
		assert.Equal(t, "User", retrieved.MiddleName(), "MiddleName mismatch")
	})

	t.Run("Get client by whatsapp contact from client_contacts table", func(t *testing.T) {
		retrieved, err := clientRepo.GetByContactValue(f.ctx, client.ContactTypeWhatsApp, "+1234567890")
		require.NoError(t, err, "Failed to get client by whatsapp contact")

		assert.Equal(t, createdClient.ID(), retrieved.ID(), "ID mismatch")
	})

	t.Run("Get client by non-existent contact", func(t *testing.T) {
		_, err := clientRepo.GetByContactValue(f.ctx, client.ContactTypeTelegram, "non_existent_user")
		require.Error(t, err, "Expected error when getting client by non-existent contact")
		require.ErrorIs(t, err, persistence.ErrClientNotFound, "Expected ErrClientNotFound")
	})
}

func TestClientRepository_GetPaginated(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	// Setup: Create multiple clients
	clientLastNames := []string{"A", "B", "C", "D", "E"}
	for i, lastName := range clientLastNames {
		p, err := phone.NewFromE164(string([]byte{'1', '0', '0', '0', '0', '0', '0', '0', '0', '0' + byte(i), '1'}))
		require.NoError(t, err, "Failed to create phone number for client %d", i) // Use require for setup

		email, err := internet.NewEmail("client" + string([]byte{'a' + byte(i)}) + "@example.com")
		require.NoError(t, err, "Failed to create email for client %d", i) // Use require for setup

		pin, err := tax.NewPin("12345678901234", country.Uzbekistan)        // Assuming same PIN is okay for test data
		require.NoError(t, err, "Failed to create tax ID for client %d", i) // Use require for setup

		testClient, err := client.New(
			"Client",
			client.WithTenantID(f.tenant.ID),
			client.WithLastName(lastName),
			client.WithMiddleName("Test"),
			client.WithPhone(p),
			client.WithAddress("Address Test"),
			client.WithEmail(email),
			client.WithGender(general.Male),
			client.WithPin(pin),
		)
		require.NoError(t, err, "Failed to create client instance %d", i) // Use require for setup

		_, err = clientRepo.Save(f.ctx, testClient)
		require.NoError(t, err, "Failed to create test client %d", i) // Use require for setup
	}

	t.Run("Get clients with limit and offset", func(t *testing.T) {
		params := &client.FindParams{
			Limit:  2,
			Offset: 1,
			SortBy: client.SortBy{
				Fields: []client.SortByField{{
					Field:     client.LastName,
					Ascending: true,
				}},
			},
		}

		clients, err := clientRepo.GetPaginated(f.ctx, params)
		require.NoError(t, err, "Failed to get paginated clients") // Use require

		require.Len(t, clients, 2, "Expected 2 clients") // Use require for length check

		// Assuming sorting by LastName works correctly (A, B, C, D, E)
		// Offset 1, Limit 2 should return B and C
		assert.Equal(t, "B", clients[0].LastName(), "First client LastName mismatch")
		assert.Equal(t, "C", clients[1].LastName(), "Second client LastName mismatch")
	})

	t.Run("Get clients with search filter", func(t *testing.T) {
		params := &client.FindParams{
			Limit:  10,
			Offset: 0,
			Search: "D", // Search across multiple fields containing "D"
			SortBy: client.SortBy{
				Fields: []client.SortByField{{
					Field:     client.LastName,
					Ascending: true,
				}},
			},
		}

		clients, err := clientRepo.GetPaginated(f.ctx, params)
		require.NoError(t, err, "Failed to get clients with search filter") // Use require

		require.Len(t, clients, 1, "Expected 1 client") // Use require for length check

		assert.Equal(t, "D", clients[0].LastName(), "Filtered client LastName mismatch")
	})
}

func TestClientRepository_Count(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	// Get initial count to account for clients created in parallel tests
	initialCount, err := clientRepo.Count(f.ctx, &client.FindParams{})
	require.NoError(t, err, "Failed to get initial client count")

	numClients := 3
	for i := 0; i < numClients; i++ {
		p, err := phone.NewFromE164(string([]byte{'2', '0', '0', '0', '0', '0', '0', '0', '0', '0' + byte(i), '1'}))
		require.NoError(t, err, "Failed to create phone number for count test %d", i)

		testClient, err := client.New(
			"Count",
			client.WithTenantID(f.tenant.ID),
			client.WithLastName("Test"),
			client.WithPhone(p),
			client.WithMiddleName(string([]byte{'X' + byte(i)})),
			client.WithGender(general.Female),
		)
		require.NoError(t, err, "Failed to create client instance for count test %d", i)

		_, err = clientRepo.Save(f.ctx, testClient)
		require.NoError(t, err, "Failed to create test client for count test %d", i)
	}

	count, err := clientRepo.Count(f.ctx, &client.FindParams{})
	require.NoError(t, err, "Failed to count clients") // Use require

	assert.Equal(t, initialCount+int64(numClients), count, "Client count mismatch")
}

func TestClientRepository_GetAll(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	initialCount, err := clientRepo.Count(f.ctx, &client.FindParams{})
	require.NoError(t, err, "Failed to get initial client count for GetAll test")

	numNewClients := 2
	for i := 0; i < numNewClients; i++ {
		p, err := phone.NewFromE164(string([]byte{'3', '0', '0', '0', '0', '0', '0', '0', '0', '0' + byte(i), '1'}))
		require.NoError(t, err, "Failed to create phone number for GetAll test %d", i)

		email, err := internet.NewEmail("all" + string([]byte{'a' + byte(i)}) + "@example.com")
		require.NoError(t, err, "Failed to create email for GetAll test %d", i)

		testClient, err := client.New(
			"All",
			client.WithTenantID(f.tenant.ID),
			client.WithLastName("Test"),
			client.WithMiddleName(string([]byte{'Y' + byte(i)})),
			client.WithPhone(p),
			client.WithAddress("GetAll Test Address"),
			client.WithEmail(email),
			client.WithGender(general.Male),
		)
		require.NoError(t, err, "Failed to create client instance for GetAll test %d", i)

		_, err = clientRepo.Save(f.ctx, testClient)
		require.NoError(t, err, "Failed to create test client for GetAll test %d", i)
	}

	allClients, err := clientRepo.GetPaginated(f.ctx, &client.FindParams{
		Limit: 1000, // Large limit to get all clients
	})
	require.NoError(t, err, "Failed to get all clients")

	assert.Len(t, allClients, int(initialCount)+numNewClients, "GetPaginated returned incorrect number of clients")
}

func TestClientRepository_Delete(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	testClient := createTestClient(t, f.tenant.ID, true)
	created, err := clientRepo.Save(f.ctx, testClient)
	require.NoError(t, err, "Failed to create test client for delete test")

	_, err = clientRepo.GetByID(f.ctx, created.ID(), f.tenant.ID)
	require.NoError(t, err, "Client should exist before deletion")

	err = clientRepo.Delete(f.ctx, created.ID(), f.tenant.ID)
	require.NoError(t, err, "Failed to delete client")

	// Verify deletion
	_, err = clientRepo.GetByID(f.ctx, created.ID(), f.tenant.ID)
	require.Error(t, err, "Expected error when getting deleted client")
	require.ErrorIs(t, err, persistence.ErrClientNotFound, "Expected ErrClientNotFound after delete")
}

func TestClientRepository_Update(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	// --- Setup initial client ---
	p, err := phone.NewFromE164("12345678901")
	require.NoError(t, err, "Failed to create phone number")

	birthDate := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	email, err := internet.NewEmail("john.doe@example.com")
	require.NoError(t, err, "Failed to create email")

	pin, err := tax.NewPin("12345678901234", country.Uzbekistan)
	require.NoError(t, err, "Failed to create tax ID")

	basicClient, err := client.New(
		"John",
		client.WithTenantID(f.tenant.ID),
		client.WithLastName("Doe"),
		client.WithMiddleName("Smith"),
		client.WithPhone(p),
		client.WithEmail(email),
		client.WithAddress("123 Main St"),
		client.WithDateOfBirth(&birthDate),
		client.WithGender(general.Male),
		client.WithPin(pin),
	)
	require.NoError(t, err, "Failed to create client instance")

	created, err := clientRepo.Save(f.ctx, basicClient)
	require.NoError(t, err, "Failed to save initial client for update test")
	// --- End Setup ---

	t.Run("Update basic info", func(t *testing.T) {
		newEmail, err := internet.NewEmail("robert.johnson@example.com")
		require.NoError(t, err, "Failed to create new email for update")

		updatedClientState := created.SetName("Robert", "Johnson", "Lee").
			SetEmail(newEmail).
			SetAddress("789 Pine St")

		_, err = clientRepo.Save(f.ctx, updatedClientState)
		require.NoError(t, err, "Failed to update client basic info")

		retrievedAfterUpdate, err := clientRepo.GetByID(f.ctx, created.ID(), f.tenant.ID) // Re-fetch to ensure persistence
		require.NoError(t, err, "Failed to retrieve client after basic update")

		assert.Equal(t, "Robert", retrievedAfterUpdate.FirstName(), "FirstName mismatch after update")
		assert.Equal(t, "Johnson", retrievedAfterUpdate.LastName(), "LastName mismatch after update")
		assert.Equal(t, "Lee", retrievedAfterUpdate.MiddleName(), "MiddleName mismatch after update")
		require.NotNil(t, retrievedAfterUpdate.Email(), "Email should not be nil after update")
		assert.Equal(t, "robert.johnson@example.com", retrievedAfterUpdate.Email().Value(), "Email mismatch after update")
		assert.Equal(t, "789 Pine St", retrievedAfterUpdate.Address(), "Address mismatch after update")
		// Also check that unchanged fields remain the same
		require.NotNil(t, retrievedAfterUpdate.Phone(), "Phone should not be nil")
		assert.Equal(t, "12345678901", retrievedAfterUpdate.Phone().Value(), "Phone should not change")
	})

	t.Run("Update by adding passport", func(t *testing.T) {
		// --- Setup client without passport ---
		pUpdate, err := phone.NewFromE164("11223344556")
		require.NoError(t, err)
		emailUpdate, err := internet.NewEmail("alice.wonder@example.com")
		require.NoError(t, err)
		pinUpdate, err := tax.NewPin("98765432101234", country.Uzbekistan)
		require.NoError(t, err)

		noPassportClient, err := client.New(
			"Alice",
			client.WithTenantID(f.tenant.ID),
			client.WithLastName("Wonder"),
			client.WithPhone(pUpdate),
			client.WithEmail(emailUpdate),
			client.WithPin(pinUpdate),
			client.WithGender(general.Female),
		)
		require.NoError(t, err, "Failed to create client instance without passport")

		createdNoPassport, err := clientRepo.Save(f.ctx, noPassportClient)
		require.NoError(t, err, "Failed to create client without passport for update test")
		require.Nil(t, createdNoPassport.Passport(), "Client should initially have no passport")
		// --- End Setup ---

		// Add passport
		clientWithPassport := createdNoPassport.SetPassport(createTestPassport())

		// Save the updated client
		_, err = clientRepo.Save(f.ctx, clientWithPassport)
		require.NoError(t, err, "Failed to update client with passport")

		// Verify the result from the database
		retrievedAfterPassport, err := clientRepo.GetByID(f.ctx, createdNoPassport.ID(), f.tenant.ID)
		require.NoError(t, err, "Failed to retrieve client after adding passport")

		require.NotNil(t, retrievedAfterPassport.Passport(), "Expected client to have passport after update")
		assert.Equal(t, "AB", retrievedAfterPassport.Passport().Series(), "Passport series mismatch after update")
		assert.Equal(t, "1234567", retrievedAfterPassport.Passport().Number(), "Passport number mismatch after update")
		assert.Equal(t, "Alice", retrievedAfterPassport.FirstName(), "FirstName should remain after adding passport")
	})
}
