package secmlir

import (
	"strings"
	"testing"
)

// Option try None handlers, a guarded None handler and the residual None
// propagation of a partial list emit with their MD-031 handler provenance
// instead of being disguised as Err handlers.
//
// Rules:
//   - rules/mlir/sec_mlir_dialect.md — v11.41.6 "Guarded, partial and Option try handlers"
//   - rules/errors/errorhandling.md — §§ 16, 19, 35
func TestOptionTryNoneHandlersEmitProvenance(t *testing.T) {
	plan := testPlan(64)
	module := scalarParityBuild(t, `module main

fn Find(flag: bool) Option[int] {
    if flag {
        return Some(4)
    }
    return None
}

fn Value(flag: bool) int {
    let value := try Find(flag) {
        None => 0
    }
    return value
}

fn Fallback(flag: bool, fallback: bool) Option[int] {
    let value := try Find(flag) {
        None where fallback => 7
    }
    return Some(value)
}
`, plan)
	output, err := Emit(module, plan)
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, want := range []string{
		`sec.try_handler_kind = "none", sec.try_handler_index = 0 : i32, sec.try_handler_exhaustive = true`,
		"sec.try_handler_guarded = true",
		"sec.try_handler_residual = true",
		`sec.try_handler_kind = "residual", sec.try_handler_index = -1 : i32`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}
