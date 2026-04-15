package db

import (
	"reflect"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// stubPool returns a zero-valued *pgxpool.Pool. We never dial or use it;
// the registry only stores the pointer.
func stubPool() *pgxpool.Pool {
	return &pgxpool.Pool{}
}

func TestPoolRegistry_RegisterAndGet(t *testing.T) {
	r := NewPoolRegistry()
	p := stubPool()

	if err := r.Register("default", p); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := r.Get("default")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != p {
		t.Fatalf("Get returned different pool: %v vs %v", got, p)
	}
}

func TestPoolRegistry_RegisterRejectsEmptyName(t *testing.T) {
	r := NewPoolRegistry()
	if err := r.Register("", stubPool()); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestPoolRegistry_RegisterRejectsNilPool(t *testing.T) {
	r := NewPoolRegistry()
	if err := r.Register("default", nil); err == nil {
		t.Fatal("expected error for nil pool")
	}
}

func TestPoolRegistry_RegisterRejectsDuplicate(t *testing.T) {
	r := NewPoolRegistry()
	if err := r.Register("default", stubPool()); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := r.Register("default", stubPool()); err == nil {
		t.Fatal("expected error on duplicate Register")
	}
}

func TestPoolRegistry_GetMissing(t *testing.T) {
	r := NewPoolRegistry()
	if _, err := r.Get("missing"); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestPoolRegistry_UnregisterLetsReRegister(t *testing.T) {
	r := NewPoolRegistry()
	_ = r.Register("default", stubPool())

	if !r.Unregister("default") {
		t.Fatal("Unregister returned false for existing name")
	}
	if r.Unregister("default") {
		t.Fatal("Unregister returned true for already-removed name")
	}
	if err := r.Register("default", stubPool()); err != nil {
		t.Fatalf("re-Register after Unregister: %v", err)
	}
}

func TestPoolRegistry_HasAndNames(t *testing.T) {
	r := NewPoolRegistry()
	_ = r.Register("zeta", stubPool())
	_ = r.Register("alpha", stubPool())
	_ = r.Register("mu", stubPool())

	if !r.Has("alpha") || r.Has("missing") {
		t.Fatal("Has returned wrong result")
	}

	want := []string{"alpha", "mu", "zeta"}
	got := r.Names()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names mismatch: want %v, got %v", want, got)
	}
}

func TestPoolRegistry_ConcurrentAccess(t *testing.T) {
	r := NewPoolRegistry()
	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			name := "p" + string(rune('a'+i%26))
			_ = r.Register(name, stubPool()) // ignore duplicate errors
			_, _ = r.Get(name)
			_ = r.Has(name)
			_ = r.Names()
		}(i)
	}
	wg.Wait()
}
