package controllers_test

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/productservice"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	importpkg "github.com/iota-uz/iota-sdk/pkg/import"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func setupImport(t *testing.T) (*itf.TestEnvironment, *controllers.PositionRowHandler, businessevent.Repository) {
	t.Helper()
	env := itf.Setup(t, itf.WithComponents(modules.Components()...), itf.WithUser(itf.User(permissions.Permissions...)))
	writer, err := composition.Resolve[history.Writer](env.Container)
	require.NoError(t, err)
	events, err := composition.Resolve[businessevent.Repository](env.Container)
	require.NoError(t, err)
	handler := controllers.NewPositionRowHandler(itf.GetService[positionservice.PositionService](env), importpkg.NewDefaultErrorFactory(), writer)
	return env, handler, events
}

func afterValue(t *testing.T, e businessevent.Event, field string) string {
	t.Helper()
	for _, d := range e.Changes[0].Fields {
		if d.Field == field {
			var s string
			if err := json.Unmarshal(d.After.Value, &s); err == nil {
				return s
			}
			return string(d.After.Value)
		}
	}
	return ""
}

func TestPositionRowHandler_PartialImport(t *testing.T) {
	t.Parallel()
	env, handler, eventRepo := setupImport(t)
	ctx := history.ImportExecution(env.Ctx, "upload-42")
	exec, _ := history.UseExecution(ctx)

	rows := map[int][]string{
		1: {"Drill", "B-1", "pc", "2"},
		2: {strings.Repeat("x", 300), "B-2", "box", "1"},
		3: {"Saw", "B-3", "pc", "1"},
	}
	for i := 1; i <= 3; i++ {
		err := handler.ProcessRow(ctx, i, rows[i])
		if i == 2 {
			require.ErrorContains(t, err, "SQLSTATE 22001", "row 2 must fail on the position insert, after its unit was created")
			continue
		}
		require.NoError(t, err)
	}

	positions, err := persistence.NewPositionRepository().GetAll(env.Ctx)
	require.NoError(t, err)
	positionIDs := map[string]string{}
	for _, p := range positions {
		positionIDs[strconv.FormatUint(uint64(p.ID()), 10)] = p.Barcode()
	}
	require.ElementsMatch(t, []string{"B-1", "B-3"}, mapValues(positionIDs))
	units, err := persistence.NewUnitRepository().GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, units, 1)

	events, err := eventRepo.ListByOperation(env.Ctx, exec.OperationID)
	require.NoError(t, err)
	byType := map[string][]businessevent.Event{}
	for _, e := range events {
		require.Equal(t, businessevent.ChannelImport, e.Channel)
		require.Equal(t, exec.OperationID, e.OperationID)
		require.Equal(t, "upload-42", e.RunID)
		byType[e.Type] = append(byType[e.Type], e)
	}

	require.Len(t, byType[services.UnitCreated], 1)
	require.Equal(t, "pc", afterValue(t, byType[services.UnitCreated][0], "title"))

	require.Len(t, byType[positionservice.PositionCreated], 2)
	barcodes := make([]string, 0, len(byType[positionservice.PositionCreated]))
	for _, e := range byType[positionservice.PositionCreated] {
		require.Contains(t, positionIDs, e.Subjects[0].ID)
		barcodes = append(barcodes, afterValue(t, e, "barcode"))
	}
	require.ElementsMatch(t, []string{"B-1", "B-3"}, barcodes)

	require.Len(t, byType[productservice.ProductCreated], 3)
	for _, e := range byType[productservice.ProductCreated] {
		require.Contains(t, positionIDs, afterValue(t, e, "position_id"))
	}

	require.Len(t, byType[history.ImportItemType], 1)
	failed := byType[history.ImportItemType][0]
	require.Equal(t, businessevent.OutcomeFailed, failed.Outcome)
	require.Equal(t, []businessevent.Subject{{Resource: history.ImportItemResource, ID: "row:2"}}, failed.Subjects)
	require.Empty(t, failed.Changes)

	require.Len(t, events, 1+2+3+1, "row 2 must leave no events of its objects")
}

func TestPositionRowHandler_RecordRejectedRows(t *testing.T) {
	t.Parallel()
	env, handler, eventRepo := setupImport(t)
	ctx := history.ImportExecution(env.Ctx, "upload-7")
	exec, _ := history.UseExecution(ctx)
	factory := importpkg.NewDefaultErrorFactory()

	require.NoError(t, handler.RecordRejectedRows(ctx, []error{
		factory.NewInvalidCellError("A", 2),
		factory.NewValidationError("D", "abc", 5, "Must be a valid number"),
	}))

	events, err := eventRepo.ListByOperation(env.Ctx, exec.OperationID)
	require.NoError(t, err)
	items := make([]string, 0, len(events))
	for _, e := range events {
		require.Equal(t, history.ImportItemType, e.Type)
		require.Equal(t, businessevent.OutcomeFailed, e.Outcome)
		require.Equal(t, businessevent.ChannelImport, e.Channel)
		items = append(items, e.Subjects[0].ID)
	}
	require.ElementsMatch(t, []string{"row:2", "row:5"}, items)
}

func mapValues(m map[string]string) []string {
	values := make([]string, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}
