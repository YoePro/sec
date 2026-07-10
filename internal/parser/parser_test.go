package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestParseModuleImportAndTypeDeclarations(t *testing.T) {
	input := `
module main

import "std/fmt"

let i: int := 0
let mut u: uint
let s: string := ""
let mut b: bool
let mut n: decimal


type Percent int range 0..100
type Meter decimal<m>
type Speed decimal<m/s>
type Email string
`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 11 {
		t.Fatalf("wrong statement count. got=%d want=11", len(program.Statements))
	}

	moduleStmt, ok := program.Statements[0].(*ast.ModuleStatement)
	if !ok {
		t.Fatalf("statement 0 is not ModuleStatement. got=%T", program.Statements[0])
	}

	if moduleStmt.Path != "main" {
		t.Fatalf("wrong module path. got=%q want=%q", moduleStmt.Path, "main")
	}

	importStmt, ok := program.Statements[1].(*ast.ImportStatement)
	if !ok {
		t.Fatalf("statement 1 is not ImportStatement. got=%T", program.Statements[1])
	}

	if importStmt.Path != "std/fmt" {
		t.Fatalf("wrong import path. got=%q want=%q", importStmt.Path, "std/fmt")
	}

	assertLetDecl(t, program.Statements[2], "i", "int", false)
	assertLetDecl(t, program.Statements[3], "u", "uint", true)
	assertLetDecl(t, program.Statements[4], "s", "string", false)
	assertLetDecl(t, program.Statements[5], "b", "bool", true)
	assertLetDecl(t, program.Statements[6], "n", "decimal", true)

	assertTypeDecl(t, program.Statements[7], "Percent", "int", "", 0, 100)
	assertTypeDecl(t, program.Statements[8], "Meter", "decimal", "m", nil, nil)
	assertTypeDecl(t, program.Statements[9], "Speed", "decimal", "m/s", nil, nil)
	assertTypeDecl(t, program.Statements[10], "Email", "string", "", nil, nil)

}

func TestParseModuleRequiresName(t *testing.T) {
	l := lexer.New("module")
	p := New(l)

	p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}

	expected := "module declaration missing name at 1:1"
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
}

func assertTypeDecl(
	t *testing.T,
	stmt ast.Statement,
	name string,
	baseType string,
	unit string,
	min any,
	max any,
) {
	t.Helper()

	typeDecl, ok := stmt.(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement is not TypeDeclStatement. got=%T", stmt)
	}

	if typeDecl.Name.Value != name {
		t.Fatalf("wrong type name. got=%q want=%q", typeDecl.Name.Value, name)
	}

	if typeDecl.BaseType.Name != baseType {
		t.Fatalf("wrong base type. got=%q want=%q", typeDecl.BaseType.Name, baseType)
	}

	if typeDecl.BaseType.Unit != unit {
		t.Fatalf("wrong unit. got=%q want=%q", typeDecl.BaseType.Unit, unit)
	}

	if min == nil && max == nil {
		if typeDecl.Contract != nil {
			t.Fatalf("expected no contract, got=%T", typeDecl.Contract)
		}
		return
	}

	rangeContract, ok := typeDecl.Contract.(*ast.RangeContract)
	if !ok {
		t.Fatalf("expected RangeContract, got=%T", typeDecl.Contract)
	}

	assertLiteralValue(t, rangeContract.Min, min)
	assertLiteralValue(t, rangeContract.Max, max)
}

func assertLetDecl(t *testing.T, stmt ast.Statement, name string, typeName string, mutable bool) {
	t.Helper()

	letStmt, ok := stmt.(*ast.LetStatement)
	if !ok {
		t.Fatalf("statement is not LetStatement. got=%T", stmt)
	}

	if letStmt.Name.Value != name {
		t.Fatalf("wrong let name. got=%q want=%q", letStmt.Name.Value, name)
	}

	if typeName == "" {
		if letStmt.Type != nil {
			t.Fatalf("expected no let type, got=%T", letStmt.Type)
		}
	} else {
		if letStmt.Type == nil {
			t.Fatalf("expected let type %q, got nil", typeName)
		}

		if letStmt.Type.Name != typeName {
			t.Fatalf("wrong let type. got=%q want=%q", letStmt.Type.Name, typeName)
		}
	}

	if letStmt.Mutable != mutable {
		t.Fatalf("wrong mutability. got=%v want=%v", letStmt.Mutable, mutable)
	}
}

func assertLiteralValue(t *testing.T, expr ast.Expression, expected any) {
	t.Helper()

	switch expectedValue := expected.(type) {
	case int:
		lit, ok := expr.(*ast.IntegerLiteral)
		if !ok {
			t.Fatalf("expected IntegerLiteral, got=%T", expr)
		}

		if lit.Value != int64(expectedValue) {
			t.Fatalf("wrong integer value. got=%d want=%d", lit.Value, expectedValue)
		}

	default:
		t.Fatalf("unsupported expected literal type %T", expected)
	}
}

func checkParserErrors(t *testing.T, p *Parser) {
	t.Helper()

	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Fatalf("parser had errors: %v", errors)
}

