package persistence_test

import (
	"errors"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/general"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
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

func createTestClient(t *testing.T, withPassport bool) client.Client {
	t.Helper()
	p, err := phone.NewFromE164("12345678901")
	if err != nil {
		t.Fatal(err)
	}

	birthDate := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	email, err := internet.NewEmail("john.doe@example.com")
	if err != nil {
		t.Fatal(err)
	}
	pin, err := tax.NewPin("12345678901234", country.Uzbekistan)
	if err != nil {
		t.Fatal(err)
	}
	opts := []client.Option{
		client.WithEmail(email),
		client.WithAddress("123 Main St"),
		client.WithDateOfBirth(&birthDate),
		client.WithGender(general.Male),
		client.WithPin(pin),
		client.WithPhone(p),
	}
	if withPassport {
		opts = append(opts, client.WithPassport(createTestPassport()))
	}
	client, err := client.New(
		"John",
		"Doe",
		"Smith",
		opts...,
	)
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func TestClientRepository_Create(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	t.Run("Create client without passport", func(t *testing.T) {
		testClient := createTestClient(t, false)

		created, err := repo.Create(f.ctx, testClient)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		if created.ID() == 0 {
			t.Error("Created client should have a non-zero ID")
		}

		if created.FirstName() != "John" {
			t.Errorf("Expected FirstName to be 'John', got '%s'", created.FirstName())
		}

		if created.LastName() != "Doe" {
			t.Errorf("Expected LastName to be 'Doe', got '%s'", created.LastName())
		}

		if created.Phone().Value() != "12345678901" {
			t.Errorf("Expected Phone to be '12345678901', got '%s'", created.Phone().Value())
		}
	})

	t.Run("Create client with passport", func(t *testing.T) {
		testClient := createTestClient(t, true)

		created, err := repo.Create(f.ctx, testClient)
		if err != nil {
			t.Fatalf("Failed to create client with passport: %v", err)
		}

		if created.ID() == 0 {
			t.Error("Created client should have a non-zero ID")
		}

		if created.Passport() == nil {
			t.Error("Created client should have a passport")
		}

		if created.Passport().Series() != "AB" {
			t.Errorf("Expected passport series to be 'AB', got '%s'", created.Passport().Series())
		}

		if created.Passport().Number() != "1234567" {
			t.Errorf("Expected passport number to be '1234567', got '%s'", created.Passport().Number())
		}
	})
}

func TestClientRepository_GetByID(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	testClient := createTestClient(t, true)
	created, err := repo.Create(f.ctx, testClient)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	t.Run("Get existing client by ID", func(t *testing.T) {
		retrieved, err := repo.GetByID(f.ctx, created.ID())
		if err != nil {
			t.Fatalf("Failed to get client by ID: %v", err)
		}

		if retrieved.ID() != created.ID() {
			t.Errorf("Expected ID %d, got %d", created.ID(), retrieved.ID())
		}

		if retrieved.FirstName() != created.FirstName() {
			t.Errorf("Expected FirstName '%s', got '%s'", created.FirstName(), retrieved.FirstName())
		}

		if retrieved.Passport() == nil {
			t.Error("Retrieved client should have a passport")
		}

		if retrieved.Passport().Series() != created.Passport().Series() {
			t.Errorf("Expected passport series '%s', got '%s'", created.Passport().Series(), retrieved.Passport().Series())
		}
	})

	t.Run("Get non-existent client by ID", func(t *testing.T) {
		_, err := repo.GetByID(f.ctx, 9999)
		if err == nil {
			t.Fatal("Expected error when getting non-existent client, got nil")
		}

		if !errors.Is(err, persistence.ErrClientNotFound) {
			t.Errorf("Expected ErrClientNotFound, got %v", err)
		}
	})
}

func TestClientRepository_GetByPhone(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	p, err := phone.NewFromE164("98765432109")
	if err != nil {
		t.Fatal(err)
	}

	birthDate := time.Date(1992, 2, 2, 0, 0, 0, 0, time.UTC)
	email, err := internet.NewEmail("jane.smith@example.com")
	if err != nil {
		t.Fatal(err)
	}

	pin, err := tax.NewPin("98765432109876", country.Uzbekistan)
	if err != nil {
		t.Fatal(err)
	}

	uniquePhoneClient, err := client.New(
		"Jane",
		"Smith",
		"Doe",
		client.WithPhone(p),
		client.WithID(0),
		client.WithAddress("456 Oak St"),
		client.WithEmail(email),
		client.WithDateOfBirth(&birthDate),
		client.WithGender(general.Female),
		client.WithPin(pin),
	)
	if err != nil {
		t.Fatal(err)
	}

	created, err := repo.Create(f.ctx, uniquePhoneClient)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	t.Run("Get existing client by phone", func(t *testing.T) {
		retrieved, err := repo.GetByPhone(f.ctx, "98765432109")
		if err != nil {
			t.Fatalf("Failed to get client by phone: %v", err)
		}

		if retrieved.ID() != created.ID() {
			t.Errorf("Expected ID %d, got %d", created.ID(), retrieved.ID())
		}

		if retrieved.FirstName() != "Jane" {
			t.Errorf("Expected FirstName 'Jane', got '%s'", retrieved.FirstName())
		}
	})

	t.Run("Get non-existent client by phone", func(t *testing.T) {
		_, err := repo.GetByPhone(f.ctx, "11111111111")
		if err == nil {
			t.Fatal("Expected error when getting non-existent client, got nil")
		}

		if !errors.Is(err, persistence.ErrClientNotFound) {
			t.Errorf("Expected ErrClientNotFound, got %v", err)
		}
	})
}

func TestClientRepository_GetPaginated(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	// Create multiple clients for pagination testing
	for i := 0; i < 5; i++ {
		p, err := phone.NewFromE164(string([]byte{'1', '0', '0', '0', '0', '0', '0', '0', '0', '0' + byte(i), '1'}))
		if err != nil {
			t.Fatal(err)
		}

		email, err := internet.NewEmail("client" + string([]byte{'a' + byte(i)}) + "@example.com")
		if err != nil {
			t.Fatal(err)
		}

		// Create a valid 14-digit PIN for Uzbekistan
		pin, err := tax.NewPin("12345678901234", country.Uzbekistan)
		if err != nil {
			t.Fatal(err)
		}

		testClient, err := client.New(
			"Client",
			string([]byte{'A' + byte(i)}), // Client A, Client B, ...
			"Test",
			client.WithPhone(p),
			client.WithID(0),
			client.WithAddress("Address Test"),
			client.WithEmail(email),
			client.WithGender(general.Male),
			client.WithPin(pin),
		)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Create(f.ctx, testClient)
		if err != nil {
			t.Fatalf("Failed to create test client %d: %v", i, err)
		}
	}

	t.Run("Get clients with limit and offset", func(t *testing.T) {
		params := &client.FindParams{
			Limit:  2,
			Offset: 1,
			SortBy: client.SortBy{
				Fields:    []client.Field{client.LastName},
				Ascending: true,
			},
		}

		clients, err := repo.GetPaginated(f.ctx, params)
		if err != nil {
			t.Fatalf("Failed to get paginated clients: %v", err)
		}

		if len(clients) != 2 {
			t.Errorf("Expected 2 clients, got %d", len(clients))
		}

		// Should return clients B and C if sorted by last name ascending
		if clients[0].LastName() != "B" {
			t.Errorf("Expected first client LastName 'B', got '%s'", clients[0].LastName())
		}

		if clients[1].LastName() != "C" {
			t.Errorf("Expected second client LastName 'C', got '%s'", clients[1].LastName())
		}
	})

	t.Run("Get clients with query filter", func(t *testing.T) {
		params := &client.FindParams{
			Limit:  10,
			Offset: 0,
			Query:  "D",
			Field:  "last_name",
			SortBy: client.SortBy{
				Fields:    []client.Field{client.LastName},
				Ascending: true,
			},
		}

		clients, err := repo.GetPaginated(f.ctx, params)
		if err != nil {
			t.Fatalf("Failed to get filtered clients: %v", err)
		}

		if len(clients) != 1 {
			t.Errorf("Expected 1 client, got %d", len(clients))
		}

		if len(clients) > 0 && clients[0].LastName() != "D" {
			t.Errorf("Expected client LastName 'D', got '%s'", clients[0].LastName())
		}
	})
}

func TestClientRepository_Count(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	// Create a known number of clients
	numClients := 3
	for i := 0; i < numClients; i++ {
		p, err := phone.NewFromE164(string([]byte{'2', '0', '0', '0', '0', '0', '0', '0', '0', '0' + byte(i), '1'}))
		if err != nil {
			t.Fatal(err)
		}

		email, err := internet.NewEmail("count" + string([]byte{'a' + byte(i)}) + "@example.com")
		if err != nil {
			t.Fatal(err)
		}

		// Create a valid 14-digit PIN for Uzbekistan
		pin, err := tax.NewPin("12345678901234", country.Uzbekistan)
		if err != nil {
			t.Fatal(err)
		}

		testClient, err := client.New(
			"Count",
			"Test",
			string([]byte{'X' + byte(i)}),
			client.WithPhone(p),
			client.WithID(0),
			client.WithAddress("Count Test Address"),
			client.WithEmail(email),
			client.WithGender(general.Female),
			client.WithPin(pin),
		)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Create(f.ctx, testClient)
		if err != nil {
			t.Fatalf("Failed to create test client %d: %v", i, err)
		}
	}

	count, err := repo.Count(f.ctx)
	if err != nil {
		t.Fatalf("Failed to get client count: %v", err)
	}

	if int(count) < numClients {
		t.Errorf("Expected at least %d clients, got %d", numClients, count)
	}
}

func TestClientRepository_GetAll(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	// Create clients for GetAll test
	initialCount, err := repo.Count(f.ctx)
	if err != nil {
		t.Fatalf("Failed to get initial client count: %v", err)
	}

	numNewClients := 2
	for i := 0; i < numNewClients; i++ {
		p, err := phone.NewFromE164(string([]byte{'3', '0', '0', '0', '0', '0', '0', '0', '0', '0' + byte(i), '1'}))
		if err != nil {
			t.Fatal(err)
		}

		email, err := internet.NewEmail("all" + string([]byte{'a' + byte(i)}) + "@example.com")
		if err != nil {
			t.Fatal(err)
		}

		// Create a valid 14-digit PIN for Uzbekistan
		pin, err := tax.NewPin("12345678901234", country.Uzbekistan)
		if err != nil {
			t.Fatal(err)
		}

		testClient, err := client.New(
			"All",
			"Test",
			string([]byte{'Y' + byte(i)}),
			client.WithPhone(p),
			client.WithID(0),
			client.WithAddress("GetAll Test Address"),
			client.WithEmail(email),
			client.WithGender(general.Male),
			client.WithPin(pin),
		)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Create(f.ctx, testClient)
		if err != nil {
			t.Fatalf("Failed to create test client %d: %v", i, err)
		}
	}

	allClients, err := repo.GetAll(f.ctx)
	if err != nil {
		t.Fatalf("Failed to get all clients: %v", err)
	}

	expectedCount := int(initialCount) + numNewClients
	if len(allClients) != expectedCount {
		t.Errorf("Expected %d clients, got %d", expectedCount, len(allClients))
	}
}

func TestClientRepository_Delete(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	clientRepo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	// Create a client with passport for deletion testing
	testClient := createTestClient(t, true)
	created, err := clientRepo.Create(f.ctx, testClient)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	// Delete the client
	err = clientRepo.Delete(f.ctx, created.ID())
	if err != nil {
		t.Fatalf("Failed to delete client: %v", err)
	}

	// Verify client is deleted
	_, err = clientRepo.GetByID(f.ctx, created.ID())
	if err == nil {
		t.Error("Expected error when getting deleted client, got nil")
	}

	if !errors.Is(err, persistence.ErrClientNotFound) {
		t.Errorf("Expected ErrClientNotFound, got %v", err)
	}
}

// TestClientRepository_Update tests updating client details
func TestClientRepository_Update(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	// Create a client without passport
	p, err := phone.NewFromE164("12345678901")
	if err != nil {
		t.Fatal(err)
	}

	birthDate := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	email, err := internet.NewEmail("john.doe@example.com")
	if err != nil {
		t.Fatal(err)
	}
	pin, err := tax.NewPin("12345678901234", country.Uzbekistan)
	if err != nil {
		t.Fatal(err)
	}

	basicClient, err := client.New(
		"John",
		"Doe",
		"Smith",
		client.WithPhone(p),
		client.WithEmail(email),
		client.WithAddress("123 Main St"),
		client.WithDateOfBirth(&birthDate),
		client.WithGender(general.Male),
		client.WithPin(pin),
	)
	if err != nil {
		t.Fatal(err)
	}

	created, err := repo.Create(f.ctx, basicClient)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test updating basic info first
	t.Run("Update basic info", func(t *testing.T) {
		newEmail, err := internet.NewEmail("robert.johnson@example.com")
		if err != nil {
			t.Fatal(err)
		}

		updatedClient := created.SetName("Robert", "Johnson", "Lee").
			SetEmail(newEmail).
			SetAddress("789 Pine St")

		result, err := repo.Update(f.ctx, updatedClient)
		if err != nil {
			t.Fatalf("Failed to update client: %v", err)
		}

		if result.FirstName() != "Robert" {
			t.Errorf("Expected FirstName 'Robert', got '%s'", result.FirstName())
		}

		if result.LastName() != "Johnson" {
			t.Errorf("Expected LastName 'Johnson', got '%s'", result.LastName())
		}

		if result.Email().Value() != "robert.johnson@example.com" {
			t.Errorf("Expected Email 'robert.johnson@example.com', got '%s'", result.Email().Value())
		}

		if result.Address() != "789 Pine St" {
			t.Errorf("Expected Address '789 Pine St', got '%s'", result.Address())
		}
	})

	// Create another client specifically for testing passport updates
	t.Run("Update with passport", func(t *testing.T) {
		// Create a new client without passport
		noPassportClient, err := client.New(
			"Alice",
			"Wonder",
			"",
			client.WithPhone(p),
			client.WithEmail(email),
			client.WithPin(pin),
			client.WithGender(general.Female),
		)
		if err != nil {
			t.Fatal(err)
		}

		created, err := repo.Create(f.ctx, noPassportClient)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Now add a passport
		pass := createTestPassport()
		clientWithPassport := created.SetPassport(pass)

		updated, err := repo.Update(f.ctx, clientWithPassport)
		if err != nil {
			t.Fatalf("Failed to update client with passport: %v", err)
		}

		if updated.Passport() == nil {
			t.Errorf("Expected client to have passport after update")
		}

		if updated.Passport().Series() != "AB" {
			t.Errorf("Expected passport series to be 'AB', got '%s'", updated.Passport().Series())
		}

		if updated.Passport().Number() != "1234567" {
			t.Errorf("Expected passport number to be '1234567', got '%s'", updated.Passport().Number())
		}
	})
}

