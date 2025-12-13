package position

import (
	"net/url"
	"testing"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/geopoint"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockUpload is a simple mock of upload.Upload for testing
type mockUpload struct {
	id uint
}

func (m *mockUpload) ID() uint {
	return m.id
}

func (m *mockUpload) TenantID() uuid.UUID {
	return uuid.Nil
}

func (m *mockUpload) Type() upload.UploadType {
	return upload.UploadTypeImage
}

func (m *mockUpload) Hash() string {
	return "hash"
}

func (m *mockUpload) Slug() string {
	return "slug"
}

func (m *mockUpload) Path() string {
	return "/path"
}

func (m *mockUpload) Name() string {
	return "name"
}

func (m *mockUpload) Size() upload.Size {
	return nil
}

func (m *mockUpload) IsImage() bool {
	return true
}

func (m *mockUpload) PreviewURL() string {
	return ""
}

func (m *mockUpload) URL() *url.URL {
	return nil
}

func (m *mockUpload) Mimetype() *mimetype.MIME {
	return nil
}

func (m *mockUpload) GeoPoint() geopoint.GeoPoint {
	return nil
}

func (m *mockUpload) CreatedAt() time.Time {
	return time.Now()
}

func (m *mockUpload) UpdatedAt() time.Time {
	return time.Now()
}

func (m *mockUpload) SetHash(hash string) {}

func (m *mockUpload) SetSlug(slug string) {}

func (m *mockUpload) SetName(name string) {}

func (m *mockUpload) SetSize(size upload.Size) {}

func (m *mockUpload) SetGeoPoint(point geopoint.GeoPoint) {}

func (m *mockUpload) SetID(id uint) {
	m.id = id
}

func TestPosition_New_Success(t *testing.T) {
	t.Parallel()

	title := "Test Product"
	barcode := "BARCODE123"

	p := New(title, barcode)

	assert.Equal(t, title, p.Title())
	assert.Equal(t, barcode, p.Barcode())
	assert.Equal(t, uint(0), p.ID())
	assert.Equal(t, uuid.Nil, p.TenantID())
	assert.Equal(t, uint(0), p.UnitID())
	assert.Nil(t, p.Unit())
	assert.Equal(t, uint(0), p.InStock())
	assert.Equal(t, StatusAvailable, p.Status())
	assert.Empty(t, p.Images())
	assert.NotNil(t, p.Events())
	assert.Empty(t, p.Events())
}

func TestPosition_New_WithOptions(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	testUnit := &unit.Unit{ID: 5}
	inStock := uint(100)
	title := "Test Product"
	barcode := "BARCODE123"

	p := New(
		title,
		barcode,
		WithID(42),
		WithTenantID(tenantID),
		WithUnit(testUnit),
		WithInStock(inStock),
		WithStatus(StatusReserved),
	)

	assert.Equal(t, uint(42), p.ID())
	assert.Equal(t, tenantID, p.TenantID())
	assert.Equal(t, title, p.Title())
	assert.Equal(t, barcode, p.Barcode())
	assert.Equal(t, uint(5), p.UnitID())
	assert.NotNil(t, p.Unit())
	assert.Equal(t, inStock, p.InStock())
	assert.Equal(t, StatusReserved, p.Status())
}

func TestPosition_SetTitle_Immutability(t *testing.T) {
	t.Parallel()

	originalTitle := "Original Title"
	newTitle := "New Title"

	p := New(originalTitle, "BAR001", WithID(1))
	newPosition := p.SetTitle(newTitle)

	// Original should be unchanged
	assert.Equal(t, originalTitle, p.Title())
	// New should have new title
	assert.Equal(t, newTitle, newPosition.Title())
	// Should be different instances
	assert.NotEqual(t, p, newPosition)
}

func TestPosition_SetBarcode_Immutability(t *testing.T) {
	t.Parallel()

	originalBarcode := "BAR001"
	newBarcode := "BAR002"

	p := New("Test", originalBarcode, WithID(1))
	newPosition := p.SetBarcode(newBarcode)

	// Original should be unchanged
	assert.Equal(t, originalBarcode, p.Barcode())
	// New should have new barcode
	assert.Equal(t, newBarcode, newPosition.Barcode())
	// Should be different instances
	assert.NotEqual(t, p, newPosition)
}

func TestPosition_SetInStock_Immutability(t *testing.T) {
	t.Parallel()

	originalInStock := uint(50)
	newInStock := uint(100)

	p := New("Test", "BAR001", WithID(1), WithInStock(originalInStock))
	newPosition := p.SetInStock(newInStock)

	// Original should be unchanged
	assert.Equal(t, originalInStock, p.InStock())
	// New should have new stock
	assert.Equal(t, newInStock, newPosition.InStock())
	// Should be different instances
	assert.NotEqual(t, p, newPosition)
}

func TestPosition_SetStatus_Immutability(t *testing.T) {
	t.Parallel()

	p := New("Test", "BAR001", WithID(1), WithStatus(StatusAvailable))
	newPosition := p.SetStatus(StatusReserved)

	// Original should be unchanged
	assert.Equal(t, StatusAvailable, p.Status())
	// New should have new status
	assert.Equal(t, StatusReserved, newPosition.Status())
	// Should be different instances
	assert.NotEqual(t, p, newPosition)
}

func TestPosition_SetStatus_AllStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status Status
	}{
		{"Available", StatusAvailable},
		{"Reserved", StatusReserved},
		{"OutOfStock", StatusOutOfStock},
		{"Backordered", StatusBackordered},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New("Test", "BAR001", WithID(1), WithStatus(StatusAvailable))
			newPosition := p.SetStatus(tt.status)

			assert.Equal(t, tt.status, newPosition.Status())
			// Original unchanged
			assert.Equal(t, StatusAvailable, p.Status())
		})
	}
}

