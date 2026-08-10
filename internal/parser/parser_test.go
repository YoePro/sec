package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
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

func TestParseGroupedImportsWithAliases(t *testing.T) {
	input := `
module main

import (
    "fmt"
    "platform/linux/amd64"
    sys "platform/linux"
)
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 4 {
		t.Fatalf("wrong statement count. got=%d want=4", len(program.Statements))
	}
	wants := []struct {
		path  string
		alias string
	}{
		{path: "fmt"},
		{path: "platform/linux/amd64"},
		{path: "platform/linux", alias: "sys"},
	}
	for i, want := range wants {
		importStmt, ok := program.Statements[i+1].(*ast.ImportStatement)
		if !ok {
			t.Fatalf("statement %d is not ImportStatement. got=%T", i+1, program.Statements[i+1])
		}
		if importStmt.Path != want.path || importStmt.Alias != want.alias {
			t.Fatalf("wrong grouped import %d: %+v", i, importStmt)
		}
	}
}

func TestParseGenericTypeDeclarations(t *testing.T) {
	input := `
type Stack[T] struct {
	value: T,
}

type Pair[A, B] struct {
	first: A,
	second: B,
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stack := program.Statements[0].(*ast.TypeDeclStatement)
	if stack.Name.Value != "Stack" || len(stack.GenericParameters) != 1 {
		t.Fatalf("wrong Stack generics: %+v", stack.GenericParameters)
	}
	if stack.GenericParameters[0].Name.Value != "T" {
		t.Fatalf("wrong Stack parameter: %+v", stack.GenericParameters[0])
	}
	if stack.StructType.Fields[0].Type.Name != "T" {
		t.Fatalf("wrong Stack field type: %+v", stack.StructType.Fields[0].Type)
	}

	pair := program.Statements[1].(*ast.TypeDeclStatement)
	if pair.Name.Value != "Pair" || len(pair.GenericParameters) != 2 {
		t.Fatalf("wrong Pair generics: %+v", pair.GenericParameters)
	}
	if pair.GenericParameters[0].Name.Value != "A" || pair.GenericParameters[1].Name.Value != "B" {
		t.Fatalf("wrong Pair parameters: %+v", pair.GenericParameters)
	}
}

func TestParseUnitDeclarations(t *testing.T) {
	input := `
unit Hertz decimal<Hz>
unit Packet uint other
unit Metre decimal physical
unit Count
unit Euro currency
unit Horsepower float physical
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 6 {
		t.Fatalf("wrong statement count. got=%d want=6", len(program.Statements))
	}

	hertz, ok := program.Statements[0].(*ast.UnitDeclStatement)
	if !ok {
		t.Fatalf("statement 0 is not UnitDeclStatement. got=%T", program.Statements[0])
	}
	if hertz.Name.Value != "Hertz" || hertz.BaseType.Name != "decimal" || hertz.BaseType.Unit != "Hz" {
		t.Fatalf("wrong Hertz unit declaration: %+v", hertz)
	}

	packet, ok := program.Statements[1].(*ast.UnitDeclStatement)
	if !ok {
		t.Fatalf("statement 1 is not UnitDeclStatement. got=%T", program.Statements[1])
	}
	if packet.Name.Value != "Packet" || packet.BaseType.Name != "uint" || packet.BaseType.Unit != "" || packet.Category != "other" {
		t.Fatalf("wrong Packet unit declaration: %+v", packet)
	}

	metre, ok := program.Statements[2].(*ast.UnitDeclStatement)
	if !ok {
		t.Fatalf("statement 2 is not UnitDeclStatement. got=%T", program.Statements[2])
	}
	if metre.Name.Value != "Metre" || metre.BaseType.Name != "decimal" || metre.Category != "physical" {
		t.Fatalf("wrong Metre unit declaration: %+v", metre)
	}

	count, ok := program.Statements[3].(*ast.UnitDeclStatement)
	if !ok {
		t.Fatalf("statement 3 is not UnitDeclStatement. got=%T", program.Statements[3])
	}
	if count.Name.Value != "Count" || count.BaseType.Name != "decimal" || count.Category != "" {
		t.Fatalf("wrong Count unit declaration: %+v", count)
	}

	euro, ok := program.Statements[4].(*ast.UnitDeclStatement)
	if !ok {
		t.Fatalf("statement 4 is not UnitDeclStatement. got=%T", program.Statements[4])
	}
	if euro.Name.Value != "Euro" || euro.BaseType.Name != "decimal" || euro.Category != "currency" {
		t.Fatalf("wrong Euro unit declaration: %+v", euro)
	}

	horsepower, ok := program.Statements[5].(*ast.UnitDeclStatement)
	if !ok {
		t.Fatalf("statement 5 is not UnitDeclStatement. got=%T", program.Statements[5])
	}
	if horsepower.Name.Value != "Horsepower" || horsepower.BaseType.Name != "float" || horsepower.Category != "physical" {
		t.Fatalf("wrong Horsepower unit declaration: %+v", horsepower)
	}
}

func TestParseRegisterTypeDeclaration(t *testing.T) {
	input := `
type MotorProtocol register[8] {
	Speed: bit[4]<rpm>,
	Enabled: bit,
	_: bit[3],
}

@address(0x40021000)
let mut motorProtocol: MotorProtocol
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("wrong statement count. got=%d want=2", len(program.Statements))
	}

	typeDecl := program.Statements[0].(*ast.TypeDeclStatement)
	if typeDecl.RegisterType == nil {
		t.Fatal("expected register type")
	}
	if typeDecl.RegisterType.Width != 8 {
		t.Fatalf("wrong register width. got=%d want=8", typeDecl.RegisterType.Width)
	}
	if len(typeDecl.RegisterType.Fields) != 3 {
		t.Fatalf("wrong register field count. got=%d want=3", len(typeDecl.RegisterType.Fields))
	}
	if reserved := typeDecl.RegisterType.Fields[2]; reserved.Name.Value != "_" || reserved.Width != 3 {
		t.Fatalf("wrong reserved field: %+v", reserved)
	}
	speed := typeDecl.RegisterType.Fields[0]
	if speed.Name.Value != "Speed" || speed.Width != 4 || speed.Unit != "rpm" {
		t.Fatalf("wrong Speed field: %+v", speed)
	}

	letStmt := program.Statements[1].(*ast.LetStatement)
	if letStmt.Address == nil || letStmt.Address.String() != "0x40021000" {
		t.Fatalf("wrong address: %#v", letStmt.Address)
	}
	if !letStmt.Mutable {
		t.Fatal("addressed let should be mutable")
	}
}

func TestParseBitBackedEnumAndRegisterField(t *testing.T) {
	input := `
enum ClockSource bit[2] {
	Internal = 0b00,
	External = 0b01,
	Bypass = 0b10
}

type ClockConfig register[32] {
	Source: ClockSource // enum-backed field
	Enabled: bit        // one bit
	_: bit[29]          // reserved bits
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	enumDecl, ok := program.Statements[0].(*ast.EnumDeclaration)
	if !ok {
		t.Fatalf("statement 0 is not EnumDeclaration. got=%T", program.Statements[0])
	}
	if !enumDecl.BitUnderlying || enumDecl.UnderlyingBitWidth != 2 || enumDecl.UnderlyingType != nil {
		t.Fatalf("wrong bit enum underlying type: %+v", enumDecl)
	}

	typeDecl := program.Statements[1].(*ast.TypeDeclStatement)
	source := typeDecl.RegisterType.Fields[0]
	if source.Type == nil || source.Type.Name != "ClockSource" || source.Width != 0 {
		t.Fatalf("wrong enum-backed register field: %+v", source)
	}
	if typeDecl.RegisterType.Fields[1].Width != 1 || typeDecl.RegisterType.Fields[2].Width != 29 {
		t.Fatalf("wrong ordinary bit fields: %+v", typeDecl.RegisterType.Fields)
	}
}

func TestParseEnumColonInitializersWarnForFormatterRecovery(t *testing.T) {
	input := `
enum ClockSource bit[2] {
	Internal: 0b00,
	External: 0b01,
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	enumDecl, ok := program.Statements[0].(*ast.EnumDeclaration)
	if !ok {
		t.Fatalf("statement 0 is not EnumDeclaration. got=%T", program.Statements[0])
	}
	if len(enumDecl.Values) != 2 || enumDecl.Values[0].Initializer == nil || enumDecl.Values[1].Initializer == nil {
		t.Fatalf("colon initializers should be parsed for formatter recovery: %+v", enumDecl.Values)
	}
	expected := []string{
		"enum initializer ':' is non-canonical; sec fmt will rewrite it to '=' at 3:10",
		"enum initializer ':' is non-canonical; sec fmt will rewrite it to '=' at 4:10",
	}
	if len(p.Warnings()) != len(expected) {
		t.Fatalf("wrong parser warning count. got=%d want=%d warnings=%v", len(p.Warnings()), len(expected), p.Warnings())
	}
	for i, want := range expected {
		if p.Warnings()[i] != want {
			t.Fatalf("wrong parser warning %d. got=%q want=%q", i, p.Warnings()[i], want)
		}
	}
}

func TestParseSelfKeywordInDiscardStatement(t *testing.T) {
	input := `
fn Test() void {
	discard self
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	discard, ok := fn.Body.Statements[0].(*ast.DiscardStatement)
	if !ok {
		t.Fatalf("statement is not DiscardStatement. got=%T", fn.Body.Statements[0])
	}
	if discard.Name.Value != "self" {
		t.Fatalf("wrong discard name. got=%q want=%q", discard.Name.Value, "self")
	}
	if discard.Value == nil || discard.Value.String() != "self" {
		t.Fatalf("wrong discard value. got=%#v", discard.Value)
	}
}

func TestParseGenericFunctionDeclaration(t *testing.T) {
	input := `
fn Identity[T](value: T) T {
	return value
}

fn Save[T: Serializable](value: T) Result[void, IOError] {
	return Ok()
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	identity := program.Statements[0].(*ast.FunctionDeclaration)
	if identity.Name.Value != "Identity" || len(identity.GenericParameters) != 1 {
		t.Fatalf("wrong Identity generics: %+v", identity.GenericParameters)
	}
	if identity.GenericParameters[0].Name.Value != "T" {
		t.Fatalf("wrong Identity parameter: %+v", identity.GenericParameters[0])
	}
	if identity.Parameters[0].Type.Name != "T" || identity.ReturnType.Name != "T" {
		t.Fatalf("generic type parameter not used in signature: params=%+v return=%+v", identity.Parameters, identity.ReturnType)
	}

	save := program.Statements[1].(*ast.FunctionDeclaration)
	if len(save.GenericParameters) != 1 {
		t.Fatalf("wrong Save generics: %+v", save.GenericParameters)
	}
	constraint := save.GenericParameters[0].Constraint
	if constraint == nil || constraint.Name != "Serializable" {
		t.Fatalf("wrong Save constraint: %+v", constraint)
	}
}

func TestParseExplicitGenericCallExpression(t *testing.T) {
	input := `
fn Test() void {
	let value := Identity[int](10)
	let other := pkg.Make[Box[string]]("hello")
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	first := fn.Body.Statements[0].(*ast.LetStatement)
	call, ok := first.Value.(*ast.CallExpression)
	if !ok {
		t.Fatalf("first value is not CallExpression. got=%T", first.Value)
	}
	if call.Function.Value != "Identity" || len(call.GenericArguments) != 1 || call.GenericArguments[0].Name != "int" {
		t.Fatalf("wrong explicit generic call: %+v", call)
	}

	second := fn.Body.Statements[1].(*ast.LetStatement)
	memberCall, ok := second.Value.(*ast.CallExpression)
	if !ok {
		t.Fatalf("second value is not CallExpression. got=%T", second.Value)
	}
	if memberCall.String() != `pkg.Make[Box]("hello")` {
		t.Fatalf("wrong member generic call string: %s", memberCall.String())
	}
	if len(memberCall.GenericArguments) != 1 || memberCall.GenericArguments[0].Name != "Box" || len(memberCall.GenericArguments[0].TypeArgs) != 1 {
		t.Fatalf("wrong member generic arguments: %+v", memberCall.GenericArguments)
	}
}

func TestParseTargetDirective(t *testing.T) {
	input := `#target(os: "linux", arch: "amd64")
module main
`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("wrong statement count. got=%d want=2", len(program.Statements))
	}

	target, ok := program.Statements[0].(*ast.TargetDirective)
	if !ok {
		t.Fatalf("statement 0 is not TargetDirective. got=%T", program.Statements[0])
	}
	if target.OS != "linux" || target.Arch != "amd64" {
		t.Fatalf("wrong target. got=%s/%s", target.OS, target.Arch)
	}
}

func TestParseNoCopyNominalTypeAttributes(t *testing.T) {
	input := `@noCopy
// The comment remains attached to the declaration in source.
type SessionID struct {
    value: uint64,
}

@noCopy
enum State {
    Ready,
}

@noCopy
type Code int

@noCopy
type Flags register[8] {
    Enabled: bit,
    _: bit[7],
}

@noCopy
type Choice union {
    None,
    Some(int),
}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 5 {
		t.Fatalf("wrong statement count. got=%d want=5", len(program.Statements))
	}
	for i, stmt := range program.Statements {
		var attributes []*ast.Attribute
		switch stmt := stmt.(type) {
		case *ast.TypeDeclStatement:
			attributes = stmt.Attributes
		case *ast.EnumDeclaration:
			attributes = stmt.Attributes
		default:
			t.Fatalf("statement %d cannot carry @noCopy. got=%T", i, stmt)
		}
		if len(attributes) != 1 || attributes[0].Name == nil || attributes[0].Name.Value != "noCopy" {
			t.Fatalf("statement %d has wrong attributes: %+v", i, attributes)
		}
	}
}

func TestParseNoCopyAttributeErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "arguments",
			input: "@noCopy()\ntype ID int\n",
			want:  "@noCopy does not take arguments at 1:8",
		},
		{
			name:  "duplicate",
			input: "@noCopy\n@noCopy\ntype ID int\n",
			want:  "duplicate attribute @noCopy at 2:1; first declared at 1:1",
		},
		{
			name:  "wrong target",
			input: "@noCopy\nfn Work() void {}\n",
			want:  "@noCopy may only annotate a nominal type declaration at 2:1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(lexer.New(tt.input))
			p.ParseProgram()
			if len(p.Errors()) != 1 || p.Errors()[0] != tt.want {
				t.Fatalf("parser errors = %v, want %q", p.Errors(), tt.want)
			}
		})
	}
}

