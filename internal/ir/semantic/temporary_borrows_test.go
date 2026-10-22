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
	// Compiler-materialized storages carry no fabricated source name.
	if got := strings.Count(main, " : !2 mutable=true class=local-automatic address-taken"); got != 2 {
		t.Fatalf("expected 2 unnamed address-taken temporary storages for mutable borrows, got %d:\n%s", got, main)
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

// MD076-P10: under a short-circuit or a branch, a borrowed temporary is
// produced, materialized and borrowed only on the path that reaches its call;
// a mutable temporary's compiler storage is declared there too, never on the
// entry path.
func TestTemporaryBorrowsExistOnlyOnReachedPaths(t *testing.T) {
	source := `
module main

type Counter struct {
    Count: int,
}

fn CreateCounter() Counter {
    return Counter{Count: 1}
}

fn Read(value: ref Counter) int {
    return value.Count
}

fn Bump(value: ref mut Counter) int {
    value.Count += 1
    return value.Count
}

fn Pick(flag: bool) bool {
    return flag && Read(CreateCounter()) > 0
}

fn Either(flag: bool) bool {
    return flag || Bump(CreateCounter()) > 1
}

fn Choose(flag: bool) int {
    if flag {
        return Read(CreateCounter())
    }
    return 0
}

fn main() int {
    return 0
}
`
	module, err := buildPackage15Source(t, source, "")
	if err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, name := range []string{"Pick(bool)", "Either(bool)", "Choose(bool)"} {
		start := strings.Index(text, `func "main::`+name+`"`)
		if start < 0 {
			t.Fatalf("missing %s in:\n%s", name, text)
		}
		body := text[start:]
		body = body[:strings.Index(body, "\n  }")]
		entry := body[strings.Index(body, "^0:"):strings.Index(body, "^1:")]
		reached := body[strings.Index(body, "^1:"):]
		for _, operation := range []string{`call.direct "main::CreateCounter()"`, "ref.borrow-", "storage.declare", "place.value-root", "place.storage-root"} {
			if strings.Contains(entry, operation) {
				t.Fatalf("%s materializes its temporary (%s) before the branch:\n%s", name, operation, body)
			}
		}
		if strings.Count(reached, `call.direct "main::CreateCounter()"`) != 1 || !strings.Contains(reached, "ref.borrow-") {
			t.Fatalf("%s does not produce and borrow its temporary exactly once on the reached path:\n%s", name, body)
		}
	}
	either := text[strings.Index(text, `func "main::Either(bool)"`):]
	either = either[:strings.Index(either, "\n  }")]
	if !strings.Contains(either, "storage.declare") || !strings.Contains(either, "ref.borrow-mut") {
		t.Fatalf("Either must borrow its mutable temporary from compiler storage:\n%s", either)
	}
}