func TestPosition_SetUnit_Immutability(t *testing.T) {
	t.Parallel()

	unit1 := &unit.Unit{ID: 1}
	unit2 := &unit.Unit{ID: 2}

	p := New("Test", "BAR001", WithID(1), WithUnit(unit1))
	assert.Equal(t, uint(1), p.UnitID())

	newPosition := p.SetUnit(unit2)

	// Original should be unchanged
	assert.Equal(t, uint(1), p.UnitID())
	// New should have new unit
	assert.Equal(t, uint(2), newPosition.UnitID())
	assert.NotNil(t, newPosition.Unit())
	assert.Equal(t, uint(2), newPosition.Unit().ID)
	// Should be different instances
	assert.NotEqual(t, p, newPosition)
}

func TestPosition_SetUnit_Nil(t *testing.T) {
	t.Parallel()

	unit1 := &unit.Unit{ID: 1}

	p := New("Test", "BAR001", WithID(1), WithUnit(unit1))
	assert.Equal(t, uint(1), p.UnitID())

	newPosition := p.SetUnit(nil)

	// Unit ID should remain the same
	assert.Equal(t, uint(1), newPosition.UnitID())
	// Unit should be nil
	assert.Nil(t, newPosition.Unit())
}

func TestPosition_SetImages_Immutability(t *testing.T) {
	t.Parallel()

	p := New("Test", "BAR001", WithID(1))
	assert.Empty(t, p.Images())

	images := []upload.Upload{
		&mockUpload{id: 1},
		&mockUpload{id: 2},
	}

	newPosition := p.SetImages(images)

	// Original should have no images
	assert.Empty(t, p.Images())
	// New should have images
	assert.Len(t, newPosition.Images(), 2)
	assert.Equal(t, uint(1), newPosition.Images()[0].ID())
	assert.Equal(t, uint(2), newPosition.Images()[1].ID())
	// Should be different instances
	assert.NotEqual(t, p, newPosition)
}

func TestPosition_SetImages_ReplaceExisting(t *testing.T) {
	t.Parallel()

	images1 := []upload.Upload{
		&mockUpload{id: 1},
	}

	images2 := []upload.Upload{
		&mockUpload{id: 2},
		&mockUpload{id: 3},
	}

	p := New("Test", "BAR001", WithID(1), WithImages(images1))
	assert.Len(t, p.Images(), 1)

	newPosition := p.SetImages(images2)

	// Original should still have old images
	assert.Len(t, p.Images(), 1)
	assert.Equal(t, uint(1), p.Images()[0].ID())

	// New should have new images
	assert.Len(t, newPosition.Images(), 2)
	assert.Equal(t, uint(2), newPosition.Images()[0].ID())
	assert.Equal(t, uint(3), newPosition.Images()[1].ID())
}