func TestTargetDirectiveMustBeFirst(t *testing.T) {
	input := `module main
#target(os: "linux", arch: "amd64")
`

	l := lexer.New(input)
	p := New(l)

	p.ParseProgram()

	expected := `#target directive must appear before any code or declarations at 2:1`
	if len(p.Errors()) != 1 || p.Errors()[0] != expected {
		t.Fatalf("wrong errors. got=%v want=%q", p.Errors(), expected)
	}
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

func TestParseModuleNameAllowsSnakeCamelAndPlainText(t *testing.T) {
	input := `
module plain
module snake_case
module camelCase
module raspberry_Matter
module i2c_sensor_driver
`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	expected := []string{"plain", "snake_case", "camelCase", "raspberry_Matter", "i2c_sensor_driver"}
	if len(program.Statements) != len(expected) {
		t.Fatalf("wrong statement count. got=%d want=%d", len(program.Statements), len(expected))
	}
	for i, want := range expected {
		stmt, ok := program.Statements[i].(*ast.ModuleStatement)
		if !ok {
			t.Fatalf("statement %d is not ModuleStatement. got=%T", i, program.Statements[i])
		}
		if stmt.Path != want {
			t.Fatalf("module %d path = %q, want %q", i, stmt.Path, want)
		}
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
	if len(p.Diagnostics()) != len(errors) {
		t.Fatalf("structured diagnostics = %d, compatibility errors = %d", len(p.Diagnostics()), len(errors))
	}
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

func TestMalformedScientificExponentHasFocusedDiagnostic(t *testing.T) {
	p := New(lexer.New(`let value := 1e+`))
	p.ParseProgram()

	expected := `malformed scientific exponent "1e+": expected at least one decimal digit at 1:14`
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
}

func TestParseExplicitMoveOwnership(t *testing.T) {
	input := `let inferred :<- source
let typed: int <- other
target <- replacement`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 3 {
		t.Fatalf("wrong statement count. got=%d want=3", len(program.Statements))
	}

	inferred, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("first statement is not LetStatement. got=%T", program.Statements[0])
	}
	if inferred.Ownership != ast.OwnershipMove || inferred.Type != nil || inferred.Value.String() != "source" {
		t.Fatalf("wrong inferred move declaration: ownership=%q type=%T value=%v", inferred.Ownership, inferred.Type, inferred.Value)
	}

	typed, ok := program.Statements[1].(*ast.LetStatement)
	if !ok {
		t.Fatalf("second statement is not LetStatement. got=%T", program.Statements[1])
	}
	if typed.Ownership != ast.OwnershipMove || typed.Type == nil || typed.Value.String() != "other" {
		t.Fatalf("wrong typed move declaration: ownership=%q type=%T value=%v", typed.Ownership, typed.Type, typed.Value)
	}

	assignment, ok := program.Statements[2].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("third statement is not AssignmentStatement. got=%T", program.Statements[2])
	}
	if assignment.Ownership != ast.OwnershipMove || assignment.Operator != "<-" || assignment.Value.String() != "replacement" {
		t.Fatalf("wrong move assignment: ownership=%q operator=%q value=%v", assignment.Ownership, assignment.Operator, assignment.Value)
	}
}

func TestMoveDeclarationsRequireCanonicalOperator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "inferred declaration",
			input:    `let inferred <- source`,
			expected: "inferred move initializer must use ':<-'",
		},
		{
			name:     "typed declaration",
			input:    `let typed: int :<- source`,
			expected: "typed move initializer must use '<-'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(lexer.New(tt.input))
			p.ParseProgram()
			if len(p.Errors()) != 1 {
				t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
			}
			if !strings.Contains(p.Errors()[0], tt.expected) {
				t.Fatalf("wrong parser error. got=%q want substring=%q", p.Errors()[0], tt.expected)
			}
		})
	}
}

func TestParseCharAndRuneNumericSuffixes(t *testing.T) {
	input := `let ch: char := 65t
let ru: rune := 0x41r`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("wrong statement count. got=%d want=2", len(program.Statements))
	}

	for index, suffix := range []string{"t", "r"} {
		stmt, ok := program.Statements[index].(*ast.LetStatement)
		if !ok {
			t.Fatalf("statement %d is not LetStatement. got=%T", index, program.Statements[index])
		}
		literal, ok := stmt.Value.(*ast.IntegerLiteral)
		if !ok {
			t.Fatalf("statement %d has wrong initializer. got=%T", index, stmt.Value)
		}
		if literal.Suffix() != suffix {
			t.Fatalf("statement %d: wrong suffix. got=%q want=%q", index, literal.Suffix(), suffix)
		}
	}
}

func TestLegacyNumericSuffixMigrationDiagnostics(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"65c", "literal suffix 'c' was replaced by 't' for char at 1:1"},
		{"1.5d", "literal suffix 'd' was replaced by 'm' for decimal at 1:1"},
		{"1e3f", "literal suffix 'f' was replaced by 'g' for binary float at 1:1"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			p := New(lexer.New(test.input))
			p.ParseProgram()
			if len(p.Errors()) == 0 || !strings.Contains(p.Errors()[0], test.want) {
				t.Fatalf("errors = %v, want %q", p.Errors(), test.want)
			}
		})
	}
}

