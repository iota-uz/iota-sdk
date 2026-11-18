package native_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

type invoiceNoted struct {
	Number string  `json:"number"`
	Note   *string `json:"note,omitempty"`
}

var (
	noteOp  = native.Operation[line, line]("billing.operations.note", native.Spec{}, nil)
	notedEv = native.Event[invoiceNoted]("billing.invoice.noted.v1", native.EventSpec{Subject: "billing.invoice"})
)

func billing(s *native.Set) error {
	return s.Add(
		native.Entity(data.NativeEntity{ID: "billing.invoice", Table: "invoices", KeyColumn: "id", KeyType: data.KeyInt64, TenantColumn: "tenant_id",
			Fields: []data.NativeField{{ID: "number", Column: "number", Type: manifest.Type{Kind: manifest.KindString}}}}),
		native.List("billing.lists.invoices"),
		native.Route("billing.invoices", "/billing/invoices"),
		native.Declare(manifest.Resource{ID: "billing.slots.invoiceHeader", Kind: manifest.ResSlot, Props: &manifest.Type{Kind: manifest.KindObject}}),
		noteOp, notedEv,
	)
}

func TestComposeCarriesEveryNativeKindForEveryConsumer(t *testing.T) {
	set, err := native.Compose(billing)
	require.NoError(t, err)

	ids := map[string]manifest.ResourceKind{}
	for _, r := range set.Resources() {
		require.Equal(t, manifest.ContractHashOf(r), r.ContractHash, r.ID)
		ids[r.ID] = r.Kind
	}
	require.Equal(t, map[string]manifest.ResourceKind{
		"billing.invoice": manifest.ResEntity, "billing.invoice.noted.v1": manifest.ResEvent, "billing.invoices": manifest.ResRoute,
		"billing.operations.note": manifest.ResOperation, "billing.slots.invoiceHeader": manifest.ResSlot,
	}, ids, "lists are kit references, not resources")
	require.Equal(t, []string{"billing.lists.invoices"}, set.Lists())
	require.Equal(t, map[string]string{"billing.invoices": "/billing/invoices"}, set.Routes())
	_, ok := set.EntityRegistry().Get("billing.invoice")
	require.True(t, ok, "the data store reads the entities through the registry view")
	d, ok := set.Contract("billing.operations.note")
	require.True(t, ok)
	require.Equal(t, native.KindOperation, d.Kind())
	require.Len(t, set.ResourcesOf(manifest.ResSlot), 1)
	r, ok := set.Resource("billing.invoices")
	require.True(t, ok)
	require.Equal(t, "/billing/invoices", r.Path)
	_, ok = set.Resource("billing.lists.invoices")
	require.False(t, ok)
}

func TestSetRejectsDuplicatesAndMisroutedDeclarations(t *testing.T) {
	set, err := native.Compose(billing)
	require.NoError(t, err)
	require.NoError(t, set.Add(noteOp, notedEv), "the same contract and event values are idempotent")
	require.Error(t, set.Add(native.Operation[line, line]("billing.operations.note", native.Spec{}, nil)), "another contract under a taken ID")
	require.Error(t, set.Add(native.Route("billing.operations.note", "/x")), "IDs are unique across kinds")
	require.Error(t, set.Add(native.Route("billing.other", "relative")))
	require.Error(t, set.Add(native.List("Not A List")))
	require.Error(t, set.Add(native.Declare(manifest.Resource{ID: "billing.x", Kind: manifest.ResEvent})), "events carry their payload type: native.Event")
	require.Error(t, set.Add(native.Declare(manifest.Resource{ID: "billing.y"})), "a declaration needs a kind")
	require.Error(t, set.Add(native.Entity(data.NativeEntity{ID: "billing.bad", Table: "x y"})), "entity descriptors are validated")

	_, err = native.Compose(billing, billing)
	require.Error(t, err, "a module composed twice registers its entity, list and route twice")
	_, err = native.Compose(billing, func(*native.Set) error { return errors.New("boom") })
	require.ErrorContains(t, err, "boom")
}

func TestEventIntentValidatesPayloadAndSubject(t *testing.T) {
	note := "late"
	subject := &spi.ObjectRef{Entity: "billing.invoice", Key: "7"}
	in, err := notedEv.IntentOf(subject, invoiceNoted{Number: "I-1", Note: &note})
	require.NoError(t, err)
	require.Equal(t, "billing.invoice.noted.v1", in.Event)
	require.JSONEq(t, `{"number":"I-1","note":"late"}`, string(in.Payload))

	_, err = notedEv.Intent(subject, json.RawMessage(`{"number":5}`))
	require.ErrorContains(t, err, "payload of billing.invoice.noted.v1")
	_, err = notedEv.IntentOf(nil, invoiceNoted{Number: "I-1"})
	require.ErrorContains(t, err, "needs a subject")
	_, err = notedEv.IntentOf(&spi.ObjectRef{Entity: "core.user", Key: "1"}, invoiceNoted{Number: "I-1"})
	require.ErrorContains(t, err, "must be a billing.invoice ref")
	_, err = native.NewEvent[invoiceNoted]("billing.invoice.noted", native.EventSpec{})
	require.Error(t, err, "a native event ID ends in a version")
}

func TestContractRulesAreTheHostsAndDirectsOnly(t *testing.T) {
	op := noteOp.Resource()
	cmd := native.Command[line, line]("billing.commands.save", native.Spec{Permission: "Invoice.Write"}, nil)
	atomic := native.AtomicCommand[line, line]("billing.commands.post", native.Spec{}, nil).Resource()
	query := native.Query[line, line]("billing.queries.find", native.Spec{}, nil).Resource()
	code := func(err error) string {
		we, ok := native.AsError(err)
		if !ok {
			return ""
		}
		return we.Code
	}

	require.NoError(t, native.CheckCall(native.ViaTxCall, op))
	require.NoError(t, native.CheckCall(native.ViaTxCall, atomic))
	require.Equal(t, native.CodeOrdinaryCommandInTx, code(native.CheckCall(native.ViaTxCall, cmd.Resource())))
	require.Equal(t, native.CodeOrdinaryCommandInTx, code(native.CheckCall(native.ViaTxCall, query)))
	require.NoError(t, native.CheckCall(native.ViaCtxCall, atomic))
	require.Equal(t, native.CodeNotCommand, code(native.CheckCall(native.ViaCtxCall, op)))
	require.Equal(t, native.CodeNotQuery, code(native.CheckCall(native.ViaQuery, op)))

	require.Equal(t, []string{"Invoice.Write"}, native.Permissions(cmd.Resource(), cmd), "the RBAC name comes from the Go definition")
	require.Equal(t, native.CodePermissionDenied, code(native.CheckPermissions(func(string) bool { return false }, cmd.Resource(), cmd)))
	require.NoError(t, native.CheckPermissions(func(p string) bool { return p == "Invoice.Write" }, cmd.Resource(), cmd))

	require.Equal(t, native.CodeInvalidValue, code(native.CheckInput(op, json.RawMessage(`{"sku":5,"qty":1}`))))
	require.Equal(t, native.CodeInvalidOutput, code(native.CheckOutput(op, json.RawMessage(`{"sku":5,"qty":1}`))))
	require.NoError(t, native.CheckInput(op, json.RawMessage(`{"sku":"A","qty":1}`)))
	require.Equal(t, native.CodeNestingTooDeep, code(native.NestingTooDeep("x")))
}