func TestParseLetInitializer(t *testing.T) {
	input := `let a := 123`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
	}

	letStmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("statement is not LetStatement. got=%T", program.Statements[0])
	}

	if letStmt.Name.Value != "a" {
		t.Fatalf("wrong let name. got=%q want=%q", letStmt.Name.Value, "a")
	}

	if letStmt.Type != nil {
		t.Fatalf("expected no type annotation, got %T", letStmt.Type)
	}

	intLit, ok := letStmt.Value.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expected IntegerLiteral, got=%T", letStmt.Value)
	}

	if intLit.Value != 123 {
		t.Fatalf("wrong integer value. got=%d want=%d", intLit.Value, 123)
	}
}

func TestParseLetExpressionString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`let f := !!enabled`, `(!(!enabled))`},
		{`let x := (1 + 2) * 3`, `((1 + 2) * 3)`},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)

		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
		}

		letStmt, ok := program.Statements[0].(*ast.LetStatement)
		if !ok {
			t.Fatalf("statement is not LetStatement. got=%T", program.Statements[0])
		}

		if letStmt.Value == nil {
			t.Fatal("expected let initializer, got nil")
		}

		if got := letStmt.Value.String(); got != tt.want {
			t.Fatalf("wrong expression string for %q. got=%q want=%q", tt.input, got, tt.want)
		}
	}
}

func TestParseAssignmentStatement(t *testing.T) {
	tests := []struct {
		input    string
		target   string
		operator string
		value    string
	}{
		{input: `a = u - 6`, target: "a", operator: "=", value: "(u - 6)"},
		{input: `p += .1`, target: "p", operator: "+=", value: ".1"},
		{input: `p += 0.1`, target: "p", operator: "+=", value: "0.1"},
		{input: `precent = Percent(_a)`, target: "precent", operator: "=", value: "Percent(_a)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)

			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
			}

			stmt, ok := program.Statements[0].(*ast.AssignmentStatement)
			if !ok {
				t.Fatalf("statement is not AssignmentStatement. got=%T", program.Statements[0])
			}

			if stmt.Target.String() != tt.target {
				t.Fatalf("wrong assignment target. got=%q want=%q", stmt.Target.String(), tt.target)
			}

			if stmt.Operator != tt.operator {
				t.Fatalf("wrong assignment operator. got=%q want=%q", stmt.Operator, tt.operator)
			}

			if got := stmt.Value.String(); got != tt.value {
				t.Fatalf("wrong assignment value. got=%q want=%q", got, tt.value)
			}
		})
	}
}

func TestParseLetGroups(t *testing.T) {
	tests := []struct {
		input    string
		count    int
		mutable  bool
		typeName string
	}{
		{input: `int mut: a, b, c`, count: 3, mutable: true, typeName: "int"},
		{input: `float: a := 5.4, pi := 3.14`, count: 2, mutable: false, typeName: "float"},
		{input: `let a := 9, b := "hello", c := true`, count: 3, mutable: false, typeName: ""},
		{input: `let mut a := 9, b := "hello", c := false`, count: 3, mutable: true, typeName: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
			}

			group, ok := program.Statements[0].(*ast.LetGroupStatement)
			if !ok {
				t.Fatalf("statement is not LetGroupStatement. got=%T", program.Statements[0])
			}

			if len(group.Lets) != tt.count {
				t.Fatalf("wrong let count. got=%d want=%d", len(group.Lets), tt.count)
			}

			for _, let := range group.Lets {
				if let.Mutable != tt.mutable {
					t.Fatalf("wrong mutability for %s. got=%v want=%v", let.Name.Value, let.Mutable, tt.mutable)
				}
				if tt.typeName == "" {
					if let.Type != nil {
						t.Fatalf("expected no type for %s, got %s", let.Name.Value, let.Type.Name)
					}
					continue
				}
				if let.Type == nil || let.Type.Name != tt.typeName {
					t.Fatalf("wrong type for %s. got=%v want=%q", let.Name.Value, let.Type, tt.typeName)
				}
			}
		})
	}
}

func TestRejectImmutableTypedDeclarationWithoutInitializer(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: `int: a, b, c`, want: `immutable typed declaration requires initializer for "a" at 1:6`},
		{input: `let a: int`, want: `let declaration requires initializer for "a" at 1:5`},
		{input: `let mut a`, want: `let declaration requires initializer for "a" at 1:9`},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		p.ParseProgram()

		if len(p.Errors()) != 1 {
			t.Fatalf("wrong parser error count for %q. got=%d want=1 errors=%v", tt.input, len(p.Errors()), p.Errors())
		}
		if p.Errors()[0] != tt.want {
			t.Fatalf("wrong parser error for %q. got=%q want=%q", tt.input, p.Errors()[0], tt.want)
		}
	}
}

