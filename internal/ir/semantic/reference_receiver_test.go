package semantic

import (
	"errors"
	"strings"
	"testing"
)

// From package 15 an instance method receives `self` as an explicit ref or
// ref mut parameter carrying Sema's receiver facts; implicit and `self.`
// field reads and writes dereference it; a call passes an implicit borrow of
// an owned receiver, a copied shared reference, or the caller's own `self`
// (P15-47).
//
// Rules:
//   - rules/declarations/impl.md — §§ 8, 11
//   - rules/compiler/semantic_ir.md — §§ 60(3), 61(4)
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 59-60
func TestPackage15LowersInstanceMethodReceivers(t *testing.T) {
	source := `module main

type Counter struct { value: int, limit: int }

impl Counter {
    fn Value() int {
        return value + self.limit
    }
    fn Reset() void {
        self.value = 0
    }
    fn Twice() int {
        return self.Value() + Value()
    }
}

fn Use() int {
    let mut counter := Counter { value: 1, limit: 2 }
    counter.Reset()
    let v := counter.Value()
    let r := ref counter
    let w := r.Value()
    return v + w
}
`
	module, err := analyzedModule(t, source, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{
		`Counter.Value(%0 self: !3 [shared-reference] {`, `proof=caller-owned-borrow space=ordinary stability=Stable provenance=receiver}`,
		`Counter.Reset(%0 self: !9 [mutable-reference]`,
		"place.field %1 field=#0", "place.field %4 field=#1", "place.write %2, %3",
		`call.direct "main::Counter.Reset()"(%4[mutable-reference])`,
		`call.direct "main::Counter.Value()"(%6[shared-reference])`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("module lacks %q:\n%s", want, text)
		}
	}
	twice := functionNamed(t, module, "Counter.Twice")
	copies := 0
	for _, block := range twice.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpReferenceCopyShared {
				copies++
			}
		}
	}
	if copies != 2 {
		t.Errorf("both nested calls in Twice must pass a copy of self, got %d:\n%s", copies, text)
	}

	// Package 14 has no references, so instance methods stay a boundary.
	_, err = analyzedModule(t, source, 14)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Feature != "instance method call" {
		t.Fatalf("package 14 err = %v, want the instance method boundary", err)
	}
}

// A write to an implicit field makes Sema derive a mutable receiver (impl.md
// § 8); the write lowers through the ref mut self parameter.
func TestPackage15ImplicitFieldWriteUsesMutableReceiver(t *testing.T) {
	module, err := analyzedModule(t, `module main

type Counter struct { value: int }

impl Counter {
    fn Bump() void {
        value = 2
    }
}
`, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	if !strings.Contains(text, "self: !") || !strings.Contains(text, "[mutable-reference]") || !strings.Contains(text, "place.write") {
		t.Fatalf("implicit field write did not lower through ref mut self:\n%s", text)
	}
}
