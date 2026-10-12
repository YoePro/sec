package secmlir

import (
	"errors"
	"testing"
)

// Sec MLIR handler metadata knows only ok, err-variant, err-catch-all and
// merge, so Semantic IR None handlers of an Option try are rejected
// explicitly instead of being emitted as Err handlers.
//
// Rules:
//   - rules/mlir/dialect-versions/sec_mlir_dialect_v6.md — §12 "Handler provenance metadata"
//   - rules/errors/errorhandling.md — §35
func TestOptionTryNoneHandlersAreAnEmissionBoundary(t *testing.T) {
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
`, plan)
	output, err := Emit(module, plan)
	var unsupported *UnsupportedLoweringError
	if output != nil || !errors.As(err, &unsupported) || unsupported.Feature != "Option try None handlers" {
		t.Fatalf("emit = %q, %v; want the Option None handler boundary", output, err)
	}
}
