package e2e_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
)

type orderRef = map[string]string

type checkoutOut struct {
	Cents  int    `json:"cents"`
	Status string `json:"status"`
}

func (e *env) newOrder(a *actor, number string, cents int) orderRef {
	e.t.Helper()
	var out struct {
		Order orderRef `json:"order"`
	}
	e.mustRPC(a, "command", "e2e.pricing.createOrder", map[string]any{"number": number, "cents": cents}, &out)
	return out.Order
}

func (e *env) readOrder(a *actor, o orderRef) checkoutOut {
	e.t.Helper()
	var out checkoutOut
	e.mustRPC(a, "query", "e2e.pricing.readOrder", map[string]any{"order": o}, &out)
	return out
}

func (e *env) checkout(a *actor, o orderRef, percent int) rpcResult {
	return e.rpc(a, "command", "e2e.pricing.checkout", map[string]any{"order": o, "percent": percent})
}

// Scenario 3: a published command is replaced by another package through the
// published operation only; an independent guard of a third package still
// applies after the replacement; native/foreign generic writes are impossible
// (at check time and at run time); reverting the binding restores the
// previous behavior.
func TestScenario03CommandReplacementWithIndependentGuard(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	user := e.newActor("shopper")

	e.deploy(admin, "e2e.pricing", "1.0.0", loadTree(t, "pricing"))
	e.deploy(admin, "e2e.limits", "1.0.0", loadTree(t, "limits"))

	// Original behavior: the package's own checkout, the guard of the
	// independent package on the operation it calls.
	a := e.newOrder(user, "A-1", 10000)
	var out checkoutOut
	res := e.checkout(user, a, 20)
	require.True(t, res.OK, "%+v", res.Error)
	require.NoError(t, jsonUnmarshal(res.Value, &out))
	require.Equal(t, checkoutOut{Cents: 8000, Status: "paid"}, out)
	b := e.newOrder(user, "A-2", 10000)
	require.True(t, e.checkout(user, b, 22).OK, "22% is allowed by the guard")

	c := e.newOrder(user, "A-3", 10000)
	denied := e.checkout(user, c, 30)
	require.False(t, denied.OK)
	require.Equal(t, http.StatusUnprocessableEntity, denied.Status)
	require.Equal(t, "validation", denied.Error.Kind)
	require.Len(t, denied.Error.Issues, 1)
	require.Equal(t, "e2e.limits.discountTooHigh", denied.Error.Issues[0].Code)
	require.JSONEq(t, `{"max":25}`, string(denied.Error.Issues[0].Params))
	require.Equal(t, "e2e.limits.maxDiscount", denied.Error.Issues[0].Source, "the issue names the guard that raised it")
	require.Equal(t, checkoutOut{Cents: 10000, Status: "open"}, e.readOrder(user, c), "a refused checkout leaves no trace (same transaction)")

	// The replacement is built against the published contracts; activating it
	// does not replace anything by itself.
	e.deploy(admin, "e2e.promo", "1.0.0", loadTree(t, "promo"))
	d := e.newOrder(user, "B-1", 10000)
	res = e.checkout(user, d, 10)
	require.True(t, res.OK)
	require.NoError(t, jsonUnmarshal(res.Value, &out))
	require.Equal(t, checkoutOut{Cents: 9000, Status: "paid"}, out, "no binding: the original implementation serves")

	// A replacement of another package's command is an explicit decision.
	e.bind(admin, manage.SlotCommand, "e2e.pricing.checkout", "e2e.promo", "1.0.0", "checkout", http.StatusCreated)
	e.bind(admin, manage.SlotCommand, "e2e.pricing.checkout", "e2e.promo", "1.0.0", "noSuchExport", http.StatusBadRequest)
	f := e.newOrder(user, "B-2", 10000)
	res = e.checkout(user, f, 10)
	require.True(t, res.OK, "%+v", res.Error)
	require.NoError(t, jsonUnmarshal(res.Value, &out))
	require.Equal(t, checkoutOut{Cents: 8500, Status: "promo"}, out, "the replacement computed the result (10% + 5% bonus)")
	require.Equal(t, "open", e.readOrder(user, f).Status, "the replacement cannot write the foreign entity, so the order is still open")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.invocations WHERE contract = 'e2e.pricing.checkout' AND outcome = 'ok' AND binding_id IS NOT NULL
		AND result::text LIKE '%promo%'`), "the invocation records the replacement")

	// The independent guard applies to the replacement as well: 22% was fine
	// for the original; with the bonus the replacement asks for 27%.
	g := e.newOrder(user, "B-3", 10000)
	denied = e.checkout(user, g, 22)
	require.False(t, denied.OK)
	require.Equal(t, "validation", denied.Error.Kind)
	require.Equal(t, "e2e.limits.discountTooHigh", denied.Error.Issues[0].Code)
	require.Equal(t, checkoutOut{Cents: 10000, Status: "open"}, e.readOrder(user, g))

	// Generic writes to another package's entity: accepted by the compiler only
	// where suppressed, and refused by the host.
	sneak := e.rpc(user, "command", "e2e.promo.sneak", map[string]any{"order": g})
	require.False(t, sneak.OK)
	require.Contains(t, sneak.Error.Message, "read-only through generic data calls", "%+v", sneak.Error)
	require.Equal(t, 10000, e.readOrder(user, g).Cents, "the foreign row is untouched")
	// ... and for a native entity the check refuses it before anything is published.
	bad := loadTree(t, "promo").
		with("src/server.ts", "    // @ts-expect-error pricing.Order is another package's entity\n    await tx.update(pricing.Order, input.order, { cents: 1 })",
			"    await tx.insert(crm.Client, {} as never)").
		with("src/server.ts", `import * as pricing from "@iota-kit/e2e.pricing"`, "import * as pricing from \"@iota-kit/e2e.pricing\"\nimport * as crm from \"@iota-kit/crm\"")
	e.put(admin, "e2e.promo", bad)
	chk := e.check(admin, "e2e.promo")
	require.True(t, chk.HasErrors)
	var typeErr bool
	for _, d := range chk.Diagnostics {
		typeErr = typeErr || (d.File == "src/server.ts" && strings.HasPrefix(d.Code, "ts."))
	}
	require.True(t, typeErr, "generic write to crm.Client is a type error: %+v", chk.Diagnostics)
	e.put(admin, "e2e.promo", loadTree(t, "promo"))

	// Reverting the binding restores the previous behavior.
	e.unbind(admin, manage.SlotCommand, "e2e.pricing.checkout")
	h := e.newOrder(user, "C-1", 10000)
	res = e.checkout(user, h, 10)
	require.True(t, res.OK)
	require.NoError(t, jsonUnmarshal(res.Value, &out))
	require.Equal(t, checkoutOut{Cents: 9000, Status: "paid"}, out)
	require.Equal(t, "paid", e.readOrder(user, h).Status)
	denied = e.checkout(user, e.newOrder(user, "C-2", 10000), 30)
	require.Equal(t, "e2e.limits.discountTooHigh", denied.Error.Issues[0].Code, "the guard applies to the original too")

	// The binding history keeps both decisions.
	var hist []manage.Binding
	e.mgmt(admin, http.MethodGet, "/bindings?history=true", nil, http.StatusOK, &hist)
	var replaced []manage.Binding
	for _, b := range hist {
		if b.TargetID == "e2e.pricing.checkout" && b.SlotKind == manage.SlotCommand && b.Package == "e2e.promo" {
			replaced = append(replaced, b)
		}
	}
	require.Len(t, replaced, 1)
	require.NotNil(t, replaced[0].ActiveTo, "the revert closed the replacement binding")
}
