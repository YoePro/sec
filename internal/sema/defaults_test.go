package sema

import (
	"math/big"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestDefaultValueOfPrimitiveAndConstrainedTypes(t *testing.T) {
	positive := Type{Name: "PositiveEven", Kind: IntType, Named: true, MinInteger: intBound(-2147483648), MaxInteger: intBound(2147483647), Contracts: []Contract{
		RangeContract{Min: intBound(1), Max: intBound(100)}, MarkerContract{Name: "even"},
	}}
	resolved := DefaultValueOf(positive)
	if resolved.Kind != RangeDefault || resolved.Value.Integer == nil || resolved.Value.Integer.Int64() != 2 {
		t.Fatalf("positive even default = %#v, want 2", resolved)
	}
	huge := Type{Name: "Huge", Kind: IntType, Named: true, MinInteger: intBound(-9_000_000), MaxInteger: intBound(9_000_000), Contracts: []Contract{RangeContract{Min: intBound(2_000_000), Max: intBound(3_000_000)}}}
	resolved = DefaultValueOf(huge)
	if resolved.Value.Integer == nil || resolved.Value.Integer.Int64() != 2_000_000 {
		t.Fatalf("large range default = %#v", resolved)
	}
	ambiguous := Type{Name: "NonZeroOdd", Kind: IntType, Named: true, MinInteger: intBound(-9), MaxInteger: intBound(9), Contracts: []Contract{MarkerContract{Name: "odd"}}}
	if resolved := DefaultValueOf(ambiguous); resolved.Kind != NoDefault {
		t.Fatalf("ambiguous default = %#v", resolved)
	}

	user := Type{Name: "User", Kind: StringType, Named: true, Contracts: []Contract{MembershipContract{Values: []DefaultConstant{
		{Kind: StringType, Lexeme: `"Admin"`, String: "Admin"},
		{Kind: StringType, Lexeme: `"User"`, String: "User"},
	}}}}
	resolved = DefaultValueOf(user)
	if resolved.Kind != MembershipDefault || resolved.Value.String != "Admin" {
		t.Fatalf("membership default = %#v, want Admin", resolved)
	}
	required := Type{Name: "RequiredBool", Kind: BoolType, Named: true, Contracts: []Contract{MembershipContract{Values: []DefaultConstant{{Kind: BoolType, Lexeme: "true", Bool: true}}}}}
	if resolved := DefaultValueOf(required); resolved.Kind != MembershipDefault || !resolved.Value.Bool {
		t.Fatalf("bool membership default = %#v", resolved)
	}
	point := Type{Name: "Point", Kind: StructType, Named: true, Fields: []StructField{{Name: "x", Type: Type{Name: "int", Kind: IntType}}, {Name: "label", Type: Type{Name: "string", Kind: StringType}}}}
	if resolved := DefaultValueOf(point); resolved.Kind != StructDefault || len(resolved.Fields) != 2 {
		t.Fatalf("struct default = %#v", resolved)
	}
	array := Type{Name: "int[3]", Kind: ArrayType, Element: &Type{Name: "int", Kind: IntType}, ArrayLength: 3}
	if resolved := DefaultValueOf(array); resolved.Kind != ArrayDefault || len(resolved.Elements) != 3 {
		t.Fatalf("array default = %#v", resolved)
	}
	emptyRefs := Type{Name: "ref int[0]", Kind: ArrayType, Element: &Type{Name: "ref int", Kind: ReferenceType}, ArrayLength: 0}
	if resolved := DefaultValueOf(emptyRefs); resolved.Kind != ArrayDefault || len(resolved.Elements) != 0 {
		t.Fatalf("zero-length array default = %#v", resolved)
	}
	combinedMultiples := Type{Name: "CombinedMultiples", Kind: IntType, Named: true, MinInteger: intBound(-100), MaxInteger: intBound(100), Contracts: []Contract{
		RangeContract{Min: intBound(1), Max: intBound(100)},
		MultipleOfContract{Value: intBound(4)},
		MultipleOfContract{Value: intBound(6)},
	}}
	if resolved := DefaultValueOf(combinedMultiples); resolved.Kind != RangeDefault || resolved.Value.Integer == nil || resolved.Value.Integer.Int64() != 12 {
		t.Fatalf("combined multipleOf default = %#v, want 12", resolved)
	}
	positiveAmount := Type{Name: "PositiveAmount", Kind: DecimalType, Named: true, Contracts: []Contract{RangeContract{
		ExactMin:  exactRat("0.01"),
		ExactMax:  exactRat("1000000.00"),
		MinLexeme: "0.01",
		MaxLexeme: "1000000.00",
	}}}
	if resolved := DefaultValueOf(positiveAmount); resolved.Kind != RangeDefault || resolved.Value.Lexeme != "0.01" {
		t.Fatalf("decimal range default = %#v, want 0.01", resolved)
	}
}

func TestAnalyzerMaterializesDefaultsWithoutRuntime(t *testing.T) {
	input := `module main

type Port int range 1..65535 default 8080

type PositiveAmount decimal range 0.01..1000000.00

type Position struct {
    line: int,
    enabled: bool,
    name: string,
    port: Port,
}

fn Test() Position {
    let mut count: int
    let mut amount: PositiveAmount
    return Position { line: count }
}
`
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatalf("sema errors: %v", errors)
	}

	port := a.Types()["Port"]
	if port.ExplicitDefault == nil || port.ExplicitDefault.Integer.Int64() != 8080 {
		t.Fatalf("Port default = %#v", port.ExplicitDefault)
	}
	amount := a.Types()["PositiveAmount"]
	if display, kind, ok := DefaultValueDisplay(amount); !ok || kind != RangeDefault || display != "0.01" {
		t.Fatalf("PositiveAmount default = %q, %q, %v", display, kind, ok)
	}
	fn := program.Statements[4].(*ast.FunctionDeclaration)
	let := fn.Body.Statements[0].(*ast.LetStatement)
	if let.Value == nil || let.Value.String() != "0" {
		t.Fatalf("mutable int default = %v", let.Value)
	}
	amountLet := fn.Body.Statements[1].(*ast.LetStatement)
	if amountLet.Value == nil || amountLet.Value.String() != "0.01" {
		t.Fatalf("mutable decimal default = %v", amountLet.Value)
	}
	ret := fn.Body.Statements[2].(*ast.ReturnStatement)
	literal := ret.Value.(*ast.StructLiteral)
	if len(literal.Fields) != 4 {
		t.Fatalf("materialized fields = %d, want 4", len(literal.Fields))
	}
}

