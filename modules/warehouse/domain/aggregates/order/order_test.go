package order

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/stretchr/testify/assert"
)

func TestOrder_New_Success(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	orderType := TypeIn

	o := New(orderType, WithTenantID(tenantID), WithID(1))

	assert.Equal(t, uint(1), o.ID())
	assert.Equal(t, tenantID, o.TenantID())
	assert.Equal(t, orderType, o.Type())
	assert.Equal(t, Pending, o.Status())
	assert.Empty(t, o.Items())
	assert.NotNil(t, o.Events())
	assert.Len(t, o.Events(), 1)
}

func TestOrder_New_EmitsOrderCreatedEvent(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	orderType := TypeOut

	o := New(orderType, WithTenantID(tenantID), WithID(42))

	events := o.Events()
	assert.Len(t, events, 1)

	createdEvent, ok := events[0].(OrderCreatedEvent)
	assert.True(t, ok, "First event should be OrderCreatedEvent")
	assert.Equal(t, uint(42), createdEvent.OrderID)
	assert.Equal(t, orderType, createdEvent.Type)
	assert.Equal(t, tenantID, createdEvent.TenantID)
	assert.NotZero(t, createdEvent.Timestamp)
}

func TestOrder_AddItem_Success(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Test Product", "BARCODE123", position.WithTenantID(tenantID), position.WithID(1))
	prod := product.New("RFID001", product.InStock, product.WithTenantID(tenantID), product.WithID(1))

	o := New(TypeIn, WithTenantID(tenantID), WithID(1))

	newOrder, err := o.AddItem(pos, prod)

	assert.NoError(t, err)
	assert.NotNil(t, newOrder)
	assert.Len(t, newOrder.Items(), 1)
	assert.Equal(t, pos.ID(), newOrder.Items()[0].Position().ID())
	assert.Len(t, newOrder.Items()[0].Products(), 1)
	assert.Equal(t, prod.ID(), newOrder.Items()[0].Products()[0].ID())
}

func TestOrder_AddItem_MultipleProducts(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Test Product", "BARCODE123", position.WithTenantID(tenantID), position.WithID(1))
	prod1 := product.New("RFID001", product.InStock, product.WithTenantID(tenantID), product.WithID(1))
	prod2 := product.New("RFID002", product.InStock, product.WithTenantID(tenantID), product.WithID(2))

	o := New(TypeIn, WithTenantID(tenantID), WithID(1))

	newOrder, err := o.AddItem(pos, prod1, prod2)

	assert.NoError(t, err)
	assert.Len(t, newOrder.Items(), 1)
	assert.Equal(t, 2, newOrder.Items()[0].Quantity())
	assert.Equal(t, prod1.ID(), newOrder.Items()[0].Products()[0].ID())
	assert.Equal(t, prod2.ID(), newOrder.Items()[0].Products()[1].ID())
}

func TestOrder_AddItem_EmitsItemAddedEvent(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Test Product", "BARCODE123", position.WithTenantID(tenantID), position.WithID(1))
	prod := product.New("RFID001", product.InStock, product.WithTenantID(tenantID), product.WithID(1))

	o := New(TypeIn, WithTenantID(tenantID), WithID(1))
	newOrder, err := o.AddItem(pos, prod)

	assert.NoError(t, err)

	events := newOrder.Events()
	assert.Len(t, events, 2)

	addedEvent, ok := events[1].(ItemAddedEvent)
	assert.True(t, ok, "Second event should be ItemAddedEvent")
	assert.Equal(t, uint(1), addedEvent.OrderID)
	assert.Equal(t, pos.ID(), addedEvent.Position.ID())
	assert.Len(t, addedEvent.Products, 1)
	assert.Equal(t, prod.ID(), addedEvent.Products[0].ID())
}

func TestOrder_AddItem_ShippedProduct_Error(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Test Product", "BARCODE123", position.WithTenantID(tenantID), position.WithID(1))
	shippedProd := product.New("RFID001", product.Shipped, product.WithTenantID(tenantID), product.WithID(1))

	o := New(TypeIn, WithTenantID(tenantID), WithID(1))

	newOrder, err := o.AddItem(pos, shippedProd)

	assert.Error(t, err)
	assert.Nil(t, newOrder)

	shippedErr, ok := err.(*ProductIsShippedError)
	assert.True(t, ok, "Error should be ProductIsShippedError")
	assert.Equal(t, product.Shipped, shippedErr.Current)
}

