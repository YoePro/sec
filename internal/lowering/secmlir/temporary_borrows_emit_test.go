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
			for _, want := range []string{`"sec.place.value"(`, `"sec.ref.borrow_shared"(`, `"sec.ref.borrow_mut"(`, `sec.address_taken = true, sec.synthesized = true`, `sec.source_name = ""`, `"sec.place.deref"(`, `"sec.place.read"(`, `"sec.place.write"(`} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q in:\n%s", want, text)
				}
			}
			verifyPackage15MLIR(t, "temporaries", output)
		})
	}
}

// MD076-P10 through schema 11: temporaries reached only under a short-circuit
// or branch verify with the Place, reference-guard and borrow-semantics
// verifiers on 32- and 64-bit plans.
func TestBranchScopedTemporariesEmitSchema11(t *testing.T) {
	source := `module main

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

fn main() int {
    return 0
}
`
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			output, err := Emit(buildPackage15EmitModule(t, "branch_temporaries", source), testPlan(width))
			if err != nil {
				t.Fatalf("emit: %v", err)
			}
			if !strings.Contains(string(output), "sec.synthesized = true") {
				t.Fatalf("mutable temporary storage is not marked synthesized:\n%s", output)
			}
			verifyPackage15MLIR(t, "branch_temporaries", output)
		})
	}
}

// MD-065: borrow ends at Sema's last required use (a let holder before the
// conflicting write, a payload borrow mid-arm) verify with the schema-11
// borrow-semantics verifier on 32- and 64-bit plans.
func TestLastUseBorrowEndsEmitSchema11(t *testing.T) {
	source := `module main

type Point struct {
    x: int,
}

fn Holder() int {
    let mut x := 1
    let r := ref x
    let a := *r
    x = 2
    return x + a
}

fn Payload(value: Option[Point], early: bool) int {
    match value {
        Some(ref point) => {
            if early {
                return 1
            }
            let copy := point.x
            return copy * 2
        }
        None => {
            return 0
        }
    }
}

fn main() int {
    return Holder() - 3
}
`
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			output, err := Emit(buildPackage15EmitModule(t, "borrow_ends", source), testPlan(width))
			if err != nil {
				t.Fatalf("emit: %v", err)
			}
			if strings.Count(string(output), `"sec.ref.end_borrow"(`) < 3 {
				t.Fatalf("expected Sema-placed and arm-exit end_borrow markers:\n%s", output)
			}
			verifyPackage15MLIR(t, "borrow_ends", output)
		})
	}
}
