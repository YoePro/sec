package secmlir

import (
	"strconv"
	"strings"
	"testing"
)

// MD-076 call-lifetime borrows of fresh temporaries and MD-063 whole-referent
// dereference lower to schema-11 Sec MLIR with the existing Place/reference
// vocabulary on 32- and 64-bit plans, and pass the three schema-11 verifiers
// when SEC_MLIR_BIN names the built tools.
//
// Rules:
//   - rules/memory/borrowing.md — § 5.4 (MD-076)
//   - rules/memory/references.md — § 3.4 (MD-063)
func TestTemporaryBorrowsAndDereferenceEmitSchema11(t *testing.T) {
	source := `module main

type Counter struct {
    Count: int,
}

impl Counter {
    fn GetCount() int {
        return self.Count
    }

    fn Increment() void {
        self.Count = self.Count + 1
    }
}

fn CreateCounter() Counter {
    return Counter{Count: 1}
}

fn Read(value: ref Counter) int {
    return value.Count
}

fn Bump(value: ref mut Counter) void {
    value.Count += 1
}

fn Field(value: ref Counter) ref int {
    return ref value.Count
}

fn Increment(value: ref mut int) void {
    *value += 1
}

fn main() int {
    let a := Read(CreateCounter())
    Bump(CreateCounter())
    let b := CreateCounter().GetCount()
    CreateCounter().Increment()
    let c := *Field(CreateCounter())
    let mut n := a + b + c
    let r := ref mut n
    *r = *r + 1
    Increment(n)
    return n - 5
}
`
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			module := buildPackage15EmitModule(t, "temporaries", source)
			output, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatalf("emit: %v", err)
			}
			text := string(output)
			for _, want := range []string{`"sec.place.value"(`, `"sec.ref.borrow_shared"(`, `"sec.ref.borrow_mut"(`, `sec.address_taken = true`, `"sec.place.deref"(`, `"sec.place.read"(`, `"sec.place.write"(`} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q in:\n%s", want, text)
				}
			}
			verifyPackage15MLIR(t, "temporaries", output)
		})
	}
}
