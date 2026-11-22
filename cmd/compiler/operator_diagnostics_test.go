package main

import (
	"strings"
	"testing"
)

// TestOperatorOperandDiagnosticsCLI checks public JSON diagnostic identity and
// discoverability through the canonical detailed catalog lookup.
// Rules: rules/tooling/diagnostics.md — §§12,18–19,26,30.
func TestOperatorOperandDiagnosticsCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "sema", "../../testdata/sema/operator_operand_types_invalid.sec", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, output)
	if code != 3 || len(doc.Occurrences) != 4 || strings.Count(output, `"id": "S1259"`) != 4 || strings.Contains(output, `"unregistered": true`) || !strings.Contains(output, "cannot apply operator - to uint and int") {
		t.Fatal(code, output)
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "diagnostics", "S1259")
	if code != 0 || !strings.Contains(stdout+stderr, "operator.incompatible-operand-types") {
		t.Fatal(code, stdout, stderr)
	}
}