func TestParseOpenRangeContracts(t *testing.T) {
	input := `
type Max100 int range ..100
type Min0 int range 0..
type Celsius float range -273.1..
type NegativeMax int range ..-1
type Exclusive int range 1..<10
`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 5 {
		t.Fatalf("wrong statement count. got=%d want=5", len(program.Statements))
	}

	maxType, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement 0 is not TypeDeclStatement. got=%T", program.Statements[0])
	}
	maxRange, ok := maxType.Contract.(*ast.RangeContract)
	if !ok {
		t.Fatalf("expected RangeContract for Max100, got=%T", maxType.Contract)
	}
	if maxRange.Min != nil {
		t.Fatalf("expected nil min for Max100, got=%T", maxRange.Min)
	}
	assertLiteralValue(t, maxRange.Max, 100)

	minType, ok := program.Statements[1].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement 1 is not TypeDeclStatement. got=%T", program.Statements[1])
	}
	minRange, ok := minType.Contract.(*ast.RangeContract)
	if !ok {
		t.Fatalf("expected RangeContract for Min0, got=%T", minType.Contract)
	}
	assertLiteralValue(t, minRange.Min, 0)
	if minRange.Max != nil {
		t.Fatalf("expected nil max for Min0, got=%T", minRange.Max)
	}

	celsiusType, ok := program.Statements[2].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement 2 is not TypeDeclStatement. got=%T", program.Statements[2])
	}
	celsiusRange, ok := celsiusType.Contract.(*ast.RangeContract)
	if !ok {
		t.Fatalf("expected RangeContract for Celsius, got=%T", celsiusType.Contract)
	}
	assertPrefixExpression(t, celsiusRange.Min, "-", "273.1")
	if celsiusRange.Max != nil {
		t.Fatalf("expected nil max for Celsius, got=%T", celsiusRange.Max)
	}

	negativeMaxType, ok := program.Statements[3].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement 3 is not TypeDeclStatement. got=%T", program.Statements[3])
	}
	negativeMaxRange, ok := negativeMaxType.Contract.(*ast.RangeContract)
	if !ok {
		t.Fatalf("expected RangeContract for NegativeMax, got=%T", negativeMaxType.Contract)
	}
	if negativeMaxRange.Min != nil {
		t.Fatalf("expected nil min for NegativeMax, got=%T", negativeMaxRange.Min)
	}
	assertPrefixExpression(t, negativeMaxRange.Max, "-", "1")

	exclusiveType, ok := program.Statements[4].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement 4 is not TypeDeclStatement. got=%T", program.Statements[4])
	}
	exclusiveRange, ok := exclusiveType.Contract.(*ast.RangeContract)
	if !ok {
		t.Fatalf("expected RangeContract for Exclusive, got=%T", exclusiveType.Contract)
	}
	if !exclusiveRange.Exclusive {
		t.Fatal("expected exclusive range")
	}
	assertLiteralValue(t, exclusiveRange.Min, 1)
	assertLiteralValue(t, exclusiveRange.Max, 10)
}

func TestInvalidRangeOperatorReportsOneError(t *testing.T) {
	input := `type Range int range 1...`

	l := lexer.New(input)
	p := New(l)

	p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}

	expected := `expected range operator ('..' or '..<'), got "..." at 1:23`
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
}

func TestParseSliceTypeReferences(t *testing.T) {
	input := `
[]byte mut: data
Vec[[]byte] mut: chunks
struct Packet { payload: []byte }
type ByteSlice = []byte
`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 4 {
		t.Fatalf("wrong statement count. got=%d want=4", len(program.Statements))
	}

	letStmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("statement 0 is not LetStatement. got=%T", program.Statements[0])
	}
	assertSliceType(t, letStmt.Type, "byte")

	letStmt, ok = program.Statements[1].(*ast.LetStatement)
	if !ok {
		t.Fatalf("statement 1 is not LetStatement. got=%T", program.Statements[1])
	}
	if letStmt.Type.Name != "Vec" {
		t.Fatalf("wrong generic type name. got=%q want=%q", letStmt.Type.Name, "Vec")
	}
	if len(letStmt.Type.TypeArgs) != 1 {
		t.Fatalf("wrong type arg count. got=%d want=1", len(letStmt.Type.TypeArgs))
	}
	assertSliceType(t, letStmt.Type.TypeArgs[0], "byte")

	structStmt, ok := program.Statements[2].(*ast.StructStatement)
	if !ok {
		t.Fatalf("statement 2 is not StructStatement. got=%T", program.Statements[2])
	}
	if len(structStmt.Fields) != 1 {
		t.Fatalf("wrong field count. got=%d want=1", len(structStmt.Fields))
	}
	assertSliceType(t, structStmt.Fields[0].Type, "byte")

	typeDecl, ok := program.Statements[3].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement 3 is not TypeDeclStatement. got=%T", program.Statements[3])
	}
	assertSliceType(t, typeDecl.AssignedType, "byte")
}

