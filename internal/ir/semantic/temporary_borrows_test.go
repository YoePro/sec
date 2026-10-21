package semantic

import (
	"strings"
	"testing"
)

// MD-076: a fresh owned argument or receiver is evaluated once and borrowed
// from its own compiler-materialized Place — a read-only value root for a
// shared borrow, an address-taken compiler storage for a mutable one — and
// each temporary has its own root and storage identity. A reference returned
// through the borrow names that storage and is read with ref.deref-place.
func TestTemporaryBorrowsMaterializeOwnPlaces(t *testing.T) {
	source := `
module main

type Counter struct {
    Count: int,
}

impl Counter {
    fn Increment() void {
        self.Count = self.Count + 1
    }
}

fn CreateCounter() Counter {
    return Counter{Count: 1}
}

fn Add(a: ref Counter, b: ref Counter) int {
    return a.Count + b.Count
}

fn Bump(value: ref mut Counter) void {
    value.Count += 1
}

fn Field(value: ref Counter) ref int {
    return ref value.Count
}

fn main() int {
    let sum := Add(CreateCounter(), CreateCounter())
    Bump(CreateCounter())
    CreateCounter().Increment()
    return sum + *Field(CreateCounter())
}
`
	module, err := buildPackage15Source(t, source, "")
	if err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	main := text[strings.Index(text, `func "main::main()"`):]
	if got := strings.Count(main, `call.direct "main::CreateCounter()"`); got != 5 {
		t.Fatalf("each temporary must be produced exactly once, got %d producer calls:\n%s", got, main)
	}
	if got := strings.Count(main, "place.value-root"); got != 3 {
		t.Fatalf("expected 3 shared temporary value roots, got %d:\n%s", got, main)
	}
	if got := strings.Count(main, "temporary: "); got != 2 || strings.Count(main, "address-taken") != 2 {
		t.Fatalf("expected 2 address-taken temporary storages for mutable borrows:\n%s", main)
	}
	for _, want := range []string{"ref.borrow-mut", `call.direct "main::Counter.Increment()"`, "ref.deref-place", "origins=[#"} {
		if !strings.Contains(main, want) {
			t.Fatalf("missing %q in:\n%s", want, main)
		}
	}
	// Left-to-right: the first Add argument's root precedes the second's.
	first := strings.Index(main, "root=#")
	second := strings.Index(main[first+1:], "root=#")
	if first < 0 || second < 0 || main[first:first+8] == main[first+1+second:first+1+second+8] {
		t.Fatalf("Add temporaries must have distinct Place roots:\n%s", main)
	}
}
