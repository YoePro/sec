package main

import (
	"os"
	"strings"
	"testing"
)

// flowcontrol_match.md § 10 and the Sec MLIR match-CFG verifier: named-field
// patterns emit verified Sec MLIR reading only bound fields, and a guarded
// arm whose body adds blocks (checked arithmetic) keeps arm provenance in
// source order.
func TestMatchFieldPatternsAndGuardedArithmeticEmitVerifiedSecMLIR(t *testing.T) {
	if os.Getenv("SEC_MLIR_BIN") == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	file := "../../testdata/semantic_ir/match_field_patterns.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", file)
	if code != 0 {
		t.Fatalf("emit-sec-mlir: %d\n%s", code, stderr)
	}
	for _, want := range []string{`field = "Width"`, `field = "Height"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %s in:\n%s", want, stdout)
		}
	}
	area, _, _ := strings.Cut(stdout, "func.func @sec_fn_1")
	if strings.Contains(area, `field = "Label"`) {
		t.Fatalf("omitted field Label must not be read:\n%s", stdout)
	}
}
