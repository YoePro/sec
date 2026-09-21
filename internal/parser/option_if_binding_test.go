package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestParsePositiveOptionIfBinding preserves the canonical source form without
// turning if into a general pattern construct.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §12 "State tests" and §13 "No pattern binding in if"
//   - rules/corrections/applied/grammar-errorhandling-correction-20260824.md — "Option payload binding in if"
func TestParsePositiveOptionIfBinding(t *testing.T) {
	p := New(lexer.New(`module main
fn Read(option: Option[int]) int {
    if option is Some(value) {
        return value
    }
    return 0
}`))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	function := program.Statements[1].(*ast.FunctionDeclaration)
	ifStmt := function.Body.Statements[0].(*ast.IfStatement)
	if ifStmt.OptionBinding == nil || ifStmt.OptionBinding.Binding == nil || ifStmt.OptionBinding.Binding.Value != "value" {
		t.Fatalf("Option binding was not preserved: %#v", ifStmt.OptionBinding)
	}
	if subject, ok := ifStmt.OptionBinding.Subject.(*ast.Identifier); !ok || subject.Value != "option" || ifStmt.Condition != subject {
		t.Fatalf("Option subject was not preserved exactly once: condition=%#v binding=%#v", ifStmt.Condition, ifStmt.OptionBinding)
	}
}

// TestParseNegativeOptionIfBindingRejectsTheBinding checks the focused rule
// instead of accepting a binding on a path where no Some payload exists.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §13 "No pattern binding in if"
//   - rules/corrections/applied/if-errorhandling-correction-20260824.md — "Negative binding is invalid"
func TestParseNegativeOptionIfBindingRejectsTheBinding(t *testing.T) {
	p := New(lexer.New(`module main
fn Read(option: Option[int]) void {
    if option is not Some(value) {}
}`))
	p.ParseProgram()
	if len(p.Errors()) != 1 || !strings.Contains(p.Errors()[0], "negative Option binding is invalid; use match") {
		t.Fatalf("diagnostics = %#v", p.Errors())
	}
}
