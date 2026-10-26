package main

import (
	"os"
	"strings"
	"testing"
)

// package20.md §§ 48–54, 71: checked integer conversions under try, with the
// core ConversionError, emit verified Sec MLIR: numeric.convert_checked, a
// branch on its failed flag and ConversionError.OutOfRange on the failure edge.
func TestCheckedIntegerConversionEmitsVerifiedSecMLIR(t *testing.T) {
	if os.Getenv("SEC_MLIR_BIN") == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-sec-mlir", "../../testdata/semantic_ir/checked_integer_conversion.sec")
	if code != 0 {
		t.Fatalf("emit-sec-mlir: %d\n%s", code, stderr)
	}
	if strings.Count(stdout, `"sec.numeric.convert_checked"`) != 3 || !strings.Contains(stdout, `failure = "OutOfRange"`) {
		t.Fatalf("missing checked conversions in:\n%s", stdout)
	}
}