func TestOrder_AddItem_MixedProductsWithShipped_Error(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Test Product", "BARCODE123", position.WithTenantID(tenantID), position.WithID(1))
	goodProd := product.New("RFID001", product.InStock, product.WithTenantID(tenantID), product.WithID(1))
	shippedProd := product.New("RFID002", product.Shipped, product.WithTenantID(tenantID), product.WithID(2))

	o := New(TypeIn, WithTenantID(tenantID), WithID(1))

	newOrder, err := o.AddItem(pos, goodProd, shippedProd)

	assert.Error(t, err)
	assert.Nil(t, newOrder)

	shippedErr, ok := err.(*ProductIsShippedError)
	assert.True(t, ok, "Error should be ProductIsShippedError")
	assert.Equal(t, product.Shipped, shippedErr.Current)
}

func TestOrder_AddItem_Immutability(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Test Product", "BARCODE123", position.WithTenantID(tenantID), position.WithID(1))
	prod := product.New("RFID001", product.InStock, product.WithTenantID(tenantID), product.WithID(1))

	originalOrder := New(TypeIn, WithTenantID(tenantID), WithID(1))
	newOrder, err := originalOrder.AddItem(pos, prod)

	assert.NoError(t, err)

	// Original order should remain unchanged
	assert.Empty(t, originalOrder.Items())
	// New order should have the item
	assert.Len(t, newOrder.Items(), 1)
	// They should be different instances
	assert.NotEqual(t, originalOrder, newOrder)
}

func TestOrder_Complete_Success(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	o := New(TypeIn, WithTenantID(tenantID), WithID(1), WithStatus(Pending))

	completedOrder, err := o.Complete()

	assert.NoError(t, err)
	assert.NotNil(t, completedOrder)
	assert.Equal(t, Complete, completedOrder.Status())
	assert.NotEqual(t, o, completedOrder)
}

func TestOrder_Complete_AlreadyComplete_Error(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	o := New(TypeIn, WithTenantID(tenantID), WithID(1), WithStatus(Complete))

	completedOrder, err := o.Complete()

	assert.Error(t, err)
	assert.Nil(t, completedOrder)

	completeErr, ok := err.(*OrderIsCompleteError)
	assert.True(t, ok, "Error should be OrderIsCompleteError")
	assert.Equal(t, Complete, completeErr.Current)
}

func TestOrder_Complete_EmitsOrderCompletedEvent(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	o := New(TypeIn, WithTenantID(tenantID), WithID(42), WithStatus(Pending))

	completedOrder, err := o.Complete()

	assert.NoError(t, err)

	events := completedOrder.Events()
	assert.Greater(t, len(events), 1)

	completedEvent, ok := events[len(events)-1].(OrderCompletedEvent)
	assert.True(t, ok, "Last event should be OrderCompletedEvent")
	assert.Equal(t, uint(42), completedEvent.OrderID)
	assert.NotZero(t, completedEvent.Timestamp)
}

func TestOrder_Complete_Immutability(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	originalOrder := New(TypeIn, WithTenantID(tenantID), WithID(1), WithStatus(Pending))

	completedOrder, err := originalOrder.Complete()

	assert.NoError(t, err)

	// Original order should remain pending
	assert.Equal(t, Pending, originalOrder.Status())
	// New order should be complete
	assert.Equal(t, Complete, completedOrder.Status())
	// They should be different instances
	assert.NotEqual(t, originalOrder, completedOrder)
}

func TestOrder_SetTenantID_Immutability(t *testing.T) {
	t.Parallel()

	originalTenantID := uuid.New()
	newTenantID := uuid.New()

	o := New(TypeIn, WithTenantID(originalTenantID), WithID(1))
	newOrder := o.SetTenantID(newTenantID)

	// Original should be unchanged
	assert.Equal(t, originalTenantID, o.TenantID())
	// New should have new tenant ID
	assert.Equal(t, newTenantID, newOrder.TenantID())
	// Should be different instances
	assert.NotEqual(t, o, newOrder)
}

func TestOrder_SetStatus_Immutability(t *testing.T) {
	t.Parallel()

	o := New(TypeIn, WithID(1), WithStatus(Pending))
	newOrder := o.SetStatus(Complete)

	// Original should be unchanged
	assert.Equal(t, Pending, o.Status())
	// New should have new status
	assert.Equal(t, Complete, newOrder.Status())
	// Should be different instances
	assert.NotEqual(t, o, newOrder)
}

func TestOrder_SetItems_Immutability(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos1 := position.New("Product 1", "BAR001", position.WithTenantID(tenantID), position.WithID(1))
	pos2 := position.New("Product 2", "BAR002", position.WithTenantID(tenantID), position.WithID(2))

	prod1 := product.New("RFID001", product.InStock, product.WithTenantID(tenantID), product.WithID(1))
	prod2 := product.New("RFID002", product.InStock, product.WithTenantID(tenantID), product.WithID(2))

	o := New(TypeIn, WithTenantID(tenantID), WithID(1))

	item1 := &item{position: pos1, products: []product.Product{prod1}}
	item2 := &item{position: pos2, products: []product.Product{prod2}}
	newItems := []Item{item1, item2}

	newOrder := o.SetItems(newItems)

	// Original should have no items
	assert.Empty(t, o.Items())
	// New should have items
	assert.Len(t, newOrder.Items(), 2)
	// Should be different instances
	assert.NotEqual(t, o, newOrder)
}

