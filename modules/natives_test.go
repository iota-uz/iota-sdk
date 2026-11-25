package modules_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

// requiredEventLocales are the locales every SDK event is labeled and
// described in. Applications check their own locales in their own tests.
var requiredEventLocales = []string{"en", "ru", "uz", "uz-Cyrl"}

// eventCoverage names, for every SDK event, its Go declaration, the ID
// constant and the test of its publisher: the test exercises the mutation that
// publishes the event, asserts the event, and shows it committing with the
// change and rolling back with it (Test is "file_test.go:TestName", relative to
// this directory).
type eventCoverage struct{ Var, IDConst, Test string }

var eventCoverageByID = map[string]eventCoverage{
	"core.user.created.v1":                  {"UserCreatedEvent", "UserCreatedEventID", "core/services/user_events_test.go:TestUserService_Create_PublishesUserCreatedEventInTheCreatingTransaction"},
	"core.user.updated.v1":                  {"UserUpdatedEvent", "UserUpdatedEventID", "core/services/native_events_test.go:TestUserEvents_CommitAndRollBackWithTheChange"},
	"core.user.blocked.v1":                  {"UserBlockedEvent", "UserBlockedEventID", "core/services/native_events_test.go:TestUserEvents_CommitAndRollBackWithTheChange"},
	"core.user.unblocked.v1":                {"UserUnblockedEvent", "UserUnblockedEventID", "core/services/native_events_test.go:TestUserEvents_CommitAndRollBackWithTheChange"},
	"core.user.deleted.v1":                  {"UserDeletedEvent", "UserDeletedEventID", "core/services/native_events_test.go:TestUserEvents_CommitAndRollBackWithTheChange"},
	"core.user.role_assigned.v1":            {"UserRoleAssignedEvent", "UserRoleAssignedEventID", "core/services/native_events_test.go:TestUserEvents_CommitAndRollBackWithTheChange"},
	"core.role.created.v1":                  {"RoleCreatedEvent", "RoleCreatedEventID", "core/services/native_events_test.go:TestRoleEvents_CommitAndRollBackWithTheChange"},
	"core.role.permission_granted.v1":       {"RolePermissionGrantedEvent", "RolePermissionGrantedEventID", "core/services/native_events_test.go:TestRoleEvents_CommitAndRollBackWithTheChange"},
	"core.group.created.v1":                 {"GroupCreatedEvent", "GroupCreatedEventID", "core/services/native_events_test.go:TestGroupEvents_CommitAndRollBackWithTheChange"},
	"core.group.member_added.v1":            {"GroupMemberAddedEvent", "GroupMemberAddedEventID", "core/services/native_events_test.go:TestGroupEvents_CommitAndRollBackWithTheChange"},
	"core.group.member_removed.v1":          {"GroupMemberRemovedEvent", "GroupMemberRemovedEventID", "core/services/native_events_test.go:TestGroupEvents_CommitAndRollBackWithTheChange"},
	"crm.client.created.v1":                 {"ClientCreatedEvent", "ClientCreatedEventID", "crm/services/client_events_test.go:TestClientEvents_CommitAndRollBackWithTheClient"},
	"crm.client.updated.v1":                 {"ClientUpdatedEvent", "ClientUpdatedEventID", "crm/services/client_events_test.go:TestClientEvents_CommitAndRollBackWithTheClient"},
	"crm.client.deleted.v1":                 {"ClientDeletedEvent", "ClientDeletedEventID", "crm/services/native_events_test.go:TestClientServiceEvents_CommitAndRollBackWithTheClient"},
	"crm.chat.message_added.v1":             {"ChatMessageAddedEvent", "ChatMessageAddedEventID", "crm/services/native_events_test.go:TestChatEvents_MessageAddedCommitsAndRollsBackWithTheMessage"},
	"hrm.employee.created.v1":               {"EmployeeCreatedEvent", "EmployeeCreatedEventID", "hrm/services/native_events_test.go:TestEmployeeEvents_CommitAndRollBackWithTheEmployee"},
	"hrm.employee.updated.v1":               {"EmployeeUpdatedEvent", "EmployeeUpdatedEventID", "hrm/services/native_events_test.go:TestEmployeeEvents_CommitAndRollBackWithTheEmployee"},
	"finance.payment.created.v1":            {"PaymentCreatedEvent", "PaymentCreatedEventID", "finance/services/events_test.go:TestPaymentEvents_CommitAndRollBackWithThePayment"},
	"finance.payment.updated.v1":            {"PaymentUpdatedEvent", "PaymentUpdatedEventID", "finance/services/events_test.go:TestPaymentEvents_CommitAndRollBackWithThePayment"},
	"finance.expense.created.v1":            {"ExpenseCreatedEvent", "ExpenseCreatedEventID", "finance/services/events_test.go:TestExpenseEvents_CommitAndRollBackWithTheExpense"},
	"finance.debt.created.v1":               {"DebtCreatedEvent", "DebtCreatedEventID", "finance/services/events_test.go:TestDebtEvents_CommitAndRollBackWithTheDebt"},
	"finance.debt.settled.v1":               {"DebtSettledEvent", "DebtSettledEventID", "finance/services/events_test.go:TestDebtEvents_CommitAndRollBackWithTheDebt"},
	"finance.debt.written_off.v1":           {"DebtWrittenOffEvent", "DebtWrittenOffEventID", "finance/services/events_test.go:TestDebtEvents_CommitAndRollBackWithTheDebt"},
	"finance.debt.cancelled.v1":             {"DebtCancelledEvent", "DebtCancelledEventID", "finance/services/events_test.go:TestDebtEvents_CommitAndRollBackWithTheDebt"},
	"billing.transaction.status_changed.v1": {"TransactionStatusChangedEvent", "TransactionStatusChangedEventID", "billing/services/events_test.go:TestBillingEvents_StatusChangeCommitsAndRollsBackWithTheTransaction"},
	"warehouse.order.created.v1":            {"OrderCreatedEvent", "OrderCreatedEventID", "warehouse/services/orderservice/events_test.go:TestOrderEvents_CommitAndRollBackWithTheOrder"},
	"warehouse.order.completed.v1":          {"OrderCompletedEvent", "OrderCompletedEventID", "warehouse/services/orderservice/events_test.go:TestOrderEvents_CommitAndRollBackWithTheOrder"},
	"projects.acceptance.signed.v1":         {"AcceptanceSignedEvent", "AcceptanceSignedEventID", "projects/services/acceptance_events_test.go:TestAcceptanceEvents_SignedCommitsAndRollsBackWithTheDocument"},
	// The automation engine publishes it in the transaction of the task change (pkg/automation/engine).
	"automation.task.assigned.v1": {"TaskAssigned", "EventTaskAssigned", "../pkg/automation/engine/human_test.go:TestTaskAssignedEvents"},
}