func TestRejectIncompleteDeclarations(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`let bad:`, `let statement missing type after ':' at 1:8`},
		{`type Broken`, `type declaration missing base type after "Broken" at 1:12`},
		{`let u: uint = 5`, `let initializer must use ':=', got '=' at 1:13`},
	}

	for _, input := range tests {
		l := lexer.New(input.input)
		p := New(l)

		p.ParseProgram()

		if len(p.Errors()) == 0 {
			t.Fatalf("expected parser error for %q, got none", input.input)
		}

		if p.Errors()[0] != input.want {
			t.Fatalf("wrong parser error for %q. got=%q want=%q", input.input, p.Errors()[0], input.want)
		}
	}
}

func assertPrefixExpression(t *testing.T, expr ast.Expression, operator string, right string) {
	t.Helper()

	prefix, ok := expr.(*ast.PrefixExpression)
	if !ok {
		t.Fatalf("expected PrefixExpression, got=%T", expr)
	}

	if prefix.Operator != operator {
		t.Fatalf("wrong prefix operator. got=%q want=%q", prefix.Operator, operator)
	}

	if prefix.Right == nil {
		t.Fatal("expected prefix right expression, got nil")
	}

	if got := prefix.Right.String(); got != right {
		t.Fatalf("wrong prefix right expression. got=%q want=%q", got, right)
	}
}

func assertSliceType(t *testing.T, ref *ast.TypeReference, elementName string) {
	t.Helper()

	if ref == nil {
		t.Fatal("expected slice type, got nil")
	}

	if ref.ElementType == nil {
		t.Fatalf("expected slice element type, got %+v", ref)
	}

	if ref.ElementType.Name != elementName {
		t.Fatalf("wrong slice element type. got=%q want=%q", ref.ElementType.Name, elementName)
	}
}

func TestParseStructStatement(t *testing.T) {
	input := `struct Vehicle { _speed: Speed }`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
	}

	structStmt, ok := program.Statements[0].(*ast.StructStatement)
	if !ok {
		t.Fatalf("statement is not StructStatement. got=%T", program.Statements[0])
	}

	if structStmt.Name.Value != "Vehicle" {
		t.Fatalf("wrong struct name. got=%q want=%q", structStmt.Name.Value, "Vehicle")
	}

	if len(structStmt.Fields) != 1 {
		t.Fatalf("wrong field count. got=%d want=1", len(structStmt.Fields))
	}

	if structStmt.Fields[0].Name.Value != "_speed" {
		t.Fatalf("wrong field name. got=%q want=%q", structStmt.Fields[0].Name.Value, "_speed")
	}

	if structStmt.Fields[0].Type.Name != "Speed" {
		t.Fatalf("wrong field type. got=%q want=%q", structStmt.Fields[0].Type.Name, "Speed")
	}
}

func TestParseCommaSeparatedStructFields(t *testing.T) {
	tests := []string{
		`
type Coordinate struct {
	x: Meter,
	y: Meter,
	z: Meter,
}
`,
		`type Coordinate struct { x: Meter, y: Meter, z: Meter }`,
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			l := lexer.New(input)
			p := New(l)

			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
			}

			typeDecl, ok := program.Statements[0].(*ast.TypeDeclStatement)
			if !ok {
				t.Fatalf("statement is not TypeDeclStatement. got=%T", program.Statements[0])
			}

			if typeDecl.StructType == nil {
				t.Fatal("expected struct type")
			}

			if len(typeDecl.StructType.Fields) != 3 {
				t.Fatalf("wrong field count. got=%d want=3", len(typeDecl.StructType.Fields))
			}
		})
	}
}

func TestParseStructFieldsRequireCommas(t *testing.T) {
	input := `
type Bad struct {
	x: Meter
	y: Meter
}
`

	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()

	expected := "expected ',' or '}' after struct field at 4:2"
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
}

func TestParseStructFieldRangeContract(t *testing.T) {
	input := `
type User struct {
	Active: bool,
	Name: string,
	Age: int range 0..130,
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	typeDecl := program.Statements[0].(*ast.TypeDeclStatement)
	age := typeDecl.StructType.Fields[2]
	if age.Contract == nil {
		t.Fatal("expected field range contract")
	}
	rangeContract, ok := age.Contract.(*ast.RangeContract)
	if !ok {
		t.Fatalf("contract is not RangeContract. got=%T", age.Contract)
	}
	assertLiteralValue(t, rangeContract.Min, 0)
	assertLiteralValue(t, rangeContract.Max, 130)
}

func TestParseMalformedStructFieldMissingColonRecovery(t *testing.T) {
	input := `
type B struct {
	y int,
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	expected := "missing ':' after struct field name \"y\" at 3:2"
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
	}
}

