package product

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProduct_New_Success(t *testing.T) {
	t.Parallel()

	rfid := "RFID123456"
	status := InStock

	p := New(rfid, status)

	assert.Equal(t, rfid, p.Rfid())
	assert.Equal(t, status, p.Status())
	assert.Equal(t, uint(0), p.ID())
	assert.Equal(t, uuid.Nil, p.TenantID())
	assert.Equal(t, uint(0), p.PositionID())
	assert.Nil(t, p.Position())
	assert.NotNil(t, p.Events())
	assert.Empty(t, p.Events())
}

func TestProduct_New_WithOptions(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	positionID := uint(42)
	rfid := "RFID123456"
	status := Approved

	p := New(
		rfid,
		status,
		WithID(1),
		WithTenantID(tenantID),
		WithPositionID(positionID),
	)

	assert.Equal(t, uint(1), p.ID())
	assert.Equal(t, tenantID, p.TenantID())
	assert.Equal(t, positionID, p.PositionID())
	assert.Equal(t, rfid, p.Rfid())
	assert.Equal(t, status, p.Status())
}

func TestProduct_SetRfid_Immutability(t *testing.T) {
	t.Parallel()

	originalRfid := "RFID001"
	newRfid := "RFID002"

	p := New(originalRfid, InStock, WithID(1))
	newProduct := p.SetRfid(newRfid)

	// Original should be unchanged
	assert.Equal(t, originalRfid, p.Rfid())
	// New should have new RFID
	assert.Equal(t, newRfid, newProduct.Rfid())
	// Should be different instances
	assert.NotEqual(t, p, newProduct)
}

func TestProduct_SetStatus_Immutability(t *testing.T) {
	t.Parallel()

	p := New("RFID001", InStock, WithID(1))
	newProduct := p.SetStatus(Shipped)

	// Original should be unchanged
	assert.Equal(t, InStock, p.Status())
	// New should have new status
	assert.Equal(t, Shipped, newProduct.Status())
	// Should be different instances
	assert.NotEqual(t, p, newProduct)
}

func TestProduct_SetStatus_MultipleTransitions(t *testing.T) {
	t.Parallel()

	p1 := New("RFID001", InStock, WithID(1))
	p2 := p1.SetStatus(Approved)
	p3 := p2.SetStatus(InDevelopment)
	p4 := p3.SetStatus(Shipped)

	// Each should have different status
	assert.Equal(t, InStock, p1.Status())
	assert.Equal(t, Approved, p2.Status())
	assert.Equal(t, InDevelopment, p3.Status())
	assert.Equal(t, Shipped, p4.Status())

	// All should be different instances
	assert.NotEqual(t, p1, p2)
	assert.NotEqual(t, p2, p3)
	assert.NotEqual(t, p3, p4)
}

func TestProduct_SetPosition_Immutability(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Product", "BAR001", position.WithTenantID(tenantID), position.WithID(10))

	p := New("RFID001", InStock, WithID(1))
	newProduct := p.SetPosition(pos)

	// Original should have no position
	assert.Nil(t, p.Position())
	assert.Equal(t, uint(0), p.PositionID())
	// New should have position
	assert.NotNil(t, newProduct.Position())
	assert.Equal(t, uint(10), newProduct.PositionID())
	assert.Equal(t, pos.ID(), newProduct.Position().ID())
	// Should be different instances
	assert.NotEqual(t, p, newProduct)
}

func TestProduct_SetPosition_WithOptionDuringConstruction(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Product", "BAR001", position.WithTenantID(tenantID), position.WithID(10))

	p := New(
		"RFID001",
		InStock,
		WithPosition(pos),
	)

	assert.NotNil(t, p.Position())
	assert.Equal(t, uint(10), p.PositionID())
	assert.Equal(t, pos.ID(), p.Position().ID())
}

func TestProduct_SetPosition_ReplaceExisting(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos1 := position.New("Product 1", "BAR001", position.WithTenantID(tenantID), position.WithID(1))
	pos2 := position.New("Product 2", "BAR002", position.WithTenantID(tenantID), position.WithID(2))

	p := New("RFID001", InStock, WithPosition(pos1))
	assert.Equal(t, uint(1), p.PositionID())

	newProduct := p.SetPosition(pos2)
	assert.Equal(t, uint(2), newProduct.PositionID())
	assert.Equal(t, pos2.ID(), newProduct.Position().ID())

	// Original should still have old position
	assert.Equal(t, uint(1), p.PositionID())
}

func TestProduct_UpdatedAt_ChangesOnSetter(t *testing.T) {
	t.Parallel()

	p := New("RFID001", InStock, WithID(1))
	originalUpdatedAt := p.UpdatedAt()

	time.Sleep(10 * time.Millisecond) // Ensure time difference

	newProduct := p.SetStatus(Shipped)

	// Original should have original updatedAt
	assert.Equal(t, originalUpdatedAt, p.UpdatedAt())
	// New should have newer updatedAt
	assert.True(t, newProduct.UpdatedAt().After(originalUpdatedAt), "updatedAt should be newer")
}

