package history

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
)

// ImportItemType is the event of an import item that was rejected: its own events were rolled
// back with it, so this event is the only trace of the item in the operation.
const ImportItemType = "import.item"

const ImportItemResource = "import.item"

func ImportItemDefinition() Definition {
	return Definition{
		Type:           ImportItemType,
		Version:        1,
		Category:       businessevent.CategoryBusiness,
		TitleKey:       "BusinessHistory.Events.ImportItem.Title",
		DescriptionKey: "BusinessHistory.Events.ImportItem.Summary",
		Resources:      []Resource{{Name: ImportItemResource}},
	}
}

// ImportItem runs one item of a partially successful import in its own transaction (a savepoint
// when the import already runs in one), so the item's changes and events commit or roll back
// together. A failed item is recorded as a failed, or denied, import.item event outside of its
// rolled-back transaction, and fn's error is returned for the caller to report.
func ImportItem(ctx context.Context, w Writer, item string, fn func(context.Context) error) error {
	const op serrors.Op = "history.ImportItem"
	err := inItemTx(ctx, fn)
	if err == nil {
		return nil
	}
	outcome := businessevent.OutcomeFailed
	if errors.Is(err, composables.ErrForbidden) {
		outcome = businessevent.OutcomeDenied
	}
	recordErr := inItemTx(ctx, func(txCtx context.Context) error {
		_, err := w.Record(txCtx, Input{
			Type:     ImportItemType,
			Version:  1,
			Outcome:  outcome,
			Subjects: []businessevent.Subject{{Resource: ImportItemResource, ID: item}},
		})
		return err
	})
	if recordErr != nil {
		return errors.Join(err, serrors.Wrap(op, recordErr))
	}
	return err
}

func inItemTx(ctx context.Context, fn func(context.Context) error) error {
	tx, ok := ctx.Value(constants.TxKey).(pgx.Tx)
	if !ok {
		return composables.InTx(ctx, fn)
	}
	savepoint, err := tx.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(composables.WithTx(ctx, savepoint)); err != nil {
		if rbErr := savepoint.Rollback(ctx); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	return savepoint.Commit(ctx)
}