func TestParseMalformedStructFieldContinuesAfterComma(t *testing.T) {
	input := `
type B struct {
	y int,
	z: int,
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	expected := "missing ':' after struct field name \"y\" at 3:2"
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}

	typeDecl, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement is not TypeDeclStatement. got=%T", program.Statements[0])
	}
	if len(typeDecl.StructType.Fields) != 1 {
		t.Fatalf("wrong field count. got=%d want=1", len(typeDecl.StructType.Fields))
	}
	if typeDecl.StructType.Fields[0].Name.Value != "z" {
		t.Fatalf("wrong recovered field. got=%q want=z", typeDecl.StructType.Fields[0].Name.Value)
	}
}

func TestParseContinuesAfterMalformedStructField(t *testing.T) {
	input := `
type B struct {
	y int,
}

type C struct {
	z: int,
}

let a := 10
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	expected := "missing ':' after struct field name \"y\" at 3:2"
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
	if len(program.Statements) != 3 {
		t.Fatalf("wrong statement count. got=%d want=3", len(program.Statements))
	}

	b, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok || b.Name.Value != "B" || b.StructType == nil {
		t.Fatalf("first statement is not struct type B. got=%T %+v", program.Statements[0], program.Statements[0])
	}

	c, ok := program.Statements[1].(*ast.TypeDeclStatement)
	if !ok || c.Name.Value != "C" || c.StructType == nil {
		t.Fatalf("second statement is not struct type C. got=%T %+v", program.Statements[1], program.Statements[1])
	}
	if len(c.StructType.Fields) != 1 || c.StructType.Fields[0].Name.Value != "z" {
		t.Fatalf("wrong C fields: %+v", c.StructType.Fields)
	}

	letStmt, ok := program.Statements[2].(*ast.LetStatement)
	if !ok || letStmt.Name.Value != "a" {
		t.Fatalf("third statement is not let a. got=%T %+v", program.Statements[2], program.Statements[2])
	}
}

func TestParseStructFieldTags(t *testing.T) {
	input := `
type User struct {
	ID: int ` + "`json:\"id\" xml:\"id\"`" + `,
	Name: string ` + "`json:\"name\"`" + `,
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	typeDecl := program.Statements[0].(*ast.TypeDeclStatement)
	fields := typeDecl.StructType.Fields
	if len(fields[0].Tags) != 2 {
		t.Fatalf("wrong tag count. got=%d want=2", len(fields[0].Tags))
	}
	if fields[0].Tags[0].Key != "json" || fields[0].Tags[0].Value != "id" {
		t.Fatalf("wrong first tag: %+v", fields[0].Tags[0])
	}
}

func TestParseInvalidStructFieldTag(t *testing.T) {
	input := "type User struct { ID: int `json:id`, }"

	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()

	expected := "invalid struct field tag"
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
}

func TestParseImplWithProperty(t *testing.T) {
	input := `
impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}

		try set value {
			_speed = value
		}
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	implStmt, ok := program.Statements[0].(*ast.ImplStatement)
	if !ok {
		t.Fatalf("statement is not ImplStatement. got=%T", program.Statements[0])
	}
	if implStmt.Target.Name != "Vehicle" {
		t.Fatalf("wrong impl target. got=%q want=Vehicle", implStmt.Target.Name)
	}
	if len(implStmt.Members) != 1 {
		t.Fatalf("wrong impl member count. got=%d want=1", len(implStmt.Members))
	}
	property := implStmt.Members[0].(*ast.PropertyDeclaration)
	if property.Name.Value != "TopSpeed" || property.Type.Name != "Speed" {
		t.Fatalf("wrong property: %+v", property)
	}
	if property.Getter == nil || property.Setter == nil || !property.Setter.Fallible {
		t.Fatalf("expected getter and fallible setter, got %+v", property)
	}
}