func TestProduct_UpdatedAt_ChangesIndependently(t *testing.T) {
	t.Parallel()

	p := New("RFID001", InStock, WithID(1))
	updatedAt1 := p.UpdatedAt()

	time.Sleep(10 * time.Millisecond)

	p2 := p.SetRfid("RFID002")
	updatedAt2 := p2.UpdatedAt()

	time.Sleep(10 * time.Millisecond)

	p3 := p2.SetStatus(Approved)
	updatedAt3 := p3.UpdatedAt()

	// Each should have progressively newer timestamps
	assert.True(t, updatedAt2.After(updatedAt1))
	assert.True(t, updatedAt3.After(updatedAt2))
}

func TestProduct_CreatedAt_RemainsConstant(t *testing.T) {
	t.Parallel()

	p := New("RFID001", InStock, WithID(1))
	originalCreatedAt := p.CreatedAt()

	time.Sleep(10 * time.Millisecond)

	p2 := p.SetStatus(Shipped)

	// Both should have same createdAt
	assert.Equal(t, originalCreatedAt, p2.CreatedAt())
}

func TestProduct_Status_AllTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status Status
	}{
		{"InStock", InStock},
		{"InDevelopment", InDevelopment},
		{"Approved", Approved},
		{"Shipped", Shipped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New("RFID001", tt.status, WithID(1))
			assert.Equal(t, tt.status, p.Status())
		})
	}
}

func TestProduct_StatusValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  Status
		isValid bool
	}{
		{"InStock valid", InStock, true},
		{"InDevelopment valid", InDevelopment, true},
		{"Approved valid", Approved, true},
		{"Shipped valid", Shipped, true},
		{"Invalid status", Status("invalid"), false},
		{"Empty status", Status(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isValid, tt.status.IsValid())
		})
	}
}

func TestNewStatus_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected Status
	}{
		{"shipped", Shipped},
		{"in_stock", InStock},
		{"in_development", InDevelopment},
		{"approved", Approved},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			status, err := NewStatus(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, status)
		})
	}
}

func TestNewStatus_InvalidValue(t *testing.T) {
	t.Parallel()

	status, err := NewStatus("invalid_status")
	require.Error(t, err)
	assert.Equal(t, Status(""), status)
}

func TestProduct_TenantID(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	p := New("RFID001", InStock, WithTenantID(tenantID), WithID(1))

	assert.Equal(t, tenantID, p.TenantID())
}

func TestProduct_ID(t *testing.T) {
	t.Parallel()

	p := New("RFID001", InStock, WithID(42))

	assert.Equal(t, uint(42), p.ID())
}

func TestProduct_PositionIDDirect(t *testing.T) {
	t.Parallel()

	p := New("RFID001", InStock, WithPositionID(100))

	assert.Equal(t, uint(100), p.PositionID())
}

func TestProduct_SetPositionNil(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Product", "BAR001", position.WithTenantID(tenantID), position.WithID(10))

	p := New("RFID001", InStock, WithPosition(pos))
	assert.Equal(t, uint(10), p.PositionID())

	newProduct := p.SetPosition(nil)
	assert.Nil(t, newProduct.Position())
	assert.Equal(t, uint(10), newProduct.PositionID()) // PositionID remains unchanged when setting nil
}

func TestProduct_ChainedSetters(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Product", "BAR001", position.WithTenantID(tenantID), position.WithID(10))

	p := New("RFID001", InStock, WithID(1), WithTenantID(tenantID))

	updated := p.SetRfid("RFID002").SetStatus(Approved).SetPosition(pos)

	// Check that all changes are reflected
	assert.Equal(t, "RFID002", updated.Rfid())
	assert.Equal(t, Approved, updated.Status())
	assert.NotNil(t, updated.Position())
	assert.Equal(t, uint(10), updated.Position().ID())

	// Original should be unchanged
	assert.Equal(t, "RFID001", p.Rfid())
	assert.Equal(t, InStock, p.Status())
	assert.Nil(t, p.Position())
}

func TestProduct_Events_Empty(t *testing.T) {
	t.Parallel()

	p := New("RFID001", InStock, WithID(1))

	assert.NotNil(t, p.Events())
	assert.Empty(t, p.Events())
}

func TestProduct_AllFields_Preserved(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now()

	p := New(
		"RFID001",
		InStock,
		WithID(42),
		WithTenantID(tenantID),
		WithPositionID(100),
		WithCreatedAt(createdAt),
		WithUpdatedAt(updatedAt),
	)

	assert.Equal(t, uint(42), p.ID())
	assert.Equal(t, tenantID, p.TenantID())
	assert.Equal(t, uint(100), p.PositionID())
	assert.Equal(t, "RFID001", p.Rfid())
	assert.Equal(t, InStock, p.Status())
	assert.Equal(t, createdAt, p.CreatedAt())
	assert.Equal(t, updatedAt, p.UpdatedAt())
}
