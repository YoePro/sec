package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestOptionIfBindingIsTrueBranchLocal verifies payload typing, binding
// identity, and absence from the else and continuation scopes.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §12 "State tests" and §17 "Branch scopes"
//   - rules/corrections/applied/if-errorhandling-correction-20260824.md — "Positive Some binding"
func TestOptionIfBindingIsTrueBranchLocal(t *testing.T) {
	source := `module main
fn Read(option: Option[int]) int {
    if option is Some(value) {
        return value
    } else {
        return 0
    }
}
`
	p := parser.New(lexer.NewWithFile(source, "option-if.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %#v", result.Diagnostics)
	}
	a := NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %#v", errors)
	}

	function := result.Program.Statements[1].(*ast.FunctionDeclaration)
	ifStmt := function.Body.Statements[0].(*ast.IfStatement)
	fact, ok := a.ResolvedOptionIfBindingOf(ifStmt)
	if !ok || fact.BindingName != "value" || fact.PayloadType.Kind != IntType || fact.BindingAction != MatchBindingCopyTrivial {
		t.Fatalf("resolved Option binding = %#v, %t", fact, ok)
	}
	use := ifStmt.Consequence.Statements[0].(*ast.ReturnStatement).Value.(*ast.Identifier)
	binding, ok := a.ResolvedBindingOf(use)
	if !ok || binding.Name != "value" || binding.Type.Kind != IntType {
		t.Fatalf("resolved payload binding = %#v, %t", binding, ok)
	}
}

// TestOptionIfBindingRejectsNonOptionSubject keeps the exception from becoming
// general Result or union payload destructuring in if.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §13 "No pattern binding in if"
func TestOptionIfBindingRejectsNonOptionSubject(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main
enum Failure error { Invalid }
fn Read(result: Result[int, Failure]) int {
    if result is Some(value) { return value }
    return 0
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "requires Option subject") || !strings.Contains(errors[0].Message, "use match") {
		t.Fatalf("errors = %#v", errors)
	}
}

// TestOptionIfBindingDoesNotEscape verifies the true-branch-only lexical
// lifetime independently of control-flow termination.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §17 "Branch scopes"
func TestOptionIfBindingDoesNotEscape(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main
fn Read(option: Option[int]) int {
    if option is Some(value) {}
    return value
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "undefined variable value") {
		t.Fatalf("errors = %#v", errors)
	}
}

// TestOptionIfBindingRecordsMoveOnlyOwnership verifies that the frontend uses
// the ordinary union payload move classification even though executable
// ownership-sensitive projection remains gated in Semantic IR.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §12 "State tests"
//   - rules/control-flow/flowcontrol_match.md — ordinary payload ownership rules
func TestOptionIfBindingRecordsMoveOnlyOwnership(t *testing.T) {
	source := `module main
@noCopy
type Resource struct { Value: int }
fn Inspect(option: Option[Resource]) int {
    if option is Some(resource) {
        return resource.Value
    }
    return 0
}
`
	p := parser.New(lexer.New(source))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %#v", result.Diagnostics)
	}
	a := NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %#v", errors)
	}
	function := result.Program.Statements[2].(*ast.FunctionDeclaration)
	ifStmt := function.Body.Statements[0].(*ast.IfStatement)
	fact, ok := a.ResolvedOptionIfBindingOf(ifStmt)
	if !ok || fact.BindingAction != MatchBindingMove || fact.PayloadType.Name != "Resource" {
		t.Fatalf("move-only Option binding = %#v, %t", fact, ok)
	}
}
