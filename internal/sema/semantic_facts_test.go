package sema

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestSemanticFactsRetainBindingAndExactCallTarget(t *testing.T) {
	source := `module main
fn Value(value: int) int { return value }
fn Value(value: bool) bool { return value }
fn Main(flag: bool) bool {
  let local := flag
  return Value(local)
}`
	p := parser.New(lexer.NewWithFile(source, "facts.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	mainFn := result.Program.Statements[3].(*ast.FunctionDeclaration)
	let := mainFn.Body.Statements[0].(*ast.LetStatement)
	ret := mainFn.Body.Statements[1].(*ast.ReturnStatement)
	call := ret.Value.(*ast.CallExpression)
	use := call.Arguments[0].(*ast.Identifier)
	declFact, ok := a.ResolvedBindingOf(let.Name)
	if !ok {
		t.Fatal("local declaration has no BindingID")
	}
	useFact, ok := a.ResolvedBindingOf(use)
	if !ok {
		t.Fatal("local use has no BindingID")
	}
	if declFact.ID == 0 || declFact.ID != useFact.ID || declFact.Kind != BindingLocal {
		t.Fatalf("declaration=%#v use=%#v", declFact, useFact)
	}
	resolved, ok := a.ResolvedCallTarget(call)
	if !ok {
		t.Fatal("call target was not retained")
	}
	if resolved.Kind != ResolvedDirectCall || resolved.Function.Name != "Value" || len(resolved.Function.Parameters) != 1 || resolved.Function.Parameters[0].Type.Kind != BoolType {
		t.Fatalf("resolved call = %#v", resolved)
	}
	before := len(a.expressionTypes)
	if _, ok := a.ResolvedTypeOf(use); !ok {
		t.Fatal("resolved expression type missing")
	}
	if len(a.expressionTypes) != before {
		t.Fatal("read-only query mutated analyzer")
	}
}

func TestSemanticFactsRetainResolvedBuiltinIntegerOperators(t *testing.T) {
	source := `module main
fn Signed(a: int, b: int) int { return -(a + b) }
fn Unsigned(a: uint, b: uint) uint { return a >> b }
fn Compare(a: int, b: int) bool { return a <= b }
`
	p := parser.New(lexer.NewWithFile(source, "operators.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	signedReturn := result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement)
	negate := signedReturn.Value.(*ast.PrefixExpression)
	add := negate.Right.(*ast.InfixExpression)
	unsignedReturn := result.Program.Statements[2].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement)
	shift := unsignedReturn.Value.(*ast.InfixExpression)
	compareReturn := result.Program.Statements[3].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement)
	compare := compareReturn.Value.(*ast.InfixExpression)

	for expression, want := range map[ast.Expression]ResolvedOperatorKind{
		add: ResolvedIntegerAddChecked, negate: ResolvedIntegerNegateChecked,
		shift: ResolvedIntegerShiftRightUnsignedChecked, compare: ResolvedIntegerCompareLE,
	} {
		resolved, ok := a.ResolvedOperatorOf(expression)
		if !ok || resolved.Kind != want {
			t.Errorf("resolved %T = %#v, %t; want %s", expression, resolved, ok, want)
		}
	}
	before := len(a.resolvedOperators)
	if _, ok := a.ResolvedOperatorOf(&ast.InfixExpression{}); ok || len(a.resolvedOperators) != before {
		t.Fatal("read-only operator query resolved or mutated an unknown expression")
	}
}

func TestResolvedIntegerOperatorsCoverKindsAndActiveWidths(t *testing.T) {
	source := `module main
fn F01(a: int8) int8 { return +a }
fn F02(a: int16) int16 { return -a }
fn F03(a: int32, b: int32) int32 { return a + b }
fn F04(a: int64, b: int64) int64 { return a - b }
fn F05(a: int128, b: int128) int128 { return a * b }
fn F06(a: int256, b: int256) int256 { return a / b }
fn F07(a: uint8, b: uint8) uint8 { return a % b }
fn F08(a: uint16) uint16 { return ~a }
fn F09(a: uint32, b: uint32) uint32 { return a & b }
fn F10(a: uint64, b: uint64) uint64 { return a | b }
fn F11(a: uint128, b: uint128) uint128 { return a ^ b }
fn F12(a: uint256, count: int) uint256 { return a << count }
fn F13(a: int64, count: uint8) int64 { return a >> count }
fn F14(a: int256, count: uint16) int256 { return a << count }
fn F15(a: uint64, count: int8) uint64 { return a >> count }
fn F16(a: int, b: int) bool { return a == b }
fn F17(a: int, b: int) bool { return a != b }
fn F18(a: int, b: int) bool { return a < b }
fn F19(a: int, b: int) bool { return a <= b }
fn F20(a: int, b: int) bool { return a > b }
fn F21(a: int, b: int) bool { return a >= b }
`
	p := parser.New(lexer.NewWithFile(source, "operator-matrix.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(result.Program); len(errs) > 0 {
		t.Fatalf("sema: %v", errs)
	}
	wants := []ResolvedOperatorKind{
		ResolvedIntegerUnaryPlus, ResolvedIntegerNegateChecked,
		ResolvedIntegerAddChecked, ResolvedIntegerSubtractChecked,
		ResolvedIntegerMultiplyChecked, ResolvedIntegerDivideChecked,
		ResolvedIntegerRemainderChecked, ResolvedIntegerBitNot,
		ResolvedIntegerBitAnd, ResolvedIntegerBitOr, ResolvedIntegerBitXor,
		ResolvedIntegerShiftLeftUnsignedChecked, ResolvedIntegerShiftRightSignedChecked,
		ResolvedIntegerShiftLeftSignedChecked, ResolvedIntegerShiftRightUnsignedChecked,
		ResolvedIntegerCompareEQ, ResolvedIntegerCompareNE, ResolvedIntegerCompareLT,
		ResolvedIntegerCompareLE, ResolvedIntegerCompareGT, ResolvedIntegerCompareGE,
	}
	for index, want := range wants {
		function := result.Program.Statements[index+1].(*ast.FunctionDeclaration)
		expression := function.Body.Statements[0].(*ast.ReturnStatement).Value
		resolved, ok := a.ResolvedOperatorOf(expression)
		if !ok || resolved.Kind != want {
			t.Errorf("%s: resolved = %#v, %t; want %s", function.Name.Value, resolved, ok, want)
		}
	}
}