func TestPosition_UpdatedAt_ChangesOnSetter(t *testing.T) {
	t.Parallel()

	p := New("Test", "BAR001", WithID(1))
	originalUpdatedAt := p.UpdatedAt()

	time.Sleep(10 * time.Millisecond) // Ensure time difference

	newPosition := p.SetStatus(StatusReserved)

	// Original should have original updatedAt
	assert.Equal(t, originalUpdatedAt, p.UpdatedAt())
	// New should have newer updatedAt
	assert.True(t, newPosition.UpdatedAt().After(originalUpdatedAt), "updatedAt should be newer")
}

func TestPosition_UpdatedAt_ChangesIndependently(t *testing.T) {
	t.Parallel()

	p := New("Test", "BAR001", WithID(1))
	updatedAt1 := p.UpdatedAt()

	time.Sleep(10 * time.Millisecond)

	p2 := p.SetTitle("New Title")
	updatedAt2 := p2.UpdatedAt()

	time.Sleep(10 * time.Millisecond)

	p3 := p2.SetStatus(StatusOutOfStock)
	updatedAt3 := p3.UpdatedAt()

	// Each should have progressively newer timestamps
	assert.True(t, updatedAt2.After(updatedAt1))
	assert.True(t, updatedAt3.After(updatedAt2))
}

func TestPosition_CreatedAt_RemainsConstant(t *testing.T) {
	t.Parallel()

	p := New("Test", "BAR001", WithID(1))
	originalCreatedAt := p.CreatedAt()

	time.Sleep(10 * time.Millisecond)

	p2 := p.SetStatus(StatusReserved)

	// Both should have same createdAt
	assert.Equal(t, originalCreatedAt, p2.CreatedAt())
}

func TestPosition_StatusValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  Status
		isValid bool
	}{
		{"Available valid", StatusAvailable, true},
		{"Reserved valid", StatusReserved, true},
		{"OutOfStock valid", StatusOutOfStock, true},
		{"Backordered valid", StatusBackordered, true},
		{"Invalid status", Status("invalid"), false},
		{"Empty status", Status(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isValid, tt.status.IsValid())
		})
	}
}

func TestPosition_StatusString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status Status
		str    string
	}{
		{StatusAvailable, "AVAILABLE"},
		{StatusReserved, "RESERVED"},
		{StatusOutOfStock, "OUT_OF_STOCK"},
		{StatusBackordered, "BACKORDERED"},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			assert.Equal(t, tt.str, tt.status.String())
		})
	}
}

func TestParseStatus_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected Status
	}{
		{"AVAILABLE", StatusAvailable},
		{"RESERVED", StatusReserved},
		{"OUT_OF_STOCK", StatusOutOfStock},
		{"BACKORDERED", StatusBackordered},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			status, err := ParseStatus(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, status)
		})
	}
}

func TestParseStatus_InvalidValue(t *testing.T) {
	t.Parallel()

	status, err := ParseStatus("invalid_status")
	require.Error(t, err)
	assert.Equal(t, Status(""), status)
}

func TestPosition_TenantID(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	p := New("Test", "BAR001", WithTenantID(tenantID), WithID(1))

	assert.Equal(t, tenantID, p.TenantID())
}

func TestPosition_ID(t *testing.T) {
	t.Parallel()

	p := New("Test", "BAR001", WithID(42))

	assert.Equal(t, uint(42), p.ID())
}

func TestPosition_UnitID_WithOption(t *testing.T) {
	t.Parallel()

	p := New("Test", "BAR001", WithUnitID(100))

	assert.Equal(t, uint(100), p.UnitID())
}

