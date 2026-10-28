package services_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/orderservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/productservice"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type warehouse struct {
	env       *itf.TestEnvironment
	units     *services.UnitService
	inventory *services.InventoryService
	products  *productservice.ProductService
	positions *positionservice.PositionService
	orders    *orderservice.OrderService
	events    businessevent.Repository
}

func setupWarehouse(t *testing.T) warehouse {
	t.Helper()
	env := itf.Setup(t, itf.WithComponents(modules.Components()...), itf.WithUser(itf.User(permissions.Permissions...)))
	events, err := composition.Resolve[businessevent.Repository](env.Container)
	require.NoError(t, err)
	return warehouse{
		env:       env,
		units:     itf.GetService[services.UnitService](env),
		inventory: itf.GetService[services.InventoryService](env),
		products:  itf.GetService[productservice.ProductService](env),
		positions: itf.GetService[positionservice.PositionService](env),
		orders:    itf.GetService[orderservice.OrderService](env),
		events:    events,
	}
}

func (w warehouse) operation() context.Context {
	return itf.TestExecution(w.env.Ctx)
}

func (w warehouse) unit(t *testing.T, title string) *unit.Unit {
	t.Helper()
	u, err := w.units.Create(w.operation(), &unit.CreateDTO{Title: title, ShortTitle: title})
	require.NoError(t, err)
	return u
}

func (w warehouse) position(t *testing.T, barcode string) position.Position {
	t.Helper()
	p, err := w.positions.Create(w.operation(), &position.CreateDTO{Title: "Drill", Barcode: barcode, UnitID: w.unit(t, "pc-"+barcode).ID})
	require.NoError(t, err)
	return p
}

func (w warehouse) product(t *testing.T, rfid string) product.Product {
	t.Helper()
	pos := w.position(t, "P-"+rfid)
	require.NoError(t, w.products.Create(w.operation(), &product.CreateDTO{PositionID: pos.ID(), Rfid: rfid, Status: string(product.InStock)}))
	p, err := persistence.NewProductRepository().GetByRfid(w.env.Ctx, rfid)
	require.NoError(t, err)
	return p
}

func (w warehouse) order(t *testing.T) order.Order {
	t.Helper()
	require.NoError(t, w.orders.Create(w.operation(), order.CreateDTO{Type: string(order.TypeIn), Status: string(order.Pending)}))
	all, err := w.orders.GetAll(w.env.Ctx)
	require.NoError(t, err)
	require.NotEmpty(t, all)
	return all[len(all)-1]
}