func TestParseNumericDigitSeparatorsPreservesLexemesAndValues(t *testing.T) {
	input := `let decimal := 1_000_000
let binary := 0b1111_0000
let hex := 0xFFFF_FFFF
let fraction := 1_234.5_678`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	if len(program.Statements) != 4 {
		t.Fatalf("wrong statement count. got=%d want=4", len(program.Statements))
	}

	wantIntegers := []struct {
		lexeme string
		value  int64
	}{
		{"1_000_000", 1_000_000},
		{"0b1111_0000", 240},
		{"0xFFFF_FFFF", 4_294_967_295},
	}
	for index, want := range wantIntegers {
		stmt := program.Statements[index].(*ast.LetStatement)
		literal := stmt.Value.(*ast.IntegerLiteral)
		if literal.Token.Lexeme != want.lexeme || literal.BigValue.Int64() != want.value {
			t.Fatalf("statement %d = %q/%s, want %q/%d", index, literal.Token.Lexeme, literal.BigValue, want.lexeme, want.value)
		}
	}
	stmt := program.Statements[3].(*ast.LetStatement)
	fraction := stmt.Value.(*ast.FloatLiteral)
	if fraction.Token.Lexeme != "1_234.5_678" || fraction.Value != 1234.5678 {
		t.Fatalf("fraction = %q/%v, want preserved lexeme and value 1234.5678", fraction.Token.Lexeme, fraction.Value)
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

func TestRejectTryAssignmentWithoutHandlerBlock(t *testing.T) {
	l := lexer.New(`try p += 1`)
	p := New(l)

	p.ParseProgram()

	expected := `try assignment requires a handler block at 1:1`
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d want=1 errors=%v", len(p.Errors()), p.Errors())
	}
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
}

func TestParseTryAssignmentHandlers(t *testing.T) {
	input := `
try car.TopSpeed = current_speed {
	Err(error) => fmt.println("failed")
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt, ok := program.Statements[0].(*ast.TryAssignmentStatement)
	if !ok {
		t.Fatalf("statement is not TryAssignmentStatement. got=%T", program.Statements[0])
	}
	if stmt.Assignment == nil {
		t.Fatal("expected nested assignment")
	}
	if len(stmt.Handlers) != 1 {
		t.Fatalf("wrong handler count. got=%d want=1", len(stmt.Handlers))
	}
	if stmt.Handlers[0].Body == nil {
		t.Fatal("expected expression handler body")
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
		{input: `TokenType (
ILLEGAL := "ILLEGAL",
EOF := "EOF",
IDENT := "IDENT",
)`, count: 3, mutable: false, typeName: "TokenType"},
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
		{input: `TokenType (
ILLEGAL
)`, want: `immutable typed declaration requires initializer for "ILLEGAL" at 2:1`},
		{input: `int mut a, b, c`, want: `typed mutable declaration requires ':' after mut; write int mut: a at 1:9`},
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

func TestCallExpressionIsNotParsedAsTypedDeclarationGroup(t *testing.T) {
	l := lexer.New(`Foo(bar)`)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
	}
	if _, ok := program.Statements[0].(*ast.ExpressionStatement); !ok {
		t.Fatalf("statement is not ExpressionStatement. got=%T", program.Statements[0])
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
ref byte[] mut: data
Vec[byte[]] mut: chunks
struct Packet { payload: ref byte[] }
type ByteSlice = ref byte[]
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
	if !letStmt.Type.Ref {
		t.Fatalf("expected slice variable type to be ref")
	}

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

func TestParseFixedArrayTypeReference(t *testing.T) {
	input := `
fn ArrayLoop(values: int[3]) void {
	for value in values {
		let copy: int := value
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	paramType := fn.Parameters[0].Type
	if paramType.ElementType == nil {
		t.Fatalf("expected array element type, got %+v", paramType)
	}
	if paramType.ArrayLength != 3 {
		t.Fatalf("wrong array length. got=%d want=3", paramType.ArrayLength)
	}
	if paramType.ElementType.Name != "int" {
		t.Fatalf("wrong array element type. got=%q want=int", paramType.ElementType.Name)
	}
}

func TestParseFixedArrayTypeReferenceWithHexLength(t *testing.T) {
	input := `fn Test(values: int[0x3]) void {}`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	paramType := fn.Parameters[0].Type
	if paramType.ArrayLength != 3 {
		t.Fatalf("wrong array length. got=%d want=3", paramType.ArrayLength)
	}
}

func TestParseNestedPostfixArrayTypeReference(t *testing.T) {
	input := `fn Matrix(values: int[3][4]) void {}`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	paramType := fn.Parameters[0].Type
	if paramType.ArrayLength != 3 {
		t.Fatalf("wrong outer array length. got=%d want=3", paramType.ArrayLength)
	}
	if paramType.ElementType == nil || paramType.ElementType.ArrayLength != 4 {
		t.Fatalf("wrong inner array type. got=%+v", paramType.ElementType)
	}
	if paramType.ElementType.ElementType == nil || paramType.ElementType.ElementType.Name != "int" {
		t.Fatalf("wrong nested array element. got=%+v", paramType.ElementType)
	}
}

func TestParseContractSequence(t *testing.T) {
	input := `
type PageSize int range 10..100 multipleOf 10
type OddNumber int odd
type EvenNumber int even
type Role string in ["admin", "user", "guest"]
type Tags string[] notEmpty unique
type Measurement float finite
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	pageSize := program.Statements[0].(*ast.TypeDeclStatement)
	pageContracts, ok := pageSize.Contract.(*ast.ContractList)
	if !ok || len(pageContracts.Contracts) != 2 {
		t.Fatalf("wrong PageSize contracts: %#v", pageSize.Contract)
	}
	if _, ok := pageContracts.Contracts[0].(*ast.RangeContract); !ok {
		t.Fatalf("first PageSize contract is not range: %#v", pageContracts.Contracts[0])
	}
	multiple, ok := pageContracts.Contracts[1].(*ast.MarkerContract)
	if !ok || multiple.Name != "multipleOf" || multiple.Value.String() != "10" {
		t.Fatalf("wrong multipleOf contract: %#v", pageContracts.Contracts[1])
	}

	odd := program.Statements[1].(*ast.TypeDeclStatement)
	oddContract, ok := odd.Contract.(*ast.MarkerContract)
	if !ok || oddContract.Name != "odd" {
		t.Fatalf("wrong OddNumber contract: %#v", odd.Contract)
	}

	even := program.Statements[2].(*ast.TypeDeclStatement)
	evenContract, ok := even.Contract.(*ast.MarkerContract)
	if !ok || evenContract.Name != "even" {
		t.Fatalf("wrong EvenNumber contract: %#v", even.Contract)
	}

	role := program.Statements[3].(*ast.TypeDeclStatement)
	membership, ok := role.Contract.(*ast.MembershipContract)
	if !ok || len(membership.Values) != 3 {
		t.Fatalf("wrong Role contract: %#v", role.Contract)
	}

	tags := program.Statements[4].(*ast.TypeDeclStatement)
	tagContracts, ok := tags.Contract.(*ast.ContractList)
	if !ok || len(tagContracts.Contracts) != 2 {
		t.Fatalf("wrong Tags contracts: %#v", tags.Contract)
	}
	for i, name := range []string{"notEmpty", "unique"} {
		marker, ok := tagContracts.Contracts[i].(*ast.MarkerContract)
		if !ok || marker.Name != name {
			t.Fatalf("wrong Tags contract %d: %#v", i, tagContracts.Contracts[i])
		}
	}

	measurement := program.Statements[5].(*ast.TypeDeclStatement)
	finite, ok := measurement.Contract.(*ast.MarkerContract)
	if !ok || finite.Name != "finite" {
		t.Fatalf("wrong Measurement contract: %#v", measurement.Contract)
	}
}

func TestParseLetVariableContract(t *testing.T) {
	input := `let mut percentage: int range 0..100 := 50`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	letStmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("statement is not LetStatement. got=%T", program.Statements[0])
	}
	if letStmt.Contract == nil {
		t.Fatal("expected let contract")
	}
	if _, ok := letStmt.Contract.(*ast.RangeContract); !ok {
		t.Fatalf("let contract is not RangeContract. got=%T", letStmt.Contract)
	}
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

func TestParseContextualMatrixMultiplyPrecedence(t *testing.T) {
	input := `
fn Test(a: int, b: int, c: int, d: int) int {
	let x := 10
	return a + b x c x d
}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	let := fn.Body.Statements[0].(*ast.LetStatement)
	if let.Name.Value != "x" {
		t.Fatalf("ordinary x binding was not preserved: %+v", let.Name)
	}
	ret := fn.Body.Statements[1].(*ast.ReturnStatement)
	if got, want := ret.Value.String(), "(a + ((b x c) x d))"; got != want {
		t.Fatalf("matrix multiplication precedence = %q, want %q", got, want)
	}
}

func TestContextualXDoesNotConsumeFollowingAssignment(t *testing.T) {
	input := `
fn Test(a: int, b: int) void {
	discard a
	x = b
}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("statement count = %d, want 2", len(fn.Body.Statements))
	}
	assignment, ok := fn.Body.Statements[1].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("second statement = %T, want AssignmentStatement", fn.Body.Statements[1])
	}
	if target, ok := assignment.Target.(*ast.Identifier); !ok || target.Value != "x" {
		t.Fatalf("assignment target = %#v, want identifier x", assignment.Target)
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
	if !ref.Slice {
		t.Fatalf("expected slice marker, got %+v", ref)
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

func TestParseExplicitImplExtension(t *testing.T) {
	input := `
impl extends Vehicle {
	fn Stop() void {
	}
}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	impl, ok := program.Statements[0].(*ast.ImplStatement)
	if !ok {
		t.Fatalf("statement is not ImplStatement. got=%T", program.Statements[0])
	}
	if !impl.Extends || impl.Target == nil || impl.Target.Name != "Vehicle" {
		t.Fatalf("wrong impl extension AST: %+v", impl)
	}
}

func TestParseImplAndInterfaceEvents(t *testing.T) {
	input := `
interface PressSource {
	event ButtonPressed[ButtonPressData]
}

type Button struct {
	ButtonPressed: Event[ButtonPressData, 8],
	buttonPressedStorage: EventStorage[ButtonPressData, 8],
}

impl Button {
	event Pressed using buttonPressedStorage
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	iface, ok := program.Statements[0].(*ast.InterfaceDeclaration)
	if !ok {
		t.Fatalf("statement 0 is not InterfaceDeclaration. got=%T", program.Statements[0])
	}
	if len(iface.Events) != 1 || iface.Events[0].Name.Value != "ButtonPressed" || iface.Events[0].Payload.Name != "ButtonPressData" {
		t.Fatalf("wrong interface event: %+v", iface.Events)
	}

	typeDecl := program.Statements[1].(*ast.TypeDeclStatement)
	eventField := typeDecl.StructType.Fields[0]
	if eventField.Type.Name != "Event" || len(eventField.Type.TypeArgs) != 1 || eventField.Type.TypeArgs[0].Name != "ButtonPressData" || !eventField.Type.EventCapacitySet || eventField.Type.EventCapacity != 8 {
		t.Fatalf("wrong event field type: %+v", eventField.Type)
	}

	implStmt := program.Statements[2].(*ast.ImplStatement)
	event, ok := implStmt.Members[0].(*ast.EventDeclaration)
	if !ok {
		t.Fatalf("impl member is not EventDeclaration. got=%T", implStmt.Members[0])
	}
	if event.Name.Value != "Pressed" || event.Storage.Value != "buttonPressedStorage" {
		t.Fatalf("wrong impl event declaration: %+v", event)
	}
}

func TestParseInterfaceDeclarationAndImplements(t *testing.T) {
	input := `
interface Vehicle {
	fn Start() void
	fn Stop() void

	property IsRunning: bool {
		get
	}
}

type Car struct implements Vehicle {
	running: bool,
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	iface, ok := program.Statements[0].(*ast.InterfaceDeclaration)
	if !ok {
		t.Fatalf("statement 0 is not InterfaceDeclaration. got=%T", program.Statements[0])
	}
	if iface.Name.Value != "Vehicle" || len(iface.Methods) != 2 || len(iface.Properties) != 1 {
		t.Fatalf("wrong interface declaration: %+v", iface)
	}
	if len(iface.Methods[0].Parameters) != 0 {
		t.Fatalf("expected no explicit self parameter: %+v", iface.Methods[0].Parameters)
	}

	typeDecl, ok := program.Statements[1].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement 1 is not TypeDeclStatement. got=%T", program.Statements[1])
	}
	if len(typeDecl.Implements) != 1 || typeDecl.Implements[0].Name != "Vehicle" {
		t.Fatalf("wrong implements list: %+v", typeDecl.Implements)
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

fn VoidOk() Result[void, IOError] {
	return Ok()
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

	voidOk := program.Statements[1].(*ast.FunctionDeclaration)
	voidOkReturn := voidOk.Body.Statements[0].(*ast.ReturnStatement)
	okExpr, ok := voidOkReturn.Value.(*ast.OkExpression)
	if !ok {
		t.Fatalf("VoidOk return is not OkExpression. got=%T", voidOkReturn.Value)
	}
	if okExpr.Value != nil {
		t.Fatalf("VoidOk Ok() should not have value. got=%T", okExpr.Value)
	}

	bar := program.Statements[2].(*ast.FunctionDeclaration)
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

func TestParseTryCoversFollowingArithmeticExpression(t *testing.T) {
	p := New(lexer.New(`fn Add(left: int, right: int) Result[int, ArithmeticError] {
	let value := try left + right
	return Ok(value)
}`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	letStmt := program.Statements[0].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.LetStatement)
	tryExpr, ok := letStmt.Value.(*ast.TryExpression)
	if !ok {
		t.Fatalf("initializer is %T, want TryExpression", letStmt.Value)
	}
	if infix, ok := tryExpr.Expression.(*ast.InfixExpression); !ok || infix.Operator != "+" {
		t.Fatalf("try operand is %#v, want complete addition", tryExpr.Expression)
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

func TestParseTryExpressionHandlersWithExplicitMatchWrapper(t *testing.T) {
	input := `
fn UseResult() Result[int, IOError] {
	let value := try Calculate() {
		match {
			Err(IOError.InvalidValue) => 0
			Err(error) => return Err(error)
		}
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

func TestParseTryExpressionHandlerAllowsTrailingComment(t *testing.T) {
	input := `
fn UseResult() Result[int, IOError] {
	let value := try Calculate() {
		match {
			Err(IOError.InvalidValue) => 0
			Err(error) => return Err(error) // propagate
		}
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
	program := p.ParseProgram()
	checkParserErrors(t, p)

	impl, ok := program.Statements[0].(*ast.ImplStatement)
	if !ok {
		t.Fatalf("statement 0 is not ImplStatement. got=%T", program.Statements[0])
	}
	if len(impl.Members) != 1 {
		t.Fatalf("wrong impl member count. got=%d want=1", len(impl.Members))
	}
	invalid, ok := impl.Members[0].(*ast.InvalidMember)
	if !ok {
		t.Fatalf("impl member is not InvalidMember. got=%T", impl.Members[0])
	}
	if invalid.Message != "variable declarations are not allowed inside impl" {
		t.Fatalf("wrong invalid message. got=%q", invalid.Message)
	}
	if invalid.Recovery == nil || invalid.Recovery.DiagnosticID != diagnostics.ParserInvalidBlockMember {
		t.Fatalf("missing invalid-member recovery: %#v", invalid.Recovery)
	}
}

func TestParseImplReservesFreeOperation(t *testing.T) {
	input := `
impl File {
	free {
		Close()
	}

	fn Done() void {
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	impl, ok := program.Statements[0].(*ast.ImplStatement)
	if !ok {
		t.Fatalf("statement 0 is not ImplStatement. got=%T", program.Statements[0])
	}
	if len(impl.Members) != 2 {
		t.Fatalf("wrong impl member count. got=%d want=2", len(impl.Members))
	}
	invalid, ok := impl.Members[0].(*ast.InvalidMember)
	if !ok {
		t.Fatalf("impl member 0 is not InvalidMember. got=%T", impl.Members[0])
	}
	if invalid.Message != "free operations are reserved for destruction but are not implemented yet" {
		t.Fatalf("wrong invalid message. got=%q", invalid.Message)
	}
	if _, ok := impl.Members[1].(*ast.FunctionDeclaration); !ok {
		t.Fatalf("impl member 1 should recover to function declaration. got=%T", impl.Members[1])
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

func TestParseUnitsInvalidFixtureRecovery(t *testing.T) {
	input, err := os.ReadFile("../../testdata/units_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	l := lexer.New(string(input))
	p := New(l)
	p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatal("expected units_invalid.sec to produce parser errors")
	}
	if p.Errors()[0] != `unit InvalidCategory category must be physical, currency, or other, got "experimental" at 271:30` {
		t.Fatalf("wrong first parser error. got=%q errors=%v", p.Errors()[0], p.Errors())
	}
}

func TestParsePropertiesInvalidFixtureRecovery(t *testing.T) {
	input, err := os.ReadFile("../../testdata/properties_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	l := lexer.New(string(input))
	p := New(l)
	p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatal("expected properties_invalid.sec to produce parser errors")
	}
	for _, err := range p.Errors() {
		if strings.Contains(err, `unexpected token "}"`) {
			t.Fatalf("unexpected cascading parser error: %q errors=%v", err, p.Errors())
		}
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

func TestParseExplicitTypeDefault(t *testing.T) {
	p := New(lexer.New(`type Port int range 1..65535 default 8080`))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	declaration, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok || declaration.Default == nil || declaration.Default.String() != "8080" {
		t.Fatalf("explicit default was not retained: %#v", program.Statements[0])
	}
}

func TestParseUnaryPlusAndCanonicalExpressionStarts(t *testing.T) {
	input := `module main

type Port int range +1..65535 default +8080

fn Value() int {
    return +1
}
`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	decl := program.Statements[1].(*ast.TypeDeclStatement)
	assertPrefixExpression(t, decl.Default, "+", "8080")
	fn := program.Statements[2].(*ast.FunctionDeclaration)
	ret := fn.Body.Statements[0].(*ast.ReturnStatement)
	assertPrefixExpression(t, ret.Value, "+", "1")

	for _, tokenType := range []lexer.TokenType{
		lexer.IDENT, lexer.SELF, lexer.INT, lexer.FLOAT, lexer.STRING, lexer.CHAR,
		lexer.INTERPSTRING, lexer.TRUE, lexer.FALSE, lexer.PLUS, lexer.MINUS,
		lexer.NOT, lexer.BIT_NOT, lexer.TRY, lexer.SPAWN, lexer.AWAIT, lexer.MATCH,
		lexer.FN, lexer.CAPTURE, lexer.AT, lexer.LBRACKET, lexer.REF, lexer.LPAREN,
	} {
		if !p.isExpressionStart(tokenType) {
			t.Errorf("prefix token %q is missing from expression-start classification", tokenType)
		}
	}
}

func TestParseTryThenMultilineStructLiteralWithoutCommas(t *testing.T) {
	input := `
fn NewWithFile(input: string, file: string) Result[Lexer, AllocationError] {
    let runes := try input.ToRuneArray()

    return Ok(Lexer {
        input: runes
        file: file
        pos: 0
        line: 1
        column: 1
    })
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
	returnStmt := fn.Body.Statements[1].(*ast.ReturnStatement)
	okExpr := returnStmt.Value.(*ast.OkExpression)
	literal, ok := okExpr.Value.(*ast.StructLiteral)
	if !ok {
		t.Fatalf("Ok value is not StructLiteral. got=%T", okExpr.Value)
	}
	if len(literal.Fields) != 5 {
		t.Fatalf("struct literal field count = %d, want 5", len(literal.Fields))
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

func TestParseSwitchStatement(t *testing.T) {
	input := `
fn Test(value: int) void {
	switch value {
	case < 0:
		return
	case 0, 1, 2..<10:
		fallthrough
	default:
		return
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	switchStmt, ok := fn.Body.Statements[0].(*ast.SwitchStatement)
	if !ok {
		t.Fatalf("statement is not SwitchStatement. got=%T", fn.Body.Statements[0])
	}
	if switchStmt.Subject == nil {
		t.Fatal("expected switch subject")
	}
	if len(switchStmt.Cases) != 2 {
		t.Fatalf("wrong case count. got=%d want=2", len(switchStmt.Cases))
	}
	if len(switchStmt.Cases[0].Items) != 1 {
		t.Fatalf("wrong first case item count. got=%d want=1", len(switchStmt.Cases[0].Items))
	}
	if _, ok := switchStmt.Cases[0].Items[0].(*ast.SwitchRelationalCase); !ok {
		t.Fatalf("first case item is not SwitchRelationalCase. got=%T", switchStmt.Cases[0].Items[0])
	}
	if len(switchStmt.Cases[1].Items) != 3 {
		t.Fatalf("wrong second case item count. got=%d want=3", len(switchStmt.Cases[1].Items))
	}
	if _, ok := switchStmt.Cases[1].Items[2].(*ast.SwitchRangeCase); !ok {
		t.Fatalf("third second-case item is not SwitchRangeCase. got=%T", switchStmt.Cases[1].Items[2])
	}
	if switchStmt.Default == nil {
		t.Fatal("expected default clause")
	}
}

func TestSwitchCaseLogicalOrSuggestsComma(t *testing.T) {
	input := `
fn Test(value: int) void {
	switch value {
	case 1 || 2:
		return
	default:
		return
	}
}
`

	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()

	expected := "use ',' between switch case values; '||' creates a boolean expression at 4:9"
	if len(p.Errors()) != 1 || p.Errors()[0] != expected {
		t.Fatalf("parser errors = %#v, want %#v", p.Errors(), []string{expected})
	}
}

func TestParseSubjectlessSwitchStatement(t *testing.T) {
	input := `
fn Test(value: int) void {
	switch {
	case value < 0:
		return
	default:
		return
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	switchStmt, ok := fn.Body.Statements[0].(*ast.SwitchStatement)
	if !ok {
		t.Fatalf("statement is not SwitchStatement. got=%T", fn.Body.Statements[0])
	}
	if switchStmt.Subject != nil {
		t.Fatalf("expected subjectless switch, got=%T", switchStmt.Subject)
	}
	if len(switchStmt.Cases) != 1 || switchStmt.Default == nil {
		t.Fatalf("wrong switch clauses. cases=%d default=%v", len(switchStmt.Cases), switchStmt.Default != nil)
	}
}

func TestParseSelectStatement(t *testing.T) {
	input := `
fn Test(rx: Receiver[int], tx: Sender[int], task: Task[int]) void {
	select {
		value := rx.Receive() => {
			discard value
		}
		tx.Send(1) => {
		}
		result := await task => {
			discard result
		}
		after 10 => {
		}
		default => {
		}
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	selectStmt, ok := fn.Body.Statements[0].(*ast.SelectStatement)
	if !ok {
		t.Fatalf("statement is not SelectStatement. got=%T", fn.Body.Statements[0])
	}
	if len(selectStmt.Branches) != 5 {
		t.Fatalf("wrong select branch count. got=%d want=5", len(selectStmt.Branches))
	}
	if selectStmt.Branches[0].Kind != ast.SelectOperationBranch || selectStmt.Branches[0].Binding == nil || selectStmt.Branches[0].Binding.Value != "value" {
		t.Fatalf("wrong receive branch: %+v", selectStmt.Branches[0])
	}
	if selectStmt.Branches[1].Kind != ast.SelectOperationBranch || selectStmt.Branches[1].Binding != nil {
		t.Fatalf("wrong send branch: %+v", selectStmt.Branches[1])
	}
	if selectStmt.Branches[2].Kind != ast.SelectOperationBranch || selectStmt.Branches[2].Binding == nil || selectStmt.Branches[2].Binding.Value != "result" {
		t.Fatalf("wrong await branch: %+v", selectStmt.Branches[2])
	}
	if selectStmt.Branches[3].Kind != ast.SelectTimeoutBranch {
		t.Fatalf("wrong timeout branch: %+v", selectStmt.Branches[3])
	}
	if selectStmt.Branches[4].Kind != ast.SelectDefaultBranch {
		t.Fatalf("wrong default branch: %+v", selectStmt.Branches[4])
	}
}

func TestParseUnsafeAsmStatement(t *testing.T) {
	input := `
fn Test() void {
	unsafe {
		asm "nop"
		asm("ret")
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	unsafeStmt, ok := fn.Body.Statements[0].(*ast.UnsafeStatement)
	if !ok {
		t.Fatalf("statement is not UnsafeStatement. got=%T", fn.Body.Statements[0])
	}
	if unsafeStmt.Body == nil || len(unsafeStmt.Body.Statements) != 2 {
		t.Fatalf("wrong unsafe body. got=%#v", unsafeStmt.Body)
	}
	first, ok := unsafeStmt.Body.Statements[0].(*ast.AsmStatement)
	if !ok {
		t.Fatalf("first unsafe body statement is not AsmStatement. got=%T", unsafeStmt.Body.Statements[0])
	}
	if first.Template == nil || first.Template.Value != "nop" {
		t.Fatalf("wrong first asm template. got=%#v", first.Template)
	}
	second, ok := unsafeStmt.Body.Statements[1].(*ast.AsmStatement)
	if !ok {
		t.Fatalf("second unsafe body statement is not AsmStatement. got=%T", unsafeStmt.Body.Statements[1])
	}
	if second.Template == nil || second.Template.Value != "ret" {
		t.Fatalf("wrong second asm template. got=%#v", second.Template)
	}
}

func TestParseInlineAsmBlock(t *testing.T) {
	input := `
fn _sysWrite(fd: int64, ref ptr: byte, len: int64) int64 {
	unsafe {
		asm {
			"syscall"
			inputs: rax(1), rdi(fd), rsi(ptr), rdx(len)
			outputs: rax
		}
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if !fn.Parameters[1].Ref {
		t.Fatal("ptr parameter should be ref")
	}
	if fn.Parameters[1].Name.Value != "ptr" || fn.Parameters[1].Type.Name != "byte" {
		t.Fatalf("wrong ref parameter. got=%s: %s", fn.Parameters[1].Name.Value, fn.Parameters[1].Type.Name)
	}
	unsafeStmt := fn.Body.Statements[0].(*ast.UnsafeStatement)
	asmStmt := unsafeStmt.Body.Statements[0].(*ast.AsmStatement)
	if asmStmt.Block == nil {
		t.Fatal("expected asm block")
	}
	if asmStmt.Block.Template == nil || asmStmt.Block.Template.Value != "syscall" {
		t.Fatalf("wrong asm template. got=%#v", asmStmt.Block.Template)
	}
	if len(asmStmt.Block.Inputs) != 4 {
		t.Fatalf("wrong input count. got=%d want=4", len(asmStmt.Block.Inputs))
	}
	if len(asmStmt.Block.Outputs) != 1 || asmStmt.Block.Outputs[0].Register != "rax" {
		t.Fatalf("wrong outputs. got=%#v", asmStmt.Block.Outputs)
	}
}

func TestParseUnsafeFunctionWithAsmNamedOutputAndClobbers(t *testing.T) {
	input := `
unsafe fn _rawSyscall3(number: uint, arg1: uint, arg2: uint, arg3: uint) int {
	asm {
		"syscall"

		inputs:
			rax(number)
			rdi(arg1)
			rsi(arg2)
			rdx(arg3)

		outputs:
			rax(result)

		clobbers:
			rcx
			r11
			memory
	}

	return result
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if !fn.Unsafe {
		t.Fatal("function should be unsafe")
	}
	asmStmt := fn.Body.Statements[0].(*ast.AsmStatement)
	if len(asmStmt.Block.Inputs) != 4 {
		t.Fatalf("wrong input count. got=%d want=4", len(asmStmt.Block.Inputs))
	}
	if len(asmStmt.Block.Outputs) != 1 || asmStmt.Block.Outputs[0].Register != "rax" || asmStmt.Block.Outputs[0].Name != "result" {
		t.Fatalf("wrong outputs. got=%#v", asmStmt.Block.Outputs)
	}
	if len(asmStmt.Block.Clobbers) != 3 || asmStmt.Block.Clobbers[2] != "memory" {
		t.Fatalf("wrong clobbers. got=%#v", asmStmt.Block.Clobbers)
	}
}

func TestParseExternFunctionDeclaration(t *testing.T) {
	input := `
extern "C" fn write(fd: int32, buffer: RawPtr[byte], length: uint) int64

fn Use(ref mut value: int) void {
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("wrong statement count. got=%d want=2", len(program.Statements))
	}

	externFn := program.Statements[0].(*ast.FunctionDeclaration)
	if !externFn.Extern || externFn.ABI != "C" {
		t.Fatalf("wrong extern function metadata: extern=%t abi=%q", externFn.Extern, externFn.ABI)
	}
	if externFn.Body != nil {
		t.Fatal("extern function should not have body")
	}
	if externFn.Parameters[1].Type.Name != "RawPtr" || len(externFn.Parameters[1].Type.TypeArgs) != 1 {
		t.Fatalf("wrong RawPtr parameter type: %+v", externFn.Parameters[1].Type)
	}

	useFn := program.Statements[1].(*ast.FunctionDeclaration)
	if !useFn.Parameters[0].Ref || !useFn.Parameters[0].MutableRef {
		t.Fatalf("expected ref mut parameter: %+v", useFn.Parameters[0])
	}
}

func TestParseExternLinkName(t *testing.T) {
	input := `
@link_name("c-write")
extern "C" fn write(fd: int32) int32
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
	}
	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if !fn.Extern || fn.ABI != "C" || fn.LinkName != "c-write" {
		t.Fatalf("wrong linked extern metadata: %+v", fn)
	}
}

func TestParseMutableSelfElementReferenceArgument(t *testing.T) {
	input := `
module fmt

type Buffer struct {
    data: byte[4],
}

fn Write(ref value: byte) void {
}

impl Buffer {
    fn Flush() void {
        Write(ref mut self.data[0])
    }
}
`

	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()
	checkParserErrors(t, p)
}

func TestParseUnsafeExternSystemFunctionDeclaration(t *testing.T) {
	input := `
unsafe extern "system" fn _rawSyscall(number: int64) int {
	return 0
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
	}

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if !fn.Unsafe || !fn.Extern || fn.ABI != "system" {
		t.Fatalf("wrong unsafe extern metadata: unsafe=%t extern=%t abi=%q", fn.Unsafe, fn.Extern, fn.ABI)
	}
	if fn.Body == nil || len(fn.Body.Statements) != 1 {
		t.Fatalf("extern system function should have body. got=%+v", fn.Body)
	}
}

func TestParseForRangeAndInfiniteLoops(t *testing.T) {
	input := `
fn Test() void {
	for i in 0..<10 {
		continue
	}

	for {
		break
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	rangeFor, ok := fn.Body.Statements[0].(*ast.ForStatement)
	if !ok {
		t.Fatalf("first statement is not ForStatement. got=%T", fn.Body.Statements[0])
	}
	if len(rangeFor.Bindings) != 1 || rangeFor.Bindings[0].Name != "i" {
		t.Fatalf("wrong for bindings. got=%#v", rangeFor.Bindings)
	}
	if _, ok := rangeFor.Iterable.(*ast.RangeExpression); !ok {
		t.Fatalf("iterable is not RangeExpression. got=%T", rangeFor.Iterable)
	}
	if _, ok := rangeFor.Body.Statements[0].(*ast.ContinueStatement); !ok {
		t.Fatalf("range body statement is not ContinueStatement. got=%T", rangeFor.Body.Statements[0])
	}

	infiniteFor, ok := fn.Body.Statements[1].(*ast.ForStatement)
	if !ok {
		t.Fatalf("second statement is not ForStatement. got=%T", fn.Body.Statements[1])
	}
	if len(infiniteFor.Bindings) != 0 || infiniteFor.Iterable != nil {
		t.Fatalf("infinite for should not have bindings or iterable. got=%#v %T", infiniteFor.Bindings, infiniteFor.Iterable)
	}
	if _, ok := infiniteFor.Body.Statements[0].(*ast.BreakStatement); !ok {
		t.Fatalf("infinite body statement is not BreakStatement. got=%T", infiniteFor.Body.Statements[0])
	}
}

func TestParseWhileStatement(t *testing.T) {
	input := `
fn Test(running: bool) void {
	while running {
		continue
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	whileStmt, ok := fn.Body.Statements[0].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("statement is not WhileStatement. got=%T", fn.Body.Statements[0])
	}
	if _, ok := whileStmt.Condition.(*ast.Identifier); !ok {
		t.Fatalf("condition is not Identifier. got=%T", whileStmt.Condition)
	}
	if _, ok := whileStmt.Body.Statements[0].(*ast.ContinueStatement); !ok {
		t.Fatalf("body statement is not ContinueStatement. got=%T", whileStmt.Body.Statements[0])
	}
}

func TestParseWhileRequiresCondition(t *testing.T) {
	input := `
fn Test() void {
	while {
	}
	return
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d errors=%v", len(p.Errors()), p.Errors())
	}
	expected := "while statement missing condition at 3:8"
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("parser should recover after invalid while. got=%d statements", len(fn.Body.Statements))
	}
}

func TestParseWhileAssignmentConditionErrorsAndRecovers(t *testing.T) {
	input := `
fn Test() void {
	let mut running: bool := false

	while running = true {
		break
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	expectedError := "assignment in while condition at 5:16"
	if len(p.Errors()) != 1 || p.Errors()[0] != expectedError {
		t.Fatalf("wrong parser errors. got=%v want=%q", p.Errors(), expectedError)
	}
	if len(p.Diagnostics()) != 1 || p.Diagnostics()[0].ID != diagnostics.ParserInvalidAssignmentExpr {
		t.Fatalf("wrong structured parser diagnostics: %+v", p.Diagnostics())
	}
	if len(p.Warnings()) != 0 {
		t.Fatalf("assignment condition must not remain a warning: %v", p.Warnings())
	}

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	whileStmt, ok := fn.Body.Statements[1].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("statement is not WhileStatement. got=%T", fn.Body.Statements[1])
	}
	ident, ok := whileStmt.Condition.(*ast.Identifier)
	if !ok || ident.Value != "running" {
		t.Fatalf("wrong condition. got=%T %v", whileStmt.Condition, whileStmt.Condition)
	}
	if _, ok := whileStmt.Body.Statements[0].(*ast.BreakStatement); !ok {
		t.Fatalf("body statement is not BreakStatement. got=%T", whileStmt.Body.Statements[0])
	}
}

func TestRecoveryPreservesNewlyCentralizedStatementStarts(t *testing.T) {
	input := `
? damaged
unit Metre decimal physical

fn Test() void {
	? damaged
	discard Work()
}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) != 2 {
		t.Fatalf("parser errors = %v, want two primary errors", p.Errors())
	}
	if len(program.Statements) != 3 {
		t.Fatalf("top-level statement count = %d, want 3", len(program.Statements))
	}
	if _, ok := program.Statements[0].(*ast.InvalidStatement); !ok {
		t.Fatalf("first recovered statement = %T, want InvalidStatement", program.Statements[0])
	}
	if _, ok := program.Statements[1].(*ast.UnitDeclStatement); !ok {
		t.Fatalf("first preserved valid statement = %T, want UnitDeclStatement", program.Statements[1])
	}
	fn := program.Statements[2].(*ast.FunctionDeclaration)
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("function statement count = %d, want 2", len(fn.Body.Statements))
	}
	if _, ok := fn.Body.Statements[0].(*ast.InvalidStatement); !ok {
		t.Fatalf("recovered body statement = %T, want InvalidStatement", fn.Body.Statements[0])
	}
	if _, ok := fn.Body.Statements[1].(*ast.DiscardStatement); !ok {
		t.Fatalf("preserved body statement = %T, want DiscardStatement", fn.Body.Statements[1])
	}
}

func TestFocusedRecoveryDiagnosticMetadata(t *testing.T) {
	tests := []struct {
		input      string
		id         string
		expected   lexer.TokenType
		unexpected lexer.TokenType
	}{
		{input: "? damaged", id: diagnostics.ParserUnexpectedToken, unexpected: lexer.QUESTION},
		{input: "else {}", id: diagnostics.ParserMisplacedKeyword, unexpected: lexer.ELSE},
		{input: "type Broken struct (", id: diagnostics.ParserMissingToken, expected: lexer.LBRACE, unexpected: lexer.LPAREN},
	}

	for _, test := range tests {
		p := New(lexer.New(test.input))
		p.ParseProgram()
		if len(p.Diagnostics()) == 0 {
			t.Fatalf("%q produced no structured diagnostic", test.input)
		}
		got := p.Diagnostics()[0]
		if got.ID != test.id {
			t.Fatalf("%q diagnostic ID = %s, want %s", test.input, got.ID, test.id)
		}
		if got.Unexpected == nil || got.Unexpected.Type != test.unexpected {
			t.Fatalf("%q unexpected token = %+v, want %s", test.input, got.Unexpected, test.unexpected)
		}
		if test.expected != "" && (len(got.Expected) != 1 || got.Expected[0] != test.expected) {
			t.Fatalf("%q expected tokens = %v, want %s", test.input, got.Expected, test.expected)
		}
	}
}

func TestParseResultRetainsRecoveryEventsAndInvalidNodes(t *testing.T) {
	input := `
? damaged
fn Valid() void {
	? damaged
	return
}
`

	result := New(lexer.New(input)).Parse()
	if !result.HasErrors || result.Fatal {
		t.Fatalf("wrong result flags: hasErrors=%t fatal=%t", result.HasErrors, result.Fatal)
	}
	if len(result.Diagnostics) != 2 || len(result.Recovery) != 2 {
		t.Fatalf("wrong recovery result: diagnostics=%d recovery=%d", len(result.Diagnostics), len(result.Recovery))
	}
	if result.Diagnostics[0].Context != RecoveryContextTopLevel || result.Diagnostics[1].Context != RecoveryContextBlock {
		t.Fatalf("wrong recovery contexts: %+v", result.Diagnostics)
	}
	if result.Diagnostics[0].Episode == 0 || result.Diagnostics[1].Episode == 0 || result.Diagnostics[0].Episode == result.Diagnostics[1].Episode {
		t.Fatalf("diagnostics did not get distinct recovery episodes: %+v", result.Diagnostics)
	}
	for index := range result.Recovery {
		if result.Recovery[index].Episode != result.Diagnostics[index].Episode || result.Recovery[index].Context != result.Diagnostics[index].Context {
			t.Fatalf("recovery event %d is detached from its diagnostic: diagnostic=%+v recovery=%+v", index, result.Diagnostics[index], result.Recovery[index])
		}
	}
	if len(result.Program.Statements) != 2 {
		t.Fatalf("statement count=%d, want 2", len(result.Program.Statements))
	}
	invalid, ok := result.Program.Statements[0].(*ast.InvalidStatement)
	if !ok || invalid.Recovery == nil || invalid.Recovery.Skipped == 0 {
		t.Fatalf("missing top-level invalid recovery node: %#v", result.Program.Statements[0])
	}
	fn := result.Program.Statements[1].(*ast.FunctionDeclaration)
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("function statement count=%d, want 2", len(fn.Body.Statements))
	}
	if _, ok := fn.Body.Statements[0].(*ast.InvalidStatement); !ok {
		t.Fatalf("body recovery node=%T, want InvalidStatement", fn.Body.Statements[0])
	}
}

func TestParseResultRecordsVirtualMissingToken(t *testing.T) {
	result := New(lexer.New(`type Broken struct (`)).Parse()
	if !result.HasErrors {
		t.Fatal("missing-token source should have parser errors")
	}
	found := false
	for _, event := range result.Recovery {
		if event.Kind == RecoveryInsertMissingToken && len(event.Expected) == 1 && event.Expected[0] == lexer.LBRACE {
			found = true
			if event.Confidence != RecoveryUnambiguous {
				t.Fatalf("missing-token confidence=%s", event.Confidence)
			}
			if event.Context != RecoveryContextTopLevel || event.Episode == 0 {
				t.Fatalf("missing-token recovery lacks context or episode: %+v", event)
			}
			matchedDiagnostic := false
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.ID == event.DiagnosticID && diagnostic.Context == event.Context && diagnostic.Episode == event.Episode {
					matchedDiagnostic = true
					break
				}
			}
			if !matchedDiagnostic {
				t.Fatalf("missing-token event is detached from diagnostics: %+v", event)
			}
		}
	}
	if !found {
		t.Fatalf("missing virtual-token recovery event: %+v", result.Recovery)
	}
}

func TestInvalidExpressionIsRetained(t *testing.T) {
	result := New(lexer.New(`fn Test() void { let value := ) }`)).Parse()
	if !result.HasErrors {
		t.Fatal("invalid expression should produce parser error")
	}
	fn := result.Program.Statements[0].(*ast.FunctionDeclaration)
	letStmt := fn.Body.Statements[0].(*ast.LetStatement)
	invalid, ok := letStmt.Value.(*ast.InvalidExpression)
	if !ok || invalid.Recovery == nil || invalid.Recovery.DiagnosticID != diagnostics.ParserInvalidExpression {
		t.Fatalf("invalid expression was not retained: %#v", letStmt.Value)
	}
}

func TestInvalidTypeReferenceIsRetained(t *testing.T) {
	result := New(lexer.New(`fn Test(value: ref) void {}`)).Parse()
	if !result.HasErrors {
		t.Fatal("invalid reference type should produce parser error")
	}
	fn := result.Program.Statements[0].(*ast.FunctionDeclaration)
	typ := fn.Parameters[0].Type
	if typ == nil || !typ.Invalid || typ.Recovery == nil || typ.Recovery.DiagnosticID != diagnostics.ParserInvalidTypeReference {
		t.Fatalf("invalid type reference was not retained: %#v", typ)
	}
}

func TestInvalidDeclarationIsRetained(t *testing.T) {
	result := New(lexer.New("fn Broken(\nfn Valid() void {}\n")).Parse()
	if !result.HasErrors {
		t.Fatal("invalid declaration should produce parser errors")
	}
	invalid, ok := result.Program.Statements[0].(*ast.InvalidDeclaration)
	if !ok || invalid.Recovery == nil || invalid.Recovery.DiagnosticID != diagnostics.ParserInvalidDeclaration {
		t.Fatalf("invalid declaration was not retained: %#v", result.Program.Statements[0])
	}
	if len(result.Program.Statements) != 2 {
		t.Fatalf("statement count=%d, want invalid declaration plus valid sibling", len(result.Program.Statements))
	}
	if _, ok := result.Program.Statements[1].(*ast.FunctionDeclaration); !ok {
		t.Fatalf("following declaration was not retained: %T", result.Program.Statements[1])
	}
}

func TestMatchRecoveryRetainsInvalidArmAndFollowingSibling(t *testing.T) {
	input := `fn Test(value: bool) int {
	return match value {
		true 1
		false => 2
	}
}`
	result := New(lexer.New(input)).Parse()
	if !result.HasErrors {
		t.Fatal("malformed match arm should produce parser errors")
	}
	fn := result.Program.Statements[0].(*ast.FunctionDeclaration)
	matchExpr := fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.MatchExpression)
	if len(matchExpr.Arms) != 2 {
		t.Fatalf("match arm count=%d, want invalid arm plus valid sibling", len(matchExpr.Arms))
	}
	if pattern, ok := matchExpr.Arms[0].Pattern.(*ast.InvalidPattern); !ok || !matchExpr.Arms[0].Invalid || pattern.Recovery == nil {
		t.Fatalf("first arm did not retain invalid pattern: %#v", matchExpr.Arms[0])
	}
	if pattern, ok := matchExpr.Arms[1].Pattern.(*ast.BooleanLiteral); !ok || pattern.Value {
		t.Fatalf("following match sibling was not retained: %#v", matchExpr.Arms[1].Pattern)
	}
}

func TestTryRecoveryRetainsInvalidHandlerAndFollowingSibling(t *testing.T) {
	input := `fn Test() Result[int, IOError] {
	let value := try Calculate() {
		Err(first) 0
		Err(second) => 1
	}
	return Ok(value)
}`
	result := New(lexer.New(input)).Parse()
	if !result.HasErrors {
		t.Fatal("malformed try handler should produce parser errors")
	}
	fn := result.Program.Statements[0].(*ast.FunctionDeclaration)
	tryExpr := fn.Body.Statements[0].(*ast.LetStatement).Value.(*ast.TryExpression)
	if len(tryExpr.Handlers) != 2 {
		t.Fatalf("try handler count=%d, want invalid handler plus valid sibling", len(tryExpr.Handlers))
	}
	if pattern, ok := tryExpr.Handlers[0].Pattern.(*ast.InvalidPattern); !ok || !tryExpr.Handlers[0].Invalid || pattern.Recovery == nil {
		t.Fatalf("first handler did not retain invalid pattern: %#v", tryExpr.Handlers[0])
	}
	if _, ok := tryExpr.Handlers[1].Pattern.(*ast.ErrExpression); !ok {
		t.Fatalf("following try sibling was not retained: %#v", tryExpr.Handlers[1].Pattern)
	}
}

func TestDiagnosticDeduplicationUsesIDRangeAndRecoveryContext(t *testing.T) {
	p := New(lexer.New("?"))
	primary := p.curToken
	p.addDiagnostic(diagnostics.ParserUnexpectedToken, primary, nil, &primary, "first")
	p.addDiagnostic(diagnostics.ParserUnexpectedToken, primary, nil, &primary, "duplicate text may differ")
	p.addDiagnostic(diagnostics.ParserSyntaxError, primary, nil, &primary, "secondary diagnostic for the same local cause")
	if len(p.Diagnostics()) != 1 || len(p.Errors()) != 1 {
		t.Fatalf("same diagnostic or same-cause secondary was not suppressed: diagnostics=%d errors=%d", len(p.Diagnostics()), len(p.Errors()))
	}

	p.endRecoveryEpisode()
	p.recoveryContext = RecoveryContextBlock
	p.addDiagnostic(diagnostics.ParserUnexpectedToken, primary, nil, &primary, "same range in a distinct context")
	if len(p.Diagnostics()) != 2 {
		t.Fatalf("distinct recovery context was incorrectly deduplicated: %+v", p.Diagnostics())
	}
	if p.Diagnostics()[0].Episode == p.Diagnostics()[1].Episode {
		t.Fatalf("stable boundary did not start a new episode: %+v", p.Diagnostics())
	}
}

func TestDiagnosticRollbackRestoresDeduplicationAndEpisodeState(t *testing.T) {
	p := New(lexer.New("?"))
	primary := p.curToken
	p.addDiagnostic(diagnostics.ParserUnexpectedToken, primary, nil, &primary, "speculative")
	p.rollbackErrors(0)
	p.addDiagnostic(diagnostics.ParserUnexpectedToken, primary, nil, &primary, "committed")

	diagnosticsAfterRollback := p.Diagnostics()
	if len(diagnosticsAfterRollback) != 1 || diagnosticsAfterRollback[0].Message != "committed" {
		t.Fatalf("rollback left stale diagnostic or dedup key: %+v", diagnosticsAfterRollback)
	}
	if diagnosticsAfterRollback[0].Episode != 1 {
		t.Fatalf("rollback did not restore deterministic episode numbering: %+v", diagnosticsAfterRollback[0])
	}
}

func TestDelimiterStackPreservesMismatchedOuterCloser(t *testing.T) {
	delimiters := newDelimiterStack(lexer.RPAREN)
	if !delimiters.consume(lexer.LBRACKET) || delimiters.depth() != 2 {
		t.Fatalf("nested delimiter was not tracked: %#v", delimiters)
	}
	if delimiters.canConsume(lexer.RPAREN) || delimiters.consume(lexer.RPAREN) {
		t.Fatal("mismatched closer should be left to the outer recovery context")
	}
	if delimiters.depth() != 2 {
		t.Fatalf("mismatched closer mutated delimiter stack: %#v", delimiters)
	}
	if !delimiters.consume(lexer.RBRACKET) || !delimiters.consume(lexer.RPAREN) || !delimiters.empty() {
		t.Fatalf("matching closers did not unwind stack: %#v", delimiters)
	}
}

func TestMalformedStructFieldRecoveryIgnoresNestedComma(t *testing.T) {
	p := New(lexer.New("bad(foo, bar), next"))
	primary := p.curToken
	p.addDiagnostic(diagnostics.ParserSyntaxError, primary, nil, nil, "malformed field")
	if !p.skipMalformedStructField() {
		t.Fatal("outer field comma was not found")
	}
	if p.curToken.Type != lexer.COMMA || p.peekToken.Lexeme != "next" {
		t.Fatalf("recovery synchronized at nested comma: current=%+v peek=%+v", p.curToken, p.peekToken)
	}
	events := p.RecoveryEvents()
	if len(events) != 1 || events[0].Kind != RecoverySkipTokens || events[0].Skipped < 6 || events[0].Episode == 0 {
		t.Fatalf("malformed field skip was not retained: %+v", events)
	}
}

func TestBalancedBlockRecoveryUsesSharedDelimiterStack(t *testing.T) {
	p := New(lexer.New("{ Call([1]) } fn"))
	primary := p.curToken
	p.addDiagnostic(diagnostics.ParserSyntaxError, primary, nil, nil, "skip block")
	event := p.skipCurrentBlock()
	if p.curToken.Type != lexer.RBRACE || p.peekToken.Type != lexer.FN {
		t.Fatalf("balanced block recovery stopped at nested delimiter: current=%+v peek=%+v", p.curToken, p.peekToken)
	}
	if event.Kind != RecoverySkipTokens || event.End.Type != lexer.RBRACE || event.Episode == 0 {
		t.Fatalf("balanced block recovery event=%+v", event)
	}
}

func TestParserDiagnosticLimitIsStable(t *testing.T) {
	input := strings.Repeat("else {}\n", maxParserDiagnostics+25)
	result := New(lexer.New(input)).Parse()
	if len(result.Diagnostics) != maxParserDiagnostics {
		t.Fatalf("diagnostic count=%d, want %d", len(result.Diagnostics), maxParserDiagnostics)
	}
	last := result.Diagnostics[len(result.Diagnostics)-1]
	if last.ID != diagnostics.ParserRecoveryLimit {
		t.Fatalf("last diagnostic ID=%s, want %s", last.ID, diagnostics.ParserRecoveryLimit)
	}
	count := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.ID == diagnostics.ParserRecoveryLimit {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("recovery-limit diagnostic count=%d, want 1", count)
	}
}

func TestParseMatchDiscardCatchAllPattern(t *testing.T) {
	input := `
fn Test(value: bool) int {
	let result := match value {
		_ => 1
	}
	return result
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	letStmt := fn.Body.Statements[0].(*ast.LetStatement)
	matchExpr, ok := letStmt.Value.(*ast.MatchExpression)
	if !ok {
		t.Fatalf("let value is not MatchExpression. got=%T", letStmt.Value)
	}
	pattern, ok := matchExpr.Arms[0].Pattern.(*ast.Identifier)
	if !ok || pattern.Value != "_" {
		t.Fatalf("wrong match pattern. got=%T %#v", matchExpr.Arms[0].Pattern, matchExpr.Arms[0].Pattern)
	}
}

func TestParseMatchWhereGuard(t *testing.T) {
	input := `
fn Test(value: bool) int {
	let result := match value {
		_ where value => 1
		_ => 0
	}
	return result
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	letStmt := fn.Body.Statements[0].(*ast.LetStatement)
	matchExpr := letStmt.Value.(*ast.MatchExpression)
	if matchExpr.Arms[0].Guard == nil {
		t.Fatal("expected first match arm to have guard")
	}
	guard, ok := matchExpr.Arms[0].Guard.(*ast.Identifier)
	if !ok || guard.Value != "value" {
		t.Fatalf("wrong guard. got=%T %#v", matchExpr.Arms[0].Guard, matchExpr.Arms[0].Guard)
	}
}

func TestParseReturnMatchExpression(t *testing.T) {
	input := `
fn Test(value: bool) int {
	return match value {
		_ => 1
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	ret := fn.Body.Statements[0].(*ast.ReturnStatement)
	if _, ok := ret.Value.(*ast.MatchExpression); !ok {
		t.Fatalf("return value is not MatchExpression. got=%T", ret.Value)
	}
}

func TestParseMatchDiscardPayloadPattern(t *testing.T) {
	input := `
fn Test(result: Result[int, IOError]) int {
	return match result {
		Ok(_) => 1
		Err(_) => 0
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	ret := fn.Body.Statements[0].(*ast.ReturnStatement)
	matchExpr := ret.Value.(*ast.MatchExpression)
	okPattern := matchExpr.Arms[0].Pattern.(*ast.OkExpression)
	okIdent := okPattern.Value.(*ast.Identifier)
	if okIdent.Value != "_" {
		t.Fatalf("wrong Ok discard pattern. got=%q", okIdent.Value)
	}
	errPattern := matchExpr.Arms[1].Pattern.(*ast.ErrExpression)
	errIdent := errPattern.Value.(*ast.Identifier)
	if errIdent.Value != "_" {
		t.Fatalf("wrong Err discard pattern. got=%q", errIdent.Value)
	}
}

func TestParseForRejectsConditionOnlySyntax(t *testing.T) {
	input := `
fn Test(running: bool) void {
	for running {
		break
	}

	return
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d errors=%v", len(p.Errors()), p.Errors())
	}
	expected := "condition-only for loops are not supported; use while at 3:6"
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("parser should recover after invalid for. got=%d statements", len(fn.Body.Statements))
	}
}

func TestParseForRejectsCStyleSyntax(t *testing.T) {
	input := `
fn Test() void {
	for i := 0; i < 10; i += 1 {
	}
}
`

	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()

	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d errors=%v", len(p.Errors()), p.Errors())
	}
	expected := "C-style for loops are not supported; use a range or while at 3:6"
	if p.Errors()[0] != expected {
		t.Fatalf("wrong parser error. got=%q want=%q", p.Errors()[0], expected)
	}
}

func TestParseForAllowsManyBindingsForSema(t *testing.T) {
	input := `
fn TooManyBindings(values: ref int[]) void {
	for a, b, c in values {
	}
}

fn Next() void {
	return
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("parser should continue after many-binding for. got=%d statements", len(program.Statements))
	}
	fn := program.Statements[0].(*ast.FunctionDeclaration)
	forStmt := fn.Body.Statements[0].(*ast.ForStatement)
	if len(forStmt.Bindings) != 3 {
		t.Fatalf("wrong binding count. got=%d want=3", len(forStmt.Bindings))
	}
}

func TestParseForRangeStep(t *testing.T) {
	input := `
fn Test() void {
	for i in 0..<10 step 2 {
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	forStmt := fn.Body.Statements[0].(*ast.ForStatement)
	if forStmt.Step == nil {
		t.Fatal("expected for step expression")
	}
	if forStmt.Step.String() != "2" {
		t.Fatalf("wrong step. got=%q want=2", forStmt.Step.String())
	}
}

func TestParseLambdaExpression(t *testing.T) {
	input := `
fn Test() int {
	let double := fn(value: int) int {
		return value * 2
	}

	return double(10)
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	letStmt := fn.Body.Statements[0].(*ast.LetStatement)
	lambda, ok := letStmt.Value.(*ast.LambdaExpression)
	if !ok {
		t.Fatalf("let value is not LambdaExpression. got=%T", letStmt.Value)
	}
	if len(lambda.Parameters) != 1 {
		t.Fatalf("wrong lambda parameter count. got=%d", len(lambda.Parameters))
	}
	if lambda.Parameters[0].Name.Value != "value" {
		t.Fatalf("wrong parameter name. got=%q", lambda.Parameters[0].Name.Value)
	}
	if lambda.ReturnType.Name != "int" {
		t.Fatalf("wrong return type. got=%q", lambda.ReturnType.Name)
	}
}

func TestParseDeferStatement(t *testing.T) {
	input := `
fn Test() void {
	defer {
		Cleanup()
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	deferStmt, ok := fn.Body.Statements[0].(*ast.DeferStatement)
	if !ok {
		t.Fatalf("statement is not DeferStatement. got=%T", fn.Body.Statements[0])
	}
	if deferStmt.Body == nil || len(deferStmt.Body.Statements) != 1 {
		t.Fatalf("wrong defer body. got=%#v", deferStmt.Body)
	}
}

func TestParseDeferReturnStatement(t *testing.T) {
	input := `
fn Test() void {
	defer return
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	deferStmt, ok := fn.Body.Statements[0].(*ast.DeferStatement)
	if !ok {
		t.Fatalf("statement is not DeferStatement. got=%T", fn.Body.Statements[0])
	}
	if deferStmt.Body == nil || len(deferStmt.Body.Statements) != 1 {
		t.Fatalf("wrong defer return body. got=%#v", deferStmt.Body)
	}
	if _, ok := deferStmt.Body.Statements[0].(*ast.ReturnStatement); !ok {
		t.Fatalf("defer body is not return. got=%T", deferStmt.Body.Statements[0])
	}
}

func TestParseDiscardStatement(t *testing.T) {
	input := `
fn Test(error: IOError) void {
	discard error
	discard Calculate()
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	stmt, ok := fn.Body.Statements[0].(*ast.DiscardStatement)
	if !ok {
		t.Fatalf("statement is not DiscardStatement. got=%T", fn.Body.Statements[0])
	}
	if stmt.Name == nil || stmt.Name.Value != "error" {
		t.Fatalf("wrong discard name. got=%#v", stmt.Name)
	}
	if stmt.Value == nil || stmt.Value.String() != "error" {
		t.Fatalf("wrong discard value. got=%#v", stmt.Value)
	}
	callStmt, ok := fn.Body.Statements[1].(*ast.DiscardStatement)
	if !ok {
		t.Fatalf("statement 1 is not DiscardStatement. got=%T", fn.Body.Statements[1])
	}
	if callStmt.Name != nil {
		t.Fatalf("discard call should not have Name. got=%#v", callStmt.Name)
	}
	if callStmt.Value == nil || callStmt.Value.String() != "Calculate()" {
		t.Fatalf("wrong discard call value. got=%#v", callStmt.Value)
	}
}

func TestParseCancelStatement(t *testing.T) {
	input := `
fn Test() void {
	cancel
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	if _, ok := fn.Body.Statements[0].(*ast.CancelStatement); !ok {
		t.Fatalf("statement is not CancelStatement. got=%T", fn.Body.Statements[0])
	}
}

func TestParseSpawnKindModifiers(t *testing.T) {
	input := `
fn Test() void {
	let taskHandle := spawn task Work()
	let threadHandle := spawn thread Work()
	let processHandle := spawn process Work()
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	tests := []struct {
		index int
		kind  string
	}{
		{0, "task"},
		{1, "thread"},
		{2, "process"},
	}
	for _, tt := range tests {
		letStmt := fn.Body.Statements[tt.index].(*ast.LetStatement)
		spawn, ok := letStmt.Value.(*ast.SpawnExpression)
		if !ok {
			t.Fatalf("statement %d value is not SpawnExpression. got=%T", tt.index, letStmt.Value)
		}
		if spawn.Kind != tt.kind {
			t.Fatalf("statement %d wrong spawn kind. got=%q want=%q", tt.index, spawn.Kind, tt.kind)
		}
		call, ok := spawn.Value.(*ast.CallExpression)
		if !ok {
			t.Fatalf("statement %d spawn value is not CallExpression. got=%T", tt.index, spawn.Value)
		}
		if got := call.Callee.String(); got != "Work" {
			t.Fatalf("statement %d wrong callee. got=%q", tt.index, got)
		}
	}
}

func TestParseDetachStatement(t *testing.T) {
	input := `
fn Test() void {
	detach worker
	detach calculation discard
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	first, ok := fn.Body.Statements[0].(*ast.DetachStatement)
	if !ok {
		t.Fatalf("first statement is not DetachStatement. got=%T", fn.Body.Statements[0])
	}
	if first.Value == nil || first.Value.String() != "worker" || first.DiscardResult {
		t.Fatalf("wrong first detach statement: %+v", first)
	}
	second, ok := fn.Body.Statements[1].(*ast.DetachStatement)
	if !ok {
		t.Fatalf("second statement is not DetachStatement. got=%T", fn.Body.Statements[1])
	}
	if second.Value == nil || second.Value.String() != "calculation" || !second.DiscardResult {
		t.Fatalf("wrong second detach statement: %+v", second)
	}
}

func TestParseDeferRequiresBlock(t *testing.T) {
	input := `
fn Test() void {
	defer Cleanup()
}
`

	l := lexer.New(input)
	p := New(l)
	_ = p.ParseProgram()

	expected := `defer requires a block at 3:8`
	if len(p.Errors()) != 1 || p.Errors()[0] != expected {
		t.Fatalf("wrong parser errors. got=%v want=%q", p.Errors(), expected)
	}
}

func TestParseFunctionTypeReference(t *testing.T) {
	input := `
fn Test() void {
	let positive: fn(int) bool := fn(value: int) bool {
		return value > 0
	}
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	letStmt := fn.Body.Statements[0].(*ast.LetStatement)
	if letStmt.Type.Name != "fn" {
		t.Fatalf("wrong type name. got=%q", letStmt.Type.Name)
	}
	if len(letStmt.Type.FunctionParameterTypes) != 1 {
		t.Fatalf("wrong function parameter type count. got=%d", len(letStmt.Type.FunctionParameterTypes))
	}
	if letStmt.Type.FunctionParameterTypes[0].Name != "int" {
		t.Fatalf("wrong function parameter type. got=%q", letStmt.Type.FunctionParameterTypes[0].Name)
	}
	if letStmt.Type.FunctionReturnType.Name != "bool" {
		t.Fatalf("wrong function return type. got=%q", letStmt.Type.FunctionReturnType.Name)
	}
}

func TestParseCollectionShapedTypeReferences(t *testing.T) {
	input := `
fn Test() void {
	let values: list[int, 8]
	let lookup: map[string, int, 16]
	let flags: set[string]
	let position: vector[float64, 3]
	let transform: matrix[float32, 4, 4]
	let image: tensor[float32, 3, 224, 224]
	let view: tensor_view[float32, 3]
	let shape: Shape[3]
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	tests := []struct {
		index      int
		name       string
		typeArgs   int
		constArgs  []string
		firstType  string
		secondType string
	}{
		{0, "list", 1, []string{"8"}, "int", ""},
		{1, "map", 2, []string{"16"}, "string", "int"},
		{2, "set", 1, nil, "string", ""},
		{3, "vector", 1, []string{"3"}, "float64", ""},
		{4, "matrix", 1, []string{"4", "4"}, "float32", ""},
		{5, "tensor", 1, []string{"3", "224", "224"}, "float32", ""},
		{6, "tensor_view", 1, []string{"3"}, "float32", ""},
		{7, "Shape", 0, []string{"3"}, "", ""},
	}

	for _, tt := range tests {
		letStmt := fn.Body.Statements[tt.index].(*ast.LetStatement)
		if letStmt.Type.Name != tt.name {
			t.Fatalf("statement %d wrong type name. got=%q want=%q", tt.index, letStmt.Type.Name, tt.name)
		}
		if len(letStmt.Type.TypeArgs) != tt.typeArgs {
			t.Fatalf("%s wrong type arg count. got=%d want=%d", tt.name, len(letStmt.Type.TypeArgs), tt.typeArgs)
		}
		if tt.firstType != "" && letStmt.Type.TypeArgs[0].Name != tt.firstType {
			t.Fatalf("%s wrong first type arg. got=%q want=%q", tt.name, letStmt.Type.TypeArgs[0].Name, tt.firstType)
		}
		if tt.secondType != "" && letStmt.Type.TypeArgs[1].Name != tt.secondType {
			t.Fatalf("%s wrong second type arg. got=%q want=%q", tt.name, letStmt.Type.TypeArgs[1].Name, tt.secondType)
		}
		if len(letStmt.Type.ConstArgs) != len(tt.constArgs) {
			t.Fatalf("%s wrong const arg count. got=%d want=%d", tt.name, len(letStmt.Type.ConstArgs), len(tt.constArgs))
		}
		for i, arg := range tt.constArgs {
			if letStmt.Type.ConstArgs[i].String() != arg {
				t.Fatalf("%s wrong const arg %d. got=%q want=%q", tt.name, i, letStmt.Type.ConstArgs[i].String(), arg)
			}
		}
	}
}

func TestParseUnitConversionExpression(t *testing.T) {
	input := `
fn Test(Amp: decimal<A>, Second: decimal<s>) decimal<C> {
	return decimal<C>(decimal(Amp) * decimal(Second))
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	returnStmt := fn.Body.Statements[0].(*ast.ReturnStatement)
	conversion, ok := returnStmt.Value.(*ast.ConversionExpression)
	if !ok {
		t.Fatalf("return value is not ConversionExpression. got=%T", returnStmt.Value)
	}
	if conversion.Type.Name != "decimal" || conversion.Type.Unit != "C" {
		t.Fatalf("wrong conversion target. got=%s<%s>", conversion.Type.Name, conversion.Type.Unit)
	}
	if _, ok := conversion.Value.(*ast.InfixExpression); !ok {
		t.Fatalf("conversion value is not InfixExpression. got=%T", conversion.Value)
	}
}

func TestUnitConversionLookaheadKeepsLessThanComparison(t *testing.T) {
	input := `
fn Less(left: int, right: int) bool {
	return left < right
}
`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	returnStmt := fn.Body.Statements[0].(*ast.ReturnStatement)
	infix, ok := returnStmt.Value.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("return value is not InfixExpression. got=%T", returnStmt.Value)
	}
	if infix.Operator != "<" {
		t.Fatalf("wrong operator. got=%q want=<", infix.Operator)
	}
}

func TestParserPreservesLexerDiagnosticIDs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		id    string
	}{
		{name: "invalid UTF-8", input: string([]byte{'m', 'o', 'd', 'u', 'l', 'e', ' ', 0xff}), id: diagnostics.LexerInvalidUTF8},
		{name: "unexpected BOM", input: "module main\n\uFEFFlet value := 1", id: diagnostics.LexerUnexpectedByteOrderMark},
		{name: "unsupported whitespace", input: "module\u00A0main", id: diagnostics.LexerUnsupportedWhitespace},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := New(lexer.NewWithFile(test.input, "invalid.sec")).Parse()
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.ID == test.id {
					return
				}
			}
			t.Fatalf("missing lexer diagnostic %s in %+v", test.id, result.Diagnostics)
		})
	}
}
