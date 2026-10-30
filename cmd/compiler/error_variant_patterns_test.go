package main

import (
	"os"
	"strings"
	"testing"
)

// MD-008/MD-029: Err(E.Variant...) match arms and try handlers on closed
// concrete error channels emit verified Sec MLIR with the result-err-variant
// pattern kind, guarded union payload/field projections and the guarded last
// try handler's exhaustive-try-fallthrough (sec_mlir_dialect.md v11.41.4).
func TestErrorVariantPatternsEmitVerifiedSecMLIR(t *testing.T) {
	if os.Getenv("SEC_MLIR_BIN") == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", "../../testdata/semantic_ir/error_variant_patterns.sec")
	if code != 0 {
		t.Fatalf("emit-sec-mlir: %d\n%s", code, stderr)
	}
	for _, want := range []string{`sec.match_pattern_kind = "result-err-variant"`, `"sec.result.unwrap_err"`, `"sec.union.unwrap_payload"`, `"sec.union.unwrap_field"`, `"sec.enum.cmp"`, `reason = "exhaustive-try-fallthrough"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %s in:\n%s", want, stdout)
		}
	}
}
