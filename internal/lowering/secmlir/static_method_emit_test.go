package secmlir

import (
	"strings"
	"testing"
)

// Static methods reach Sec MLIR as ordinary functions with their qualified
// Target.Name symbol and direct calls.
//
// Rules:
//   - rules/declarations/impl.md — § 13 "Static members"
//   - rules/compiler/semantic_ir.md — §§ 60, 61
func TestStaticMethodsEmitAsQualifiedFunctions(t *testing.T) {
	plan := testPlan(64)
	module := scalarParityBuild(t, `module main

type Counter struct { value: int }

impl Counter {
    static fn Maximum() int {
        return 100
    }
}

fn Use() int {
    return Counter.Maximum()
}
`, plan)
	output, err := Emit(module, plan)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "Counter.Maximum") || strings.Count(text, "Counter.Maximum") < 2 {
		t.Fatalf("emitted module lacks the static method definition and call:\n%s", text)
	}
}
