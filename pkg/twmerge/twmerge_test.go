package twmerge_test

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/twmerge"
)

// classes compares what a merge produced as a set. Upstream groups the
// survivors by class group and walks a map to do it, so "gap-1 w-0" comes back
// as "gap-1 w-0" from one call and "w-0 gap-1" from the next — within a single
// process, on the same input. Which classes survive is the contract; the order
// they are printed in is not, and asserting on it makes a test that fails a few
// runs in ten. (Harmless in HTML: a class attribute is a set.)
func classes(merged string) []string {
	fields := strings.Fields(merged)
	sort.Strings(fields)
	return fields
}

// TestMergeResolvesConflicts is the sanity floor: this package must merge the
// way the upstream one does, or the swap of a dozen component imports changed
// what those components render.
func TestMergeResolvesConflicts(t *testing.T) {
	t.Parallel()

	if got := twmerge.Merge("p-1 p-2"); got != "p-2" {
		t.Fatalf("Merge(%q) = %q, want %q", "p-1 p-2", got, "p-2")
	}
	if got := twmerge.Merge("px-2 py-1", "p-4"); got != "p-4" {
		t.Fatalf("Merge(%q, %q) = %q, want %q", "px-2 py-1", "p-4", got, "p-4")
	}
	if got, want := classes(twmerge.Merge("text-sm font-medium", "text-lg")),
		[]string{"font-medium", "text-lg"}; !equal(got, want) {
		t.Fatalf(`Merge("text-sm font-medium", "text-lg") = %v, want %v`, got, want)
	}
	if got := twmerge.Merge(""); got != "" {
		t.Fatalf("Merge(%q) = %q, want empty", "", got)
	}
}

// TestMergeIsGoroutineSafe is the reason this package exists. Every component
// merges classes while rendering, so this is exactly the concurrency an ordinary
// page under load produces.
//
// Two properties are being exercised, and both need the distinct class strings:
//
//   - every call misses the cache and therefore writes to it, which is where
//     upstream's two-mutex LRU corrupts its list and panics on a nil prev;
//   - the writes far exceed the cache capacity, so evictions run throughout
//     rather than only at the end.
//
// What would make this falsely green: reusing one class string across the
// goroutines. The first call would fill the cache and every later one would
// return from Get, so nothing would ever race on a write. Under -race the same
// body pointed at upstream's twmerge.Merge reports the data race; the panic mode
// it also catches shows up with or without the flag.
func TestMergeIsGoroutineSafe(t *testing.T) {
	t.Parallel()

	const goroutines = 32
	const perGoroutine = 250 // 8000 distinct keys against a 1000-entry cache

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func() {
			defer wg.Done()
			for i := range perGoroutine {
				// Unique per (goroutine, iteration), and a genuine conflict:
				// the second padding class must win.
				margin := fmt.Sprintf("m-%d-%d", g, i)
				got := classes(twmerge.Merge("p-1 "+margin, "p-4"))
				if want := classes(margin + " p-4"); !equal(got, want) {
					t.Errorf("Merge(%q, %q) = %v, want %v", "p-1 "+margin, "p-4", got, want)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestMergeStaysCorrectPastCacheCapacity pins the one behaviour the flush-when-
// full cache could plausibly get wrong: a key evicted by the flush must still
// merge to the same answer when it comes back, not to a stale or empty one.
//
// What would make this falsely green: asserting fewer keys than the cache holds,
// which never triggers a flush at all.
func TestMergeStaysCorrectPastCacheCapacity(t *testing.T) {
	t.Parallel()

	const keys = 2500 // more than the 1000-entry default capacity

	for i := range keys {
		in := fmt.Sprintf("gap-1 w-%d", i)
		if got, want := classes(twmerge.Merge(in)), classes(in); !equal(got, want) {
			t.Fatalf("first pass: Merge(%q) = %v, want %v", in, got, want)
		}
	}
	// Second pass over the same keys: the early ones were flushed by now, so
	// these answers are recomputed rather than read back.
	for i := range keys {
		in := fmt.Sprintf("gap-1 w-%d", i)
		if got, want := classes(twmerge.Merge(in)), classes(in); !equal(got, want) {
			t.Fatalf("after eviction: Merge(%q) = %v, want %v", in, got, want)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
