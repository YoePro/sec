package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

const tryAssignmentFixture = `module main

enum SetterError error { Rejected }
enum OtherError error { Rejected }

type Vehicle struct { speed: int }

impl Vehicle {
	property Speed: int {
		get { return speed }
		try set value { return Err(SetterError.Rejected) }
	}
}
`

// TestNakedTryAssignmentResolvesPropagation verifies exact and open-error
// propagation and the immutable statement-level fact consumed by later IR.
//
// Rules:
//   - rules/errors/errorhandling.md — §8 "try and error compatibility"
//   - rules/errors/errorhandling.md — §23 "Fallible assignment"
func TestNakedTryAssignmentResolvesPropagation(t *testing.T) {
	source := tryAssignmentFixture + `
fn Exact(vehicle: Vehicle) Result[void, SetterError] {
	try vehicle.Speed = 10
	return Ok()
}

fn Widen(vehicle: Vehicle) Result[void, error] {
	try vehicle.Speed = 20
	return Ok()
}
`
	result := parser.New(lexer.New(source)).Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	seen := 0
	for _, declaration := range result.Program.Statements {
		function, ok := declaration.(*ast.FunctionDeclaration)
		if !ok || (function.Name.Value != "Exact" && function.Name.Value != "Widen") {
			continue
		}
		statement := function.Body.Statements[0].(*ast.TryAssignmentStatement)
		fact, ok := analyzer.ResolvedTryAssignmentOf(statement)
		if !ok || fact.Kind != ResolvedTryAssignmentPropagation || fact.ErrorType.Name != "SetterError" || fact.EnclosingResultType.Kind != ResultType {
			t.Fatalf("%s fact = %#v, %t", function.Name.Value, fact, ok)
		}
		seen++
	}
	if seen != 2 {
		t.Fatalf("resolved %d propagation facts, want 2", seen)
	}
	if _, ok := analyzer.ResolvedTryAssignmentOf(&ast.TryAssignmentStatement{}); ok {
		t.Fatal("unknown statement unexpectedly acquired a resolved fact")
	}
}

// TestNakedTryAssignmentRejectsIncompatibleChannels ensures naked assignment
// remains Result-only and does not invent conversion between concrete errors.
//
// Rules:
//   - rules/errors/errorhandling.md — §8 "try and error compatibility"
//   - rules/errors/errorhandling.md — §23 "Fallible assignment"
func TestNakedTryAssignmentRejectsIncompatibleChannels(t *testing.T) {
	errors := analyzeSourceRaw(t, tryAssignmentFixture+`
fn Plain(vehicle: Vehicle) void {
	try vehicle.Speed = 10
}

fn Wrong(vehicle: Vehicle) Result[void, OtherError] {
	try vehicle.Speed = 20
	return Ok()
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %#v, want 2", errors)
	}
	if !strings.Contains(errors[0].Message, "naked try assignment propagates SetterError") || !strings.Contains(errors[0].Message, "returns void") {
		t.Fatalf("plain-return diagnostic = %q", errors[0].Message)
	}
	if !strings.Contains(errors[1].Message, "map SetterError to OtherError") {
		t.Fatalf("incompatible-error diagnostic = %q", errors[1].Message)
	}
}

// TestHandledTryAssignmentPublishesHandlerPlan preserves the existing local
// handler behavior while exposing its resolved plan on the source statement.
//
// Rules:
//   - rules/errors/errorhandling.md — §23 "Fallible assignment"
func TestHandledTryAssignmentPublishesHandlerPlan(t *testing.T) {
	source := tryAssignmentFixture + `
fn Handle(vehicle: Vehicle) void {
	try vehicle.Speed = 10 {
		Err(error) => { discard error }
	}
}
`
	result := parser.New(lexer.New(source)).Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	function := result.Program.Statements[len(result.Program.Statements)-1].(*ast.FunctionDeclaration)
	statement := function.Body.Statements[0].(*ast.TryAssignmentStatement)
	fact, ok := analyzer.ResolvedTryAssignmentOf(statement)
	if !ok || fact.Kind != ResolvedTryAssignmentHandled || len(fact.HandlerPlan.Handlers) != 1 {
		t.Fatalf("handled fact = %#v, %t", fact, ok)
	}
}