func id(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

type fieldChange struct{ before, after string }

func TestWarehouseHistory_RecordsServiceActions(t *testing.T) {
	t.Parallel()
	w := setupWarehouse(t)

	cases := []struct {
		name      string
		act       func(t *testing.T, ctx context.Context) string
		eventType string
		resource  string
		fields    map[string]fieldChange
	}{
		{
			name: "unit created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				u, err := w.units.Create(ctx, &unit.CreateDTO{Title: "Kilogram", ShortTitle: "kg"})
				require.NoError(t, err)
				return id(u.ID)
			},
			eventType: services.UnitCreated, resource: services.UnitResource,
			fields: map[string]fieldChange{"title": {"", `"Kilogram"`}, "short_title": {"", `"kg"`}},
		},
		{
			name: "unit updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				u := w.unit(t, "Litre")
				require.NoError(t, w.units.Update(ctx, u.ID, &unit.UpdateDTO{Title: "Litre", ShortTitle: "l"}))
				return id(u.ID)
			},
			eventType: services.UnitUpdated, resource: services.UnitResource,
			fields: map[string]fieldChange{"short_title": {`"Litre"`, `"l"`}},
		},
		{
			name: "unit deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				u := w.unit(t, "Box")
				_, err := w.units.Delete(ctx, u.ID)
				require.NoError(t, err)
				return id(u.ID)
			},
			eventType: services.UnitDeleted, resource: services.UnitResource,
			fields: map[string]fieldChange{"title": {`"Box"`, ""}, "short_title": {`"Box"`, ""}},
		},
		{
			name: "position created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				u := w.unit(t, "pcs")
				p, err := w.positions.Create(ctx, &position.CreateDTO{Title: "Hammer", Barcode: "100", UnitID: u.ID})
				require.NoError(t, err)
				return id(p.ID())
			},
			eventType: positionservice.PositionCreated, resource: positionservice.PositionResource,
			fields: map[string]fieldChange{"title": {"", `"Hammer"`}, "barcode": {"", `"100"`}},
		},
		{
			name: "position updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				p := w.position(t, "200")
				require.NoError(t, w.positions.Update(ctx, p.ID(), &position.UpdateDTO{Title: "Impact drill", Barcode: "200", UnitID: p.UnitID()}))
				return id(p.ID())
			},
			eventType: positionservice.PositionUpdated, resource: positionservice.PositionResource,
			fields: map[string]fieldChange{"title": {`"Drill"`, `"Impact drill"`}},
		},
		{
			name: "position deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				p := w.position(t, "300")
				_, err := w.positions.Delete(ctx, p.ID())
				require.NoError(t, err)
				return id(p.ID())
			},
			eventType: positionservice.PositionDeleted, resource: positionservice.PositionResource,
			fields: map[string]fieldChange{"title": {`"Drill"`, ""}, "barcode": {`"300"`, ""}},
		},
		{
			name: "product created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				pos := w.position(t, "400")
				require.NoError(t, w.products.Create(ctx, &product.CreateDTO{PositionID: pos.ID(), Rfid: "RFID-0", Status: string(product.InDevelopment)}))
				p, err := persistence.NewProductRepository().GetByRfid(w.env.Ctx, "RFID-0")
				require.NoError(t, err)
				return id(p.ID())
			},
			eventType: productservice.ProductCreated, resource: productservice.ProductResource,
			fields: map[string]fieldChange{"rfid": {"", `"RFID-0"`}, "status": {"", `"in_development"`}},
		},
		{
			name: "product updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				p := w.product(t, "RFID-1")
				require.NoError(t, w.products.Update(ctx, p.ID(), &product.UpdateDTO{PositionID: p.PositionID(), Rfid: "RFID-1", Status: string(product.Approved)}))
				return id(p.ID())
			},
			eventType: productservice.ProductUpdated, resource: productservice.ProductResource,
			fields: map[string]fieldChange{"status": {`"in_stock"`, `"approved"`}},
		},
		{
			name: "product deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				p := w.product(t, "RFID-2")
				_, err := w.products.Delete(ctx, p.ID())
				require.NoError(t, err)
				return id(p.ID())
			},
			eventType: productservice.ProductDeleted, resource: productservice.ProductResource,
			fields: map[string]fieldChange{"rfid": {`"RFID-2"`, ""}, "status": {`"in_stock"`, ""}},
		},
		{
			name: "order created",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				require.NoError(t, w.orders.Create(ctx, order.CreateDTO{Type: string(order.TypeOut), Status: string(order.Pending)}))
				all, err := w.orders.GetAll(w.env.Ctx)
				require.NoError(t, err)
				return id(all[len(all)-1].ID())
			},
			eventType: orderservice.OrderCreated, resource: orderservice.OrderResource,
			fields: map[string]fieldChange{"type": {"", `"out"`}, "status": {"", `"pending"`}},
		},
		{
			name: "order updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				o := w.order(t)
				require.NoError(t, w.orders.Update(ctx, o.ID(), order.UpdateDTO{Type: string(order.TypeOut), Status: string(order.Pending)}))
				return id(o.ID())
			},
			eventType: orderservice.OrderUpdated, resource: orderservice.OrderResource,
			fields: map[string]fieldChange{"type": {`"in"`, `"out"`}},
		},
		{
			name: "order completed",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				o := w.order(t)
				_, err := w.orders.Complete(ctx, o.ID())
				require.NoError(t, err)
				return id(o.ID())
			},
			eventType: orderservice.OrderCompleted, resource: orderservice.OrderResource,
			fields: map[string]fieldChange{"status": {`"pending"`, `"complete"`}},
		},
		{
			name: "order deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				o := w.order(t)
				_, err := w.orders.Delete(ctx, o.ID())
				require.NoError(t, err)
				return id(o.ID())
			},
			eventType: orderservice.OrderDeleted, resource: orderservice.OrderResource,
			fields: map[string]fieldChange{"type": {`"in"`, ""}, "status": {`"pending"`, ""}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := w.operation()
			subjectID := tc.act(t, ctx)
			w.requireSingleEvent(t, ctx, tc.eventType, tc.resource, subjectID, tc.fields)
		})
	}
}

