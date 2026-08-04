package sql

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeTx captures Exec invocations so we can assert SetTenantContext
// issues the right SQL and skips when tenantID is uuid.Nil.
type fakeTx struct {
	pgx.Tx
	execs []fakeExec
	err   error
}

type fakeExec struct {
	sql  string
	args []any
}

func (f *fakeTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execs = append(f.execs, fakeExec{sql: sql, args: args})
	return pgconn.CommandTag{}, f.err
}

func TestSetTenantContext_EmitsSetConfig(t *testing.T) {
	tx := &fakeTx{}
	tenantID := uuid.New()

	if err := SetTenantContext(context.Background(), tx, tenantID); err != nil {
		t.Fatalf("SetTenantContext: %v", err)
	}
	if len(tx.execs) != 1 {
		t.Fatalf("expected 1 exec, got %d", len(tx.execs))
	}
	if tx.execs[0].sql != setConfigTenantSQL {
		t.Fatalf("wrong sql: %q", tx.execs[0].sql)
	}
	if len(tx.execs[0].args) != 1 || tx.execs[0].args[0] != tenantID.String() {
		t.Fatalf("wrong args: %v", tx.execs[0].args)
	}
}

func TestSetTenantContext_NilTenantIsNoop(t *testing.T) {
	tx := &fakeTx{}
	if err := SetTenantContext(context.Background(), tx, uuid.Nil); err != nil {
		t.Fatalf("SetTenantContext: %v", err)
	}
	if len(tx.execs) != 0 {
		t.Fatalf("expected no exec, got %d", len(tx.execs))
	}
}

func TestSetTenantContext_NilTxErrors(t *testing.T) {
	if err := SetTenantContext(context.Background(), nil, uuid.New()); err == nil {
		t.Fatal("expected error for nil tx")
	}
}

func TestSetTenantContext_WrapsExecError(t *testing.T) {
	boom := errors.New("boom")
	tx := &fakeTx{err: boom}
	err := SetTenantContext(context.Background(), tx, uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
}

func TestNoTenantResolver(t *testing.T) {
	id, err := NoTenantResolver(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if id != uuid.Nil {
		t.Fatalf("expected uuid.Nil, got %v", id)
	}
}