func TestInvalidExplicitDefaultIsRejected(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Port int range 1..65535 default 0
`)
	assertSemaErrors(t, errors, []string{"default value 0 is invalid for Port at 3:38"})
	if errors[0].ID != diagnostics.InvalidExplicitDefault {
		t.Fatalf("wrong diagnostic ID: %q", errors[0].ID)
	}
}

func TestExplicitDefaultMustBelongToMembership(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type User string in ["Admin", "User"] default "Guest"
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.InvalidExplicitDefault {
		t.Fatalf("errors = %v, want one invalid explicit default", errors)
	}
}

func TestEveryMembershipValueMustSatisfyOtherContracts(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type InvalidEven int in [1, 2, 3] even
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v, want two invalid membership values", errors)
	}
	for _, diagnostic := range errors {
		if diagnostic.ID != diagnostics.InvalidMembershipValue {
			t.Fatalf("wrong diagnostic ID: %q", diagnostic.ID)
		}
	}

	analyzer, validErrors := analyzeSourceWithAnalyzer(t, `module main

type SmallEven int in [2, 4, 6] even
type NonEmptyName string in ["Admin", "User"] notEmpty
`)
	if len(validErrors) != 0 {
		t.Fatalf("valid membership contracts produced errors: %v", validErrors)
	}
	if value, _, ok := DefaultValueDisplay(analyzer.Types()["SmallEven"]); !ok || value != "2" {
		t.Fatalf("SmallEven default = %q, %v", value, ok)
	}
	if value, _, ok := DefaultValueDisplay(analyzer.Types()["NonEmptyName"]); !ok || value != `"Admin"` {
		t.Fatalf("NonEmptyName default = %q, %v", value, ok)
	}
}

func TestExplicitDefaultAcceptsIntegerConstantExpression(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

type Port int range 1..65535 default 8000 + 80
`)
	if len(errors) != 0 {
		t.Fatalf("constant default produced errors: %v", errors)
	}
	if value, kind, ok := DefaultValueDisplay(analyzer.Types()["Port"]); !ok || kind != ExplicitTypeDefault || value != "8080" {
		t.Fatalf("Port default = %q, %q, %v", value, kind, ok)
	}
}

func TestInvalidExplicitDecimalDefaultIsRejected(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type PositiveAmount decimal range 0.01..100.00 default 0.00
`)
	if len(errors) != 1 || errors[0].ID != diagnostics.InvalidExplicitDefault {
		t.Fatalf("errors = %v, want one invalid explicit default", errors)
	}
}

func TestNonDefaultableDeclarationsAndFieldsAreRejected(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Holder struct {
    value: ref int,
}

fn Invalid(ref source: int) Holder {
    let mut view: ref int
    return Holder {}
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v, want 2", errors)
	}
	if errors[0].ID != diagnostics.NoDefaultValue || errors[1].ID != diagnostics.MissingNonDefaultableField {
		t.Fatalf("wrong default diagnostic IDs: %v", errors)
	}
}

func intBound(value int64) *big.Int { return big.NewInt(value) }

func exactRat(value string) *big.Rat {
	exact, ok := new(big.Rat).SetString(value)
	if !ok {
		panic("invalid test rational: " + value)
	}
	return exact
}
