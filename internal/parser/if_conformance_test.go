package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestIfConformanceHeaders rejects assignment, declaration and structural
// payload bindings with one registered error each, retaining every body and
// following function. Comparison chains remain represented for Sema rejection.
// Rules: rules/control-flow/flowcontrol_if.md — §§7,13–15,26,28;
// rules/compiler/parser_recovery.md — Bounded damage.
func TestIfConformanceHeaders(t *testing.T) {
	source, err := os.ReadFile("../../testdata/control_flow/if_conformance/headers_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(source), "headers_invalid.sec")).Parse()
	want := []string{diagnostics.ParserInvalidAssignmentExpr, diagnostics.ParserInvalidPattern, diagnostics.ParserInvalidPattern}
	lines := []int{6, 10, 18}
	if !result.HasErrors || result.Fatal || len(result.Diagnostics) != len(want) {
		t.Fatalf("want three recoverable header errors: %+v", result.Diagnostics)
	}
	for i, d := range result.Diagnostics {
		if d.ID != want[i] || d.Primary.Line != lines[i] {
			t.Fatalf("header diagnostic %d: %+v", i, d)
		}
	}
	functions := 0
	for _, statement := range result.Program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		functions++
		if fn.Body == nil || (fn.Name.Value != "Following" && len(fn.Body.Statements) != 1) {
			t.Fatalf("lost function body: %+v", fn)
		}
	}
	if functions != 5 {
		t.Fatalf("retained %d functions, want five", functions)
	}
}