func TestParseImplWithNestedTypeAndEnum(t *testing.T) {
	input := `
impl Vehicle {
	type Engine struct {
		power: Kilowatt,
	}

	enum FuelType {
		petrol,
		diesel,
		electric,
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	implStmt, ok := program.Statements[0].(*ast.ImplStatement)
	if !ok {
		t.Fatalf("statement is not ImplStatement. got=%T", program.Statements[0])
	}
	if len(implStmt.Members) != 2 {
		t.Fatalf("wrong impl member count. got=%d want=2", len(implStmt.Members))
	}
	if _, ok := implStmt.Members[0].(*ast.TypeDeclStatement); !ok {
		t.Fatalf("first impl member is not TypeDeclStatement. got=%T", implStmt.Members[0])
	}
	enumDecl, ok := implStmt.Members[1].(*ast.EnumDeclaration)
	if !ok {
		t.Fatalf("second impl member is not EnumDeclaration. got=%T", implStmt.Members[1])
	}
	if enumDecl.Name.Value != "FuelType" || len(enumDecl.Values) != 3 {
		t.Fatalf("wrong enum declaration: %+v", enumDecl)
	}
}

func TestParseEnumWithUnderlyingTypeAndInitializers(t *testing.T) {
	input := `
enum Status int {
	unknown = 0,
	active = 10,
	paused,
	disabled = 99,
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	enumDecl, ok := program.Statements[0].(*ast.EnumDeclaration)
	if !ok {
		t.Fatalf("statement is not EnumDeclaration. got=%T", program.Statements[0])
	}
	if enumDecl.Name.Value != "Status" {
		t.Fatalf("wrong enum name. got=%q want=Status", enumDecl.Name.Value)
	}
	if enumDecl.UnderlyingType == nil || enumDecl.UnderlyingType.Name != "int" {
		t.Fatalf("wrong underlying type: %+v", enumDecl.UnderlyingType)
	}
	if len(enumDecl.Values) != 4 {
		t.Fatalf("wrong value count. got=%d want=4", len(enumDecl.Values))
	}
	if enumDecl.Values[0].Initializer == nil || enumDecl.Values[2].Initializer != nil {
		t.Fatalf("wrong enum initializers: %+v", enumDecl.Values)
	}
}

func TestParseFunctionDeclaration(t *testing.T) {
	input := `
fn add(a: int, b: int,) int {
	return a + b
}

fn noop() void {
	return
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("wrong statement count. got=%d want=2", len(program.Statements))
	}

	add, ok := program.Statements[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Fatalf("first statement is not FunctionDeclaration. got=%T", program.Statements[0])
	}
	if add.Name.Value != "add" || len(add.Parameters) != 2 || add.ReturnType.Name != "int" {
		t.Fatalf("wrong add function: %+v", add)
	}
	if len(add.Body.Statements) != 1 {
		t.Fatalf("wrong add body statement count. got=%d want=1", len(add.Body.Statements))
	}
	if _, ok := add.Body.Statements[0].(*ast.ReturnStatement); !ok {
		t.Fatalf("add body is not return statement. got=%T", add.Body.Statements[0])
	}

	noop := program.Statements[1].(*ast.FunctionDeclaration)
	if noop.Name.Value != "noop" || len(noop.Parameters) != 0 || noop.ReturnType.Name != "void" {
		t.Fatalf("wrong noop function: %+v", noop)
	}
	returnStmt := noop.Body.Statements[0].(*ast.ReturnStatement)
	if returnStmt.Value != nil {
		t.Fatalf("expected void return without value, got %+v", returnStmt.Value)
	}
}

func TestParseOkErrExpressions(t *testing.T) {
	input := `
fn Foo() Result[int, IOError] {
	return Ok(1)
}

fn Bar() Result[int, IOError] {
	return Err(IOError.InvalidValue)
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	foo := program.Statements[0].(*ast.FunctionDeclaration)
	fooReturn := foo.Body.Statements[0].(*ast.ReturnStatement)
	if _, ok := fooReturn.Value.(*ast.OkExpression); !ok {
		t.Fatalf("Foo return is not OkExpression. got=%T", fooReturn.Value)
	}

	bar := program.Statements[1].(*ast.FunctionDeclaration)
	barReturn := bar.Body.Statements[0].(*ast.ReturnStatement)
	if _, ok := barReturn.Value.(*ast.ErrExpression); !ok {
		t.Fatalf("Bar return is not ErrExpression. got=%T", barReturn.Value)
	}
}

func TestParseTryExpression(t *testing.T) {
	input := `
fn UseResult() Result[int, IOError] {
	let value := try Calculate()
	return Ok(value)
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	letStmt := fn.Body.Statements[0].(*ast.LetStatement)
	if _, ok := letStmt.Value.(*ast.TryExpression); !ok {
		t.Fatalf("let value is not TryExpression. got=%T", letStmt.Value)
	}
}

func TestParseTryExpressionHandlers(t *testing.T) {
	input := `
fn UseResult() Result[int, IOError] {
	let value := try Calculate() {
		Err(IOError.InvalidValue) => 0
		Err(error) => return Err(error)
	}
	return Ok(value)
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	letStmt := fn.Body.Statements[0].(*ast.LetStatement)
	tryExpr, ok := letStmt.Value.(*ast.TryExpression)
	if !ok {
		t.Fatalf("let value is not TryExpression. got=%T", letStmt.Value)
	}
	if len(tryExpr.Handlers) != 2 {
		t.Fatalf("wrong handler count. got=%d want=2", len(tryExpr.Handlers))
	}
	if tryExpr.Handlers[0].Body == nil {
		t.Fatal("first handler should have expression body")
	}
	if tryExpr.Handlers[1].ReturnBody == nil {
		t.Fatal("second handler should have return body")
	}
}

func TestParseTryHandlerBlockMissingClosingBraceRecovery(t *testing.T) {
	input := `
fn MissingClosingBrace() Speed {
	let speed := try ReadSpeed() {
		Err(error) => Speed(0)

	return speed
}
`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()

	expected := []string{
		`expected '}' after try handler block before "return" at 6:2`,
	}

	if len(p.Errors()) != len(expected) {
		t.Fatalf("wrong parser error count. got=%d want=%d errors=%v", len(p.Errors()), len(expected), p.Errors())
	}
	for i, want := range expected {
		if p.Errors()[i] != want {
			t.Fatalf("wrong parser error %d. got=%q want=%q", i, p.Errors()[i], want)
		}
	}

	if len(program.Statements) != 1 {
		t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
	}
	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("wrong function body statement count. got=%d want=2", len(fn.Body.Statements))
	}
	if _, ok := fn.Body.Statements[1].(*ast.ReturnStatement); !ok {
		t.Fatalf("second function body statement is not ReturnStatement. got=%T", fn.Body.Statements[1])
	}
}

func TestParseIfElseIfElseStatement(t *testing.T) {
	input := `
fn Grade(score: int) int {
	let mut result := 0
	if score >= 90 {
		result = 1
	} else if score >= 80 {
		result = 2
	} else {
		result = 3
	}
	return result
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	ifStmt, ok := fn.Body.Statements[1].(*ast.IfStatement)
	if !ok {
		t.Fatalf("statement is not IfStatement. got=%T", fn.Body.Statements[1])
	}
	if len(ifStmt.Consequence.Statements) != 1 {
		t.Fatalf("wrong then statement count. got=%d want=1", len(ifStmt.Consequence.Statements))
	}
	if ifStmt.Alternative == nil || len(ifStmt.Alternative.Statements) != 1 {
		t.Fatalf("expected else-if alternative")
	}
	if _, ok := ifStmt.Alternative.Statements[0].(*ast.IfStatement); !ok {
		t.Fatalf("else-if alternative is not IfStatement. got=%T", ifStmt.Alternative.Statements[0])
	}
}

func TestParseIfRangeMembershipCondition(t *testing.T) {
	input := `
fn Test(score: int) void {
	if score in 80..<100 {
	}
	return
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	ifStmt := fn.Body.Statements[0].(*ast.IfStatement)
	condition, ok := ifStmt.Condition.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("condition is not InfixExpression. got=%T", ifStmt.Condition)
	}
	if condition.Operator != "in" {
		t.Fatalf("wrong operator. got=%q want=in", condition.Operator)
	}
	rangeExpr, ok := condition.Right.(*ast.RangeExpression)
	if !ok {
		t.Fatalf("right side is not RangeExpression. got=%T", condition.Right)
	}
	if !rangeExpr.Exclusive {
		t.Fatal("range should be exclusive")
	}
}

func TestParseIfMissingConditionReportsOneError(t *testing.T) {
	input := `
fn MissingCondition() void {
	if {
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	expected := []string{
		"if statement missing condition at 3:5",
	}
	if len(p.Errors()) != len(expected) {
		t.Fatalf("wrong parser error count. got=%d want=%d errors=%v", len(p.Errors()), len(expected), p.Errors())
	}
	for i, want := range expected {
		if p.Errors()[i] != want {
			t.Fatalf("wrong parser error %d. got=%q want=%q", i, p.Errors()[i], want)
		}
	}

	if len(program.Statements) != 1 {
		t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
	}
	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if len(fn.Body.Statements) != 1 {
		t.Fatalf("wrong function body count. got=%d want=1", len(fn.Body.Statements))
	}
	ifStmt := fn.Body.Statements[0].(*ast.IfStatement)
	if ifStmt.Condition != nil {
		t.Fatalf("condition should be nil after recovery, got=%T", ifStmt.Condition)
	}
}

func TestParseInvalidIfFormsReportOneError(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "missing condition",
			input: `
fn MissingCondition() void {
	if {
	}
}
`,
			expected: "if statement missing condition at 3:5",
		},
		{
			name: "missing block",
			input: `
fn MissingBlock(value: bool) void {
	if value
}
`,
			expected: "expected '{' after if condition at 4:1",
		},
		{
			name: "missing closing brace",
			input: `
fn MissingClosingBrace(value: bool) void {
	if value {
}
`,
			expected: "unterminated function body",
		},
		{
			name: "else without if",
			input: `
fn ElseWithoutIf() void {
	else {
	}
}
`,
			expected: "else without matching if at 3:2",
		},
		{
			name: "else if without condition",
			input: `
fn ElseIfWithoutCondition(value: bool) void {
	if value {
	} else if {
	}
}
`,
			expected: "if statement missing condition at 4:12",
		},
		{
			name: "statement without braces",
			input: `
fn StatementWithoutBraces(value: bool) void {
	if value
		return
}
`,
			expected: "expected '{' after if condition at 4:3",
		},
		{
			name: "duplicate else",
			input: `
fn DuplicateElse(value: bool) void {
	if value {
	} else {
	} else {
	}
}
`,
			expected: "else without matching if at 5:4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()

			if len(p.Errors()) != 1 {
				t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
			}
			if p.Errors()[0] != tt.expected {
				t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], tt.expected)
			}
		})
	}
}

func TestParseCallExpressionStatement(t *testing.T) {
	input := `
fn ScopeTest(value: bool) void {
	if value {
		let local: int := 10
	}

	println(local)
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("wrong statement count. got=%d want=2", len(fn.Body.Statements))
	}
	if _, ok := fn.Body.Statements[1].(*ast.ExpressionStatement); !ok {
		t.Fatalf("second statement is not ExpressionStatement. got=%T", fn.Body.Statements[1])
	}
}

func TestParseImplRejectsLet(t *testing.T) {
	input := `
impl Vehicle {
	let x := 1
}
`

	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()

	expected := "impl block may only contain type, enum, property, and fn declarations at 3:2"
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
}

func TestParsePropertySetterMissingValueParameter(t *testing.T) {
	input := `
impl Vehicle {
	property Gustaf: Speed {
		set {
		}
	}
}
`

	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()

	expected := "setter for Gustaf must declare value parameter at 4:7"
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
}

func TestParseInvalidPropertyDeclarationRecovery(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "missing colon after property name",
			input: `
impl Vehicle {
	property NoType {
		get {
			return _speed
		}
	}
}
`,
			expected: "expected ':' after property name NoType at 3:18",
		},
		{
			name: "missing property name",
			input: `
impl Vehicle {
	property {
	}
}
`,
			expected: "property declaration missing name at 3:11",
		},
		{
			name: "missing property type",
			input: `
impl Vehicle {
	property Name: {
	}
}
`,
			expected: "property Name missing type after ':' at 3:17",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			p.ParseProgram()

			if len(p.Errors()) != 1 {
				t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
			}
			if p.Errors()[0] != tt.expected {
				t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], tt.expected)
			}
		})
	}
}

