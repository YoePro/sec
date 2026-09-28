package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Bodyless try retains its success type and propagation fact when nested in
// ordinary value positions rather than only when used as a let initializer.
//
// Rules:
//   - rules/errors/errorhandling.md — §12 "Naked try propagation"
//   - rules/errors/errorhandling.md — §37.3 "General try expression positions"
func TestTryExpressionWorksInCallArgumentsAndParentheses(t *testing.T) {
	source := `module main
enum ReadError error { Failed }
fn Read(value: int) Result[int, ReadError] { return Ok(value) }
fn Combine(left: int, right: int) int { return left + right }
fn Use() Result[int, ReadError] {
    let combined := Combine(try Read(1), try Read(2))
    let nested := (try Read(combined))
    return Ok(nested)
}
`
	result := parser.New(lexer.New(source)).Parse()
	if result.HasErrors {
		t.Fatalf("parse: %+v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %+v", errors)
	}

	use := result.Program.Statements[4].(*ast.FunctionDeclaration)
	call := use.Body.Statements[0].(*ast.LetStatement).Value.(*ast.CallExpression)
	tries := []*ast.TryExpression{
		call.Arguments[0].(*ast.TryExpression),
		call.Arguments[1].(*ast.TryExpression),
		use.Body.Statements[1].(*ast.LetStatement).Value.(*ast.TryExpression),
	}
	for index, expression := range tries {
		fact, ok := analyzer.ResolvedTryOf(expression)
		if !ok || fact.Kind != ResolvedTryResultPropagation || fact.SuccessType.Kind != IntType || fact.ErrorType.Name != "ReadError" {
			t.Fatalf("resolved try %d = %#v, %t", index, fact, ok)
		}
	}
}

// Naked Option try unwraps Some(T) and propagates None only through a
// compatible enclosing Option return channel.
//
// Rules:
//   - rules/errors/errorhandling.md — §12.2 "Option propagation"
//   - rules/errors/errorhandling.md — §12.3 "No arbitrary cross-channel conversion"
func TestNakedOptionTryPropagatesOnlyThroughCompatibleOption(t *testing.T) {
	valid := `module main
fn Find(value: int) Option[int] { return Some(value) }
fn Resolve() Option[int] {
    let left := try Find(1)
    let total := left + try Find(2)
    return Some(total)
}
`
	result := parser.New(lexer.New(valid)).Parse()
	if result.HasErrors {
		t.Fatalf("parse: %+v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %+v", errors)
	}
	resolve := result.Program.Statements[2].(*ast.FunctionDeclaration)
	first := resolve.Body.Statements[0].(*ast.LetStatement).Value.(*ast.TryExpression)
	second := resolve.Body.Statements[1].(*ast.LetStatement).Value.(*ast.InfixExpression).Right.(*ast.TryExpression)
	for index, expression := range []*ast.TryExpression{first, second} {
		fact, ok := analyzer.ResolvedTryOf(expression)
		if !ok || fact.Kind != ResolvedTryOptionPropagation || fact.SuccessType.Kind != IntType ||
			fact.EnclosingOptionType.Name != "Option" || len(fact.EnclosingOptionType.TypeArgs) != 1 ||
			fact.EnclosingOptionType.TypeArgs[0].Kind != IntType {
			t.Fatalf("resolved Option try %d = %#v, %t", index, fact, ok)
		}
	}

	errors := analyzeSourceRaw(t, `module main
fn FindNumber() Option[int] { return Some(1) }
fn ResultChannel() Result[int, ArithmeticError] {
    let value := try FindNumber()
    return Ok(value)
}
fn WrongOption() Option[string] {
    let value := try FindNumber()
    return Some("unused")
}
`)
	if len(errors) != 2 ||
		!strings.Contains(errors[0].Message, "cannot propagate None") ||
		!strings.Contains(errors[0].Message, "Result[int, ArithmeticError]") ||
		!strings.Contains(errors[1].Message, "Option[int]") ||
		!strings.Contains(errors[1].Message, "Option[string]") {
		t.Fatalf("Option try diagnostics = %+v", errors)
	}
}
