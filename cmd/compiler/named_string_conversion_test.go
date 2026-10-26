package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// MD-093: every direct numeric-to-nominal-string relation is rejected before
// code generation, with the same registered ID, position and helpful repair.
// Both explicit materialization forms remain valid and contracts keep their
// independent error coverage.
// Rules: rules/types/types.md — "Numeric-to-text conversion";
// rules/errors/errorhandling.md — §§ 11–14; rules/types/contracts.md — "Conversion failure layers".
func TestNamedStringConversionCLI(t *testing.T) {
	const base = "../../testdata/sema/"
	for _, command := range []string{"check", "sema", "emit-ir"} {
		_, stderr, code := runCLIForDiagnostics(t, command, "--diagnostic-format=json", base+"named_string_numeric_invalid.sec")
		document := decodeOccurrenceDocument(t, stderr)
		if code == 0 || document.Summary.Errors != 12 {
			t.Fatalf("%s accepted a numeric-to-named-string conversion: %d %s", command, code, stderr)
		}
		for _, occurrence := range document.Occurrences {
			if occurrence.ID == nil || *occurrence.ID != diagnostics.ConversionUnavailable || occurrence.Primary == nil || occurrence.Primary.Span.Start.Line == 0 {
				t.Fatalf("unregistered/unlocated conversion error: %+v", occurrence)
			}
		}
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", base+"named_string_numeric_invalid.sec")
	if code == 0 || !strings.Contains(stderr, "try Name(number.ToString())") {
		t.Fatalf("missing materialization guidance: %d %s", code, stderr)
	}
	_, stderr, code = runCLIForDiagnostics(t, "check", base+"named_string_numeric_valid.sec")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	for file, id := range map[string]string{
		"named_string_inner_contract_invalid.sec": diagnostics.CheckedConversionRequiresTry,
		"named_string_error_channel_invalid.sec":  diagnostics.TryPropagationIncompatible,
	} {
		_, stderr, code := runCLIForDiagnostics(t, "check", base+file)
		if code == 0 || !strings.Contains(stderr, id) || !strings.Contains(stderr, "ConversionError") {
			t.Fatalf("lost separate contract failure for %s: %d %s", file, code, stderr)
		}
	}
}

// The actual core declarations supply the failure types. Sema must preserve
// the materialization carrier before the independent nominal contract check.
// Rules: rules/errors/errorhandling.md — §§ 11–14; rules/types/contracts.md — "Public conversion and contract errors".
func TestNamedStringConversionFailureFacts(t *testing.T) {
	const file = "../../testdata/sema/named_string_numeric_valid.sec"
	input, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(string(input), file, hostCompilerTarget(), false)
	seen := map[string]bool{}
	for _, statement := range analyzed.Program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok || fn.Name == nil || (fn.Name.Value != "Outer" && fn.Name.Value != "Constrained") {
			continue
		}
		var expression *ast.TryExpression
		if fn.Name.Value == "Outer" {
			expression = fn.Body.Statements[0].(*ast.LetStatement).Value.(*ast.TryExpression)
		} else {
			expression = fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.OkExpression).Value.(*ast.TryExpression)
		}
		resolved, ok := analyzed.Analyzer.ResolvedTryOf(expression)
		if !ok || !resolved.SuccessType.Named || resolved.SuccessType.Kind != sema.StringType {
			t.Fatalf("lost nominal success type for %s: %+v", fn.Name.Value, resolved)
		}
		if len(resolved.Failures) < 1 || resolved.Failures[0].Kind != sema.TryFailureCarrier || resolved.Failures[0].ErrorType.Name != "StringError" {
			t.Fatalf("missing first materialization failure for %s", fn.Name.Value)
		}
		if fn.Name.Value == "Constrained" && (len(resolved.Failures) != 2 || resolved.Failures[1].Kind != sema.TryFailureContract || resolved.Failures[1].ErrorType.Name != "ConversionError") {
			t.Fatal("missing independent ConversionError.Contract failure")
		}
		seen[fn.Name.Value] = true
	}
	if !seen["Outer"] || !seen["Constrained"] {
		t.Fatalf("missing conversion facts: %v", seen)
	}
}
