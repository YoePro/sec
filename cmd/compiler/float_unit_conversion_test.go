package main

import (
	"strings"
	"testing"
)

// TestFloatingUnitConversionLoweringGate prevents accepted Sema plans from
// silently disappearing in any output pipeline before unit operations exist.
// Rules: rules/corrections/applied/semantic-ir-units-correction-20260818.md — Erasure boundary.
func TestFloatingUnitConversionLoweringGate(t *testing.T) {
	file := "../../testdata/sema/float_unit_conversion/valid.sec"
	_, output, code := runCLIForDiagnostics(t, "sema", file, "--diagnostic-format=json")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, command := range []string{"emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
		stdout, output, code := runCLIForDiagnostics(t, command, file, "-o", "-", "--diagnostic-format=json")
		doc := decodeOccurrenceDocument(t, output)
		if code != 4 || doc.Summary.Errors != 1 || !strings.Contains(doc.Occurrences[0].Message.Text, "floating unit conversion") || stdout != "" {
			t.Fatal(command, code, stdout, output)
		}
		if doc.Occurrences[0].Arguments["category"] != "unsupported-lowering" {
			t.Fatal(doc)
		}
	}
}