// publisherRoots are the source trees searched for publishers, relative to this directory: the
// modules, and the automation engine, which publishes the events of human tasks.
var publisherRoots = []string{".", "../pkg/automation/engine"}

// contractHashes pins the contracts that existed before the events of #1332:
// their payloads did not change, so their hashes must not.
var contractHashes = map[string]string{
	"core.user.created.v1":  "e3d9470226f2757ef9269af0cfa055a77c89fcc50352549de67ee3d227f7e569",
	"crm.client.created.v1": "63b4839536ba51abf135cd79186d77ddfb8cd63a6c417639441282cb69b550e4",
	"crm.client.updated.v1": "b919391660de57675d516e31d65e527f8ad018c02576bbf2c222ce64a29f8c46",
}

func TestNativeEventContractsAreUnchanged(t *testing.T) {
	set, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	for id, want := range contractHashes {
		ev, ok := set.Event(id)
		require.True(t, ok, id)
		require.Equal(t, want, ev.Resource().ContractHash, "%s: a payload change is a new event ID", id)
	}
}

// Every subject of an event is a native entity of the set, with a read
// permission, so the palette and the catalog can resolve it.
func TestNativeEventSubjectsAreEntities(t *testing.T) {
	set, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	for _, ev := range set.Events() {
		res := ev.Resource()
		require.NotEmpty(t, res.Subject, ev.ID())
		_, ok := set.EntityRegistry().Get(res.Subject)
		require.True(t, ok, "%s: subject %s is a native entity", ev.ID(), res.Subject)
		meta, ok := ev.(native.EventMetadata)
		require.True(t, ok)
		require.NotEmpty(t, meta.ReadPermission(), ev.ID())
		require.NotEmpty(t, meta.Category(), ev.ID())
	}
}

