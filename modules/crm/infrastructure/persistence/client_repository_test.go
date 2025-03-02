package persistence_test

import (
	"errors"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
)

func createTestPassport() passport.Passport {
	return passport.New(
		"AB",      // series
		"1234567", // number
		passport.WithFullName("John", "Doe", "Smith"),
		passport.WithGender("Male"),
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
	client, err := client.NewComplete(
		0, // ID will be assigned by database
		"John",
		"Doe",
		"Smith",
		p,
		"123 Main St",
		"john.doe@example.com",
		50.0, // hourly rate
		&birthDate,
		"Male",
		nil,          // passport will be set below if needed
		"1234567890", // PIN
		time.Now(),
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if withPassport {
		client = client.SetPassport(createTestPassport())
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
	uniquePhoneClient, err := client.NewComplete(
		0,
		"Jane",
		"Smith",
		"Doe",
		p,
		"456 Oak St",
		"jane.smith@example.com",
		60.0,
		&birthDate,
		"Female",
		nil,
		"9876543210",
		time.Now(),
		time.Now(),
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

func TestClientRepository_Update(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	repo := persistence.NewClientRepository(
		corepersistence.NewPassportRepository(),
	)

	testClient := createTestClient(t, false)
	created, err := repo.Create(f.ctx, testClient)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	t.Run("Update client basic info", func(t *testing.T) {
		updated := created.SetName("Robert", "Johnson", "Lee").
			SetEmail("robert.johnson@example.com").
			SetAddress("789 Pine St")

		updatedClient, err := repo.Update(f.ctx, updated)
		if err != nil {
			t.Fatalf("Failed to update client: %v", err)
		}

		if updatedClient.FirstName() != "Robert" {
			t.Errorf("Expected FirstName 'Robert', got '%s'", updatedClient.FirstName())
		}

		if updatedClient.LastName() != "Johnson" {
			t.Errorf("Expected LastName 'Johnson', got '%s'", updatedClient.LastName())
		}

		if updatedClient.Email() != "robert.johnson@example.com" {
			t.Errorf("Expected Email 'robert.johnson@example.com', got '%s'", updatedClient.Email())
		}

		if updatedClient.Address() != "789 Pine St" {
			t.Errorf("Expected Address '789 Pine St', got '%s'", updatedClient.Address())
		}
	})

	t.Run("Add passport to existing client", func(t *testing.T) {
		updated := created.SetPassport(createTestPassport())

		updatedClient, err := repo.Update(f.ctx, updated)
		if err != nil {
			t.Fatalf("Failed to update client with passport: %v", err)
		}

		if updatedClient.Passport() == nil {
			t.Error("Updated client should have a passport")
		}

		if updatedClient.Passport().Series() != "AB" {
			t.Errorf("Expected passport series 'AB', got '%s'", updatedClient.Passport().Series())
		}

		if updatedClient.Passport().Number() != "1234567" {
			t.Errorf("Expected passport number '1234567', got '%s'", updatedClient.Passport().Number())
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

		testClient, err := client.NewComplete(
			0,
			"Client",
			string([]byte{'A' + byte(i)}), // Client A, Client B, ...
			"Test",
			p,
			"Address Test",
			"client"+string([]byte{'a' + byte(i)})+"@example.com",
			50.0+float64(i)*10.0,
			nil,
			"Male",
			nil,
			string([]byte{'1', '1', '1', '1', '1', '1', '1', '1', '1', '1' + byte(i)}),
			time.Now(),
			time.Now(),
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

		testClient, err := client.NewComplete(
			0,
			"Count",
			"Test",
			string([]byte{'X' + byte(i)}),
			p,
			"Count Test Address",
			"count"+string([]byte{'a' + byte(i)})+"@example.com",
			100.0,
			nil,
			"Female",
			nil,
			string([]byte{'2', '2', '2', '2', '2', '2', '2', '2', '2', '2' + byte(i)}),
			time.Now(),
			time.Now(),
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

		testClient, err := client.NewComplete(
			0,
			"All",
			"Test",
			string([]byte{'Y' + byte(i)}),
			p,
			"GetAll Test Address",
			"all"+string([]byte{'a' + byte(i)})+"@example.com",
			75.0,
			nil,
			"Male",
			nil,
			string([]byte{'3', '3', '3', '3', '3', '3', '3', '3', '3', '3' + byte(i)}),
			time.Now(),
			time.Now(),
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