func TestOrder_UpdatedAt_ChangesOnSetter(t *testing.T) {
	t.Parallel()

	o := New(TypeIn, WithID(1))
	originalUpdatedAt := o.UpdatedAt()

	time.Sleep(10 * time.Millisecond) // Ensure time difference

	newOrder := o.SetStatus(Complete)

	// Original should have original updatedAt
	assert.Equal(t, originalUpdatedAt, o.UpdatedAt())
	// New should have newer updatedAt
	assert.True(t, newOrder.UpdatedAt().After(originalUpdatedAt), "updatedAt should be newer")
}

func TestOrder_MultipleAdditions(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos1 := position.New("Product 1", "BAR001", position.WithTenantID(tenantID), position.WithID(1))
	pos2 := position.New("Product 2", "BAR002", position.WithTenantID(tenantID), position.WithID(2))

	prod1 := product.New("RFID001", product.InStock, product.WithTenantID(tenantID), product.WithID(1))
	prod2 := product.New("RFID002", product.InStock, product.WithTenantID(tenantID), product.WithID(2))

	o := New(TypeIn, WithTenantID(tenantID), WithID(1))

	o1, err1 := o.AddItem(pos1, prod1)
	assert.NoError(t, err1)
	assert.Len(t, o1.Items(), 1)

	o2, err2 := o1.AddItem(pos2, prod2)
	assert.NoError(t, err2)
	assert.Len(t, o2.Items(), 2)

	// Original should be unchanged
	assert.Empty(t, o.Items())
	// First addition should be unchanged
	assert.Len(t, o1.Items(), 1)
	// Second addition should have both items
	assert.Len(t, o2.Items(), 2)
}

func TestOrder_CreatedAt_RemainsConstant(t *testing.T) {
	t.Parallel()

	o := New(TypeIn, WithID(1))
	originalCreatedAt := o.CreatedAt()

	time.Sleep(10 * time.Millisecond)

	newOrder := o.SetStatus(Complete)

	// Both should have same createdAt
	assert.Equal(t, originalCreatedAt, newOrder.CreatedAt())
}

func TestOrder_Item_Quantity(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	pos := position.New("Test", "BAR001", position.WithTenantID(tenantID), position.WithID(1))

	prod1 := product.New("RFID001", product.InStock, product.WithTenantID(tenantID), product.WithID(1))
	prod2 := product.New("RFID002", product.InStock, product.WithTenantID(tenantID), product.WithID(2))
	prod3 := product.New("RFID003", product.InStock, product.WithTenantID(tenantID), product.WithID(3))

	o := New(TypeIn, WithTenantID(tenantID), WithID(1))
	newOrder, err := o.AddItem(pos, prod1, prod2, prod3)

	assert.NoError(t, err)
	assert.Equal(t, 3, newOrder.Items()[0].Quantity())
}

func TestOrder_StatusValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  Status
		isValid bool
	}{
		{"Pending valid", Pending, true},
		{"Complete valid", Complete, true},
		{"Invalid status", Status("invalid"), false},
		{"Empty status", Status(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isValid, tt.status.IsValid())
		})
	}
}

func TestOrder_TypeValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typ     Type
		isValid bool
	}{
		{"TypeIn valid", TypeIn, true},
		{"TypeOut valid", TypeOut, true},
		{"Invalid type", Type("invalid"), false},
		{"Empty type", Type(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isValid, tt.typ.IsValid())
		})
	}
}

func TestNewStatus_Success(t *testing.T) {
	t.Parallel()

	status, err := NewStatus("pending")
	assert.NoError(t, err)
	assert.Equal(t, Pending, status)

	status, err = NewStatus("complete")
	assert.NoError(t, err)
	assert.Equal(t, Complete, status)
}

func TestNewStatus_InvalidValue(t *testing.T) {
	t.Parallel()

	status, err := NewStatus("invalid")
	assert.Error(t, err)
	assert.Equal(t, Status(""), status)
}

func TestNewType_Success(t *testing.T) {
	t.Parallel()

	typ, err := NewType("in")
	assert.NoError(t, err)
	assert.Equal(t, TypeIn, typ)

	typ, err = NewType("out")
	assert.NoError(t, err)
	assert.Equal(t, TypeOut, typ)
}

func TestNewType_InvalidValue(t *testing.T) {
	t.Parallel()

	typ, err := NewType("invalid")
	assert.Error(t, err)
	assert.Equal(t, Type(""), typ)
}