func TestPosition_ChainedSetters(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	testUnit := &unit.Unit{ID: 5}

	p := New("Original Title", "BAR001", WithID(1), WithTenantID(tenantID))

	imageList := []upload.Upload{
		&mockUpload{id: 1},
	}

	updated := p.
		SetTitle("New Title").
		SetBarcode("BAR002").
		SetInStock(200).
		SetStatus(StatusReserved).
		SetUnit(testUnit).
		SetImages(imageList)

	// Check that all changes are reflected
	assert.Equal(t, "New Title", updated.Title())
	assert.Equal(t, "BAR002", updated.Barcode())
	assert.Equal(t, uint(200), updated.InStock())
	assert.Equal(t, StatusReserved, updated.Status())
	assert.Equal(t, uint(5), updated.UnitID())
	assert.Len(t, updated.Images(), 1)

	// Original should be unchanged
	assert.Equal(t, "Original Title", p.Title())
	assert.Equal(t, "BAR001", p.Barcode())
	assert.Equal(t, uint(0), p.InStock())
	assert.Equal(t, StatusAvailable, p.Status())
	assert.Nil(t, p.Unit())
	assert.Empty(t, p.Images())
}

func TestPosition_Events_Empty(t *testing.T) {
	t.Parallel()

	p := New("Test", "BAR001", WithID(1))

	assert.NotNil(t, p.Events())
	assert.Empty(t, p.Events())
}

func TestPosition_AllFields_Preserved(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	testUnit := &unit.Unit{ID: 5}
	inStock := uint(100)
	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now()
	imageList := []upload.Upload{
		&mockUpload{id: 1},
	}

	p := New(
		"Test Title",
		"BARCODE123",
		WithID(42),
		WithTenantID(tenantID),
		WithUnit(testUnit),
		WithInStock(inStock),
		WithImages(imageList),
		WithCreatedAt(createdAt),
		WithUpdatedAt(updatedAt),
		WithStatus(StatusReserved),
	)

	assert.Equal(t, uint(42), p.ID())
	assert.Equal(t, tenantID, p.TenantID())
	assert.Equal(t, "Test Title", p.Title())
	assert.Equal(t, "BARCODE123", p.Barcode())
	assert.Equal(t, uint(5), p.UnitID())
	assert.NotNil(t, p.Unit())
	assert.Equal(t, inStock, p.InStock())
	assert.Equal(t, StatusReserved, p.Status())
	assert.Len(t, p.Images(), 1)
	assert.Equal(t, createdAt, p.CreatedAt())
	assert.Equal(t, updatedAt, p.UpdatedAt())
}

func TestPosition_MultipleStatusTransitions(t *testing.T) {
	t.Parallel()

	p1 := New("Test", "BAR001", WithID(1), WithStatus(StatusAvailable))
	p2 := p1.SetStatus(StatusReserved)
	p3 := p2.SetStatus(StatusOutOfStock)
	p4 := p3.SetStatus(StatusBackordered)
	p5 := p4.SetStatus(StatusAvailable)

	// Each should have different status
	assert.Equal(t, StatusAvailable, p1.Status())
	assert.Equal(t, StatusReserved, p2.Status())
	assert.Equal(t, StatusOutOfStock, p3.Status())
	assert.Equal(t, StatusBackordered, p4.Status())
	assert.Equal(t, StatusAvailable, p5.Status())

	// All should be different instances
	assert.NotEqual(t, p1, p2)
	assert.NotEqual(t, p2, p3)
	assert.NotEqual(t, p3, p4)
	assert.NotEqual(t, p4, p5)
}

func TestPosition_InStock_Variations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		inStock uint
	}{
		{"Zero stock", uint(0)},
		{"Low stock", uint(1)},
		{"Medium stock", uint(50)},
		{"High stock", uint(1000)},
		{"Very high stock", uint(1000000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New("Test", "BAR001", WithInStock(tt.inStock))
			assert.Equal(t, tt.inStock, p.InStock())

			// Test setter
			newInStock := tt.inStock + 10
			newPosition := p.SetInStock(newInStock)
			assert.Equal(t, newInStock, newPosition.InStock())
			assert.Equal(t, tt.inStock, p.InStock()) // Original unchanged
		})
	}
}

func TestPosition_EmptyImages(t *testing.T) {
	t.Parallel()

	p := New("Test", "BAR001", WithID(1))
	assert.Empty(t, p.Images())

	// Set to non-empty
	images := []upload.Upload{&mockUpload{id: 1}}
	p2 := p.SetImages(images)
	assert.Len(t, p2.Images(), 1)

	// Set to empty
	p3 := p2.SetImages([]upload.Upload{})
	assert.Empty(t, p3.Images())
}