func TestParseStructLiteralAndMemberAccess(t *testing.T) {
	input := `
let speed := Speed(10)
let vehicle := Vehicle{ _speed: speed }
let current := vehicle._speed
vehicle.TopSpeed = speed
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 4 {
		t.Fatalf("wrong statement count. got=%d want=4", len(program.Statements))
	}

	letStmt := program.Statements[1].(*ast.LetStatement)
	lit, ok := letStmt.Value.(*ast.StructLiteral)
	if !ok {
		t.Fatalf("expected StructLiteral, got=%T", letStmt.Value)
	}
	if lit.Type.Name != "Vehicle" || len(lit.Fields) != 1 {
		t.Fatalf("wrong struct literal: %+v", lit)
	}

	memberLet := program.Statements[2].(*ast.LetStatement)
	if _, ok := memberLet.Value.(*ast.MemberExpression); !ok {
		t.Fatalf("expected MemberExpression, got=%T", memberLet.Value)
	}

	assign := program.Statements[3].(*ast.AssignmentStatement)
	if assign.Target.String() != "vehicle.TopSpeed" {
		t.Fatalf("wrong assignment target. got=%q", assign.Target.String())
	}
}

func TestParseTypeDeclWithStructAndVariants(t *testing.T) {
	input := `
type FileReader struct { handle: void }
type IOError = FileNotFound AccessDenied InvalidValue
`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("wrong statement count. got=%d want=2", len(program.Statements))
	}

	typeDecl, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement 0 is not TypeDeclStatement. got=%T", program.Statements[0])
	}

	if typeDecl.Name.Value != "FileReader" {
		t.Fatalf("wrong type name. got=%q want=%q", typeDecl.Name.Value, "FileReader")
	}

	if typeDecl.StructType == nil {
		t.Fatal("expected StructType for FileReader")
	}

	if len(typeDecl.StructType.Fields) != 1 {
		t.Fatalf("wrong struct field count. got=%d want=1", len(typeDecl.StructType.Fields))
	}

	if typeDecl.StructType.Fields[0].Name.Value != "handle" {
		t.Fatalf("wrong struct field name. got=%q want=%q", typeDecl.StructType.Fields[0].Name.Value, "handle")
	}

	if typeDecl.StructType.Fields[0].Type.Name != "void" {
		t.Fatalf("wrong struct field type. got=%q want=%q", typeDecl.StructType.Fields[0].Type.Name, "void")
	}

	typeDecl, ok = program.Statements[1].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement 1 is not TypeDeclStatement. got=%T", program.Statements[1])
	}

	if typeDecl.Name.Value != "IOError" {
		t.Fatalf("wrong type name. got=%q want=%q", typeDecl.Name.Value, "IOError")
	}

	if typeDecl.AssignedType != nil {
		t.Fatalf("expected AssignedType to be nil for variant type, got=%T", typeDecl.AssignedType)
	}

	if len(typeDecl.Variants) != 3 {
		t.Fatalf("wrong variant count. got=%d want=3", len(typeDecl.Variants))
	}

	if typeDecl.Variants[0].Value != "FileNotFound" || typeDecl.Variants[1].Value != "AccessDenied" || typeDecl.Variants[2].Value != "InvalidValue" {
		t.Fatalf("wrong variant names. got=%v", []string{typeDecl.Variants[0].Value, typeDecl.Variants[1].Value, typeDecl.Variants[2].Value})
	}
}

func TestParseTypesFile(t *testing.T) {
	data, err := os.ReadFile("../../testdata/types.sec")
	if err != nil {
		t.Fatal(err)
	}

	l := lexer.New(string(data))
	p := New(l)

	program := p.ParseProgram()

	expectedErrors := []string{}
	if len(p.Errors()) != len(expectedErrors) {
		t.Fatalf("wrong parser error count. got=%d want=%d errors=%v", len(p.Errors()), len(expectedErrors), p.Errors())
	}
	for i, expected := range expectedErrors {
		if p.Errors()[i] != expected {
			t.Fatalf("wrong parser error %d. got=%q want=%q", i, p.Errors()[i], expected)
		}
	}

	if len(program.Statements) != 43 {
		t.Fatalf("expected 43 statements, got %d", len(program.Statements))
	}
}