// Checks are inserted directly: the inventory repository does not yet fill the required type of a
// check, so InventoryService.Create cannot store one.
func TestWarehouseHistory_RecordsInventoryCheckActions(t *testing.T) {
	t.Parallel()
	w := setupWarehouse(t)
	email, err := internet.NewEmail("auditor@example.com")
	require.NoError(t, err)
	auditor, err := corepersistence.NewUserRepository(corepersistence.NewUploadRepository()).Create(w.env.Ctx,
		user.New("Inventory", "Auditor", email, user.UILanguageEN, user.WithTenantID(w.env.TenantID())))
	require.NoError(t, err)
	check := func(t *testing.T, name string) uint {
		t.Helper()
		var checkID uint
		require.NoError(t, w.env.Tx.QueryRow(w.env.Ctx, `INSERT INTO inventory_checks (tenant_id, status, name, type, created_by_id)
			VALUES ($1, 'incomplete', $2, 'full', $3) RETURNING id`, w.env.TenantID(), name, auditor.ID()).Scan(&checkID))
		return checkID
	}

	cases := []struct {
		name      string
		act       func(t *testing.T, ctx context.Context) string
		eventType string
		fields    map[string]fieldChange
	}{
		{
			name: "updated",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				checkID := check(t, "November check")
				require.NoError(t, w.inventory.Update(ctx, checkID, &inventory.UpdateCheckDTO{Name: "November recount"}))
				return id(checkID)
			},
			eventType: services.InventoryCheckUpdated,
			fields:    map[string]fieldChange{"name": {`"November check"`, `"November recount"`}},
		},
		{
			name: "deleted",
			act: func(t *testing.T, ctx context.Context) string {
				t.Helper()
				checkID := check(t, "December check")
				_, err := w.inventory.Delete(ctx, checkID)
				require.NoError(t, err)
				return id(checkID)
			},
			eventType: services.InventoryCheckDeleted,
			fields:    map[string]fieldChange{"name": {`"December check"`, ""}, "status": {`"incomplete"`, ""}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := w.operation()
			subjectID := tc.act(t, ctx)
			w.requireSingleEvent(t, ctx, tc.eventType, services.InventoryCheckResource, subjectID, tc.fields)
		})
	}
}

func (w warehouse) requireSingleEvent(t *testing.T, ctx context.Context, eventType, resource, subjectID string, fields map[string]fieldChange) {
	t.Helper()
	exec, ok := history.UseExecution(ctx)
	require.True(t, ok)
	events, err := w.events.ListByOperation(w.env.Ctx, exec.OperationID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	event := events[0]
	require.Equal(t, eventType, event.Type)
	require.Equal(t, businessevent.OutcomeSucceeded, event.Outcome)
	require.Equal(t, []businessevent.Subject{{Resource: resource, ID: subjectID}}, event.Subjects)
	require.Len(t, event.Changes, 1)
	got := map[string]fieldChange{}
	for _, d := range event.Changes[0].Fields {
		got[d.Field] = fieldChange{string(d.Before.Value), string(d.After.Value)}
	}
	for field, want := range fields {
		require.Equal(t, want, got[field], field)
	}
}

func TestWarehouseHistory_BulkCreateRecordsEachProduct(t *testing.T) {
	t.Parallel()
	w := setupWarehouse(t)
	pos := w.position(t, "BULK")
	ctx := w.operation()

	created, err := w.products.CreateProductsFromTags(ctx, &product.CreateProductsFromTagsDTO{Tags: []string{"T-1", "T-2", "T-3"}, PositionID: pos.ID()})
	require.NoError(t, err)
	require.Len(t, created, 3)

	exec, _ := history.UseExecution(ctx)
	events, err := w.events.ListByOperation(w.env.Ctx, exec.OperationID)
	require.NoError(t, err)
	require.Len(t, events, 3)
	subjects := map[string]bool{}
	for _, e := range events {
		require.Equal(t, productservice.ProductCreated, e.Type)
		subjects[e.Subjects[0].ID] = true
	}
	for _, p := range created {
		require.NotZero(t, p.ID())
		require.True(t, subjects[id(p.ID())], "no event for product %d", p.ID())
	}
}

func TestWarehouseHistory_RolledBackActionLeavesNoEvents(t *testing.T) {
	t.Parallel()
	w := setupWarehouse(t)
	u := w.unit(t, "pc")
	ctx := w.operation()

	savepoint, err := w.env.Tx.Begin(ctx)
	require.NoError(t, err)
	txCtx := composables.WithTx(ctx, savepoint)
	_, err = w.positions.Create(txCtx, &position.CreateDTO{Title: "Ghost", Barcode: "404", UnitID: u.ID})
	require.NoError(t, err)
	require.NoError(t, w.orders.Create(txCtx, order.CreateDTO{Type: string(order.TypeOut), Status: string(order.Pending)}))
	require.NoError(t, savepoint.Rollback(ctx))

	exec, _ := history.UseExecution(ctx)
	events, err := w.events.ListByOperation(w.env.Ctx, exec.OperationID)
	require.NoError(t, err)
	require.Empty(t, events)
}
