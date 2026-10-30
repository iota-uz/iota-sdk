package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Snapshot is the recorded state of one object: field name to a value in the JSON form of its type.
type Snapshot map[string]any

// Mutation is a committed change of one object. Before is nil for a created object, After for a deleted one.
type Mutation struct {
	Type     string
	Version  int
	Resource string
	ID       string
	Before   Snapshot
	After    Snapshot
}

// RecordMutation records the mutation in the transaction carried by ctx. An update that changed
// none of the recorded fields records nothing. Updates are keyed by their changes, so distinct
// updates of one object within one operation are separate events while a retried one is not.
func RecordMutation(ctx context.Context, w Writer, m Mutation) error {
	const op serrors.Op = "history.RecordMutation"
	change, err := m.change()
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if m.Before != nil && m.After != nil && len(change.Fields) == 0 {
		return nil
	}
	in := Input{
		Type:     m.Type,
		Version:  m.Version,
		Outcome:  businessevent.OutcomeSucceeded,
		Subjects: []businessevent.Subject{{Resource: m.Resource, ID: m.ID}},
		Changes:  []businessevent.Change{change},
	}
	if m.Before != nil && m.After != nil {
		data, err := json.Marshal(change.Fields)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		sum := sha256.Sum256(data)
		in.Key = subjectRef(m.Resource, m.ID) + "\n" + hex.EncodeToString(sum[:])
	}
	if _, err := w.Record(ctx, in); err != nil {
		return serrors.Wrap(op, err)
	}
	return nil
}

func (m Mutation) change() (businessevent.Change, error) {
	names := make([]string, 0, len(m.Before)+len(m.After))
	for name := range m.Before {
		names = append(names, name)
	}
	for name := range m.After {
		if _, ok := m.Before[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	fields := make([]fielddiff.Diff, 0, len(names))
	for _, name := range names {
		before, err := snapshotValue(m.Before, name)
		if err != nil {
			return businessevent.Change{}, err
		}
		after, err := snapshotValue(m.After, name)
		if err != nil {
			return businessevent.Change{}, err
		}
		if before.State == after.State && string(before.Value) == string(after.Value) {
			continue
		}
		fields = append(fields, fielddiff.Diff{Field: name, Before: before, After: after})
	}
	return businessevent.Change{Resource: m.Resource, ID: m.ID, Fields: fields}, nil
}

func snapshotValue(s Snapshot, name string) (fielddiff.FieldValue, error) {
	v, ok := s[name]
	if !ok {
		return fielddiff.Absent(), nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fielddiff.FieldValue{}, err
	}
	return fielddiff.Present(data), nil
}

// Money is the ValueMoney form of m; nil stays null.
func Money(m *money.Money) any {
	if m == nil {
		return nil
	}
	return map[string]string{"amount": MinorUnits(m.BigAmount(), m.Currency().Fraction), "currency": m.Currency().Code}
}

// MinorUnits is the ValueDecimal form of an amount kept in minor units with the given fraction digits.
func MinorUnits(amount *big.Int, fraction int) string {
	digits := new(big.Int).Abs(amount).String()
	sign := ""
	if amount.Sign() < 0 {
		sign = "-"
	}
	if fraction <= 0 {
		return sign + digits
	}
	if len(digits) <= fraction {
		digits = strings.Repeat("0", fraction-len(digits)+1) + digits
	}
	return sign + digits[:len(digits)-fraction] + "." + digits[len(digits)-fraction:]
}

// UUID is the ValueString form of id; the nil UUID is null.
func UUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id.String()
}

// Date is the ValueDate form of t; the zero time is null.
func Date(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format(time.DateOnly)
}

// DateTime is the ValueDateTime form of t; the zero time is null.
func DateTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// ContributeDefinitions registers a component's event types in the shared registry when the
// container is built, so they are known before any service records or any reader renders them.
func ContributeDefinitions(builder *composition.Builder, defs ...Definition) {
	composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
		registry, err := composition.Resolve[*Registry](container)
		if err != nil {
			return nil, err
		}
		return nil, registry.Register(defs...)
	})
}
