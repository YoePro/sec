package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// MD-028 regression matrix: try protects its complete following expression,
// including receivers, arguments, operators, indexing and every successive
// member and call of a chain. The failure set records every possible failure
// in evaluation order; each failure is handled locally, propagates through
// the declared compatible channel, or is rejected with its source.
//
// Rules:
//   - rules/errors/errorhandling.md — §§ 11, 11.1, 13, 22
//   - rules/errors/runtime_checks.md — "try boundaries", "What try converts"
//   - rules/corrections/applied/md028-md032-md055-try-switch-import-comments-correction-20261012.md — §§ 2, 5.1

const protectedSubtreePrelude = `
module main

enum StageError error { Failed }

type Stage struct { level: int }

impl Stage {
    fn Next() Result[Stage, StageError] {
        return Ok(Stage { level: level + 1 })
    }
    fn Level() int {
        return level
    }
}

fn Start() Result[Stage, StageError] {
    return Ok(Stage { level: 0 })
}

fn Wrap(value: int) Result[int, StageError] {
    return Ok(value)
}

fn Find(flag: bool) Option[int] {
    if flag {
        return Some(1)
    }
    return None
}
`

func analyzeProtectedSubtree(t *testing.T, source string) (*Analyzer, *ast.Program, []Error) {
	t.Helper()
	program := parser.New(lexer.New(protectedSubtreePrelude + source)).ParseProgram()
	analyzer := NewAnalyzer()
	return analyzer, program, analyzer.Analyze(program)
}

// firstLetTry returns the try initializing the first let of function name.
func firstLetTry(t *testing.T, program *ast.Program, name string) *ast.TryExpression {
	t.Helper()
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok || function.Name.Value != name {
			continue
		}
		let := function.Body.Statements[0].(*ast.LetStatement)
		if try, ok := let.Value.(*ast.TryExpression); ok {
			return try
		}
		if infix, ok := let.Value.(*ast.InfixExpression); ok {
			return infix.Left.(*ast.TryExpression)
		}
	}
	t.Fatalf("no let try in %s", name)
	return nil
}

func failureSummary(points []TryFailurePoint) string {
	parts := make([]string, 0, len(points))
	for _, point := range points {
		parts = append(parts, string(point.Kind)+":"+typeDisplayName(point.ErrorType)+"@"+point.Expression.String())
	}
	return strings.Join(parts, " ")
}

func TestTryProtectsCompleteChainsReceiversAndArguments(t *testing.T) {
	analyzer, program, errors := analyzeProtectedSubtree(t, `
fn Chain() Result[int, StageError] {
    let level := try Start().Next().Next().Level()
    return Ok(level)
}

fn Receiver() Result[int, StageError] {
    let level := try Start().Level()
    return Ok(level)
}

fn Argument(a: int, b: int) Result[int, error] {
    let value := try Wrap(a + b)
    return Ok(value)
}

fn Mixed(values: int[3], index: int, a: int) Result[int, error] {
    let value := try Wrap(a + values[index])
    return Ok(value)
}
`)
	assertSemaErrors(t, errors, nil)
	for name, want := range map[string]string{
		// Each stage is tested before the member that consumes it, so the
		// first failure short-circuits every later call.
		"Chain":    "carrier:StageError@Start() carrier:StageError@Start().Next() carrier:StageError@Start().Next().Next()",
		"Receiver": "carrier:StageError@Start()",
		"Argument": "arithmetic:ArithmeticError@(a + b) carrier:StageError@Wrap((a + b))",
		// Distinct concrete errors propagate through the declared open error
		// channel with their own identities.
		"Mixed": "bounds:IndexError@values[index] arithmetic:ArithmeticError@(a + values[index]) carrier:StageError@Wrap((a + values[index]))",
	} {
		fact, ok := analyzer.ResolvedTryOf(firstLetTry(t, program, name))
		if !ok || fact.Kind != ResolvedTryFailureSetPropagation {
			t.Fatalf("%s: try fact = %+v, %t", name, fact, ok)
		}
		if got := failureSummary(fact.Failures); got != want {
			t.Errorf("%s failures:\n got %s\nwant %s", name, got, want)
		}
	}
}

func TestTryProtectedSubtreeRejectsUnpropagatableAndAmbiguousFailures(t *testing.T) {
	_, _, errors := analyzeProtectedSubtree(t, `
fn Narrow(values: int[3], index: int) Result[int, StageError] {
    let value := try Wrap(values[index])
    return Ok(value)
}

fn Binding(values: int[3], index: int) int {
    let value := try Wrap(values[index]) {
        Err(failure) => 0
    }
    return value
}

fn Discard(values: int[3], index: int) int {
    let value := try Wrap(values[index]) {
        Err(_) => 0
    }
    return value
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v, want the narrow-channel and heterogeneous-binding rejections", errors)
	}
	if errors[0].ID != "S1093" || !strings.Contains(errors[0].Message, "IndexError from values[index]") {
		t.Errorf("narrow channel diagnostic = %v, want S1093 naming the offending path", errors[0])
	}
	if errors[1].ID != "S1091" || !strings.Contains(errors[1].Message, "Err(failure)") {
		t.Errorf("heterogeneous binding diagnostic = %v, want S1091", errors[1])
	}
}

func TestTryProtectedSubtreeBoundaries(t *testing.T) {
	analyzer, program, errors := analyzeProtectedSubtree(t, `
fn Parenthesized(a: int, b: int) Result[int, StageError] {
    let value := (try Wrap(a)) + b
    return Ok(value)
}

fn HandlerOutside(a: int, b: int) int {
    let value := try Wrap(a) {
        Err(_) => a + b
    }
    return value
}
`)
	assertSemaErrors(t, errors, nil)
	for name, want := range map[string]string{
		// Parentheses end the protected expression before `+ b`.
		"Parenthesized": "",
		// The handler body is outside the protected set of its own try.
		"HandlerOutside": "",
	} {
		fact, ok := analyzer.ResolvedTryOf(firstLetTry(t, program, name))
		if !ok || failureSummary(fact.Failures) != want {
			t.Errorf("%s: try fact = %+v, %t; want no failure-set points beyond the carrier", name, fact, ok)
		}
	}
}

func TestTryProtectedSubtreeKeepsOptionAbsenceSeparate(t *testing.T) {
	_, _, errors := analyzeProtectedSubtree(t, `
fn OptionWithError(a: int, b: int) Option[int] {
    let value := try Find(a + b > 0)
    return Some(value)
}

fn ResultInOption() Option[int] {
    let value := try Wrap(1)
    return Some(value)
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v, want both cross-channel rejections", errors)
	}
	if !strings.Contains(errors[0].Message, "absence and errors use different channels") {
		t.Errorf("Option try with an error point = %v", errors[0])
	}
	if !strings.Contains(errors[1].Message, "Option") {
		t.Errorf("Result try in an Option function = %v", errors[1])
	}
}