var publisherTestRef = regexp.MustCompile(`^([^:]+_test\.go):(Test\w+)$`)

// Every event of modules.Natives() follows the shared key grammar, is labeled
// and described in every required locale and has a publisher with a
// commit/rollback test.
func TestNativeEventsCoverage(t *testing.T) {
	set, err := native.Compose(modules.Natives()...)
	require.NoError(t, err)
	evs := set.Events()
	require.NotEmpty(t, evs)
	published := publishedEventVars(t)
	for _, ev := range evs {
		t.Run(ev.ID(), func(t *testing.T) {
			require.True(t, native.ValidEventKey(ev.ID()), "the key follows %s", native.EventKeyPattern)
			meta, ok := ev.(native.EventMetadata)
			require.True(t, ok, "a native event carries its metadata")
			for _, locale := range requiredEventLocales {
				require.NotEmpty(t, meta.Label()[locale], "Label[%s]", locale)
				require.NotEmpty(t, meta.Descriptions()[locale], "Descriptions[%s]", locale)
			}
			cov, ok := eventCoverageByID[ev.ID()]
			require.True(t, ok, "name the declaration and the publisher test of the event in eventCoverageByID")
			require.True(t, published[cov.Var], "a service passes %s to events.Publish/PublishInTx", cov.Var)
			m := publisherTestRef.FindStringSubmatch(cov.Test)
			require.NotNil(t, m, "eventCoverageByID entry %q is file_test.go:TestName", cov.Test)
			src, err := os.ReadFile(filepath.FromSlash(m[1]))
			require.NoError(t, err)
			body := testBody(t, string(src), m[2])
			require.True(t, strings.Contains(body, cov.IDConst) || strings.Contains(body, `"`+ev.ID()+`"`),
				"%s asserts the event: it names %s", m[2], cov.IDConst)
		})
	}
	for id := range eventCoverageByID {
		_, ok := set.Event(id)
		require.True(t, ok, "eventCoverageByID names an event that is no longer declared: %s", id)
	}
}

// publishedEventVars are the names of the event variables that reach the event
// argument of a real events.Publish / events.PublishInTx call in the non-test
// sources of the modules, directly or through a local variable assigned from
// them. A name that is only declared, listed in Natives or assigned to a local
// nothing publishes is not in the set.
func publishedEventVars(t *testing.T) map[string]bool {
	t.Helper()
	published := map[string]bool{}
	fset := token.NewFileSet()
	for _, root := range publisherRoots {
		require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					// events.PublishInTx(ctx, tenant, ev, ...), events.Publish(ctx, tx, scope, ev, ...)
					arg, isPublish := map[string]int{"PublishInTx": 2, "Publish": 3}[sel.Sel.Name]
					pkg, isEvents := sel.X.(*ast.Ident)
					if !isPublish || !isEvents || pkg.Name != "events" || len(call.Args) <= arg {
						return true
					}
					for name := range resolveEventNames(fn.Body, call.Args[arg]) {
						published[name] = true
					}
					return true
				})
			}
			return nil
		}))
	}
	return published
}

// resolveEventNames are the identifiers expr can be: itself, or those the
// local it names is assigned from in body.
func resolveEventNames(body *ast.BlockStmt, expr ast.Expr) map[string]bool {
	out := map[string]bool{}
	var visit func(name string, seen map[string]bool)
	visit = func(name string, seen map[string]bool) {
		if seen[name] {
			return
		}
		seen[name] = true
		out[name] = true
		ast.Inspect(body, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok || len(assign.Lhs) != len(assign.Rhs) {
				return true
			}
			for i, lhs := range assign.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
					if rhs, ok := assign.Rhs[i].(*ast.Ident); ok {
						visit(rhs.Name, seen)
					}
				}
			}
			return true
		})
	}
	if id, ok := expr.(*ast.Ident); ok {
		visit(id.Name, map[string]bool{})
	}
	return out
}

// testBody is the source of the top-level test function name.
func testBody(t *testing.T, src, name string) string {
	t.Helper()
	start := regexp.MustCompile(`(?m)^func ` + name + `\(t \*testing\.T\) \{`).FindStringIndex(src)
	require.NotNil(t, start, "the publisher test %s exists", name)
	rest := src[start[0]:]
	if end := strings.Index(rest[1:], "\nfunc "); end >= 0 {
		return rest[:end+1]
	}
	return rest
}
