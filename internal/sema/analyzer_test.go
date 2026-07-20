package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func TestAnalyzeSimpleLetInitializers(t *testing.T) {
	input := `
module main

let a: int := -5
let b: uint := 5
let c: uint := -5
let d: bool := "test"
let e: uuid := 1
`

	errors := analyzeSource(t, input)

	expected := []string{
		"value -5 overflows uint at 6:16",
		"cannot initialize bool with string at 7:16",
		"unknown type uuid at 8:8",
	}

	assertSemaErrors(t, errors, expected)
}

func TestModuleDeclarationIsRequired(t *testing.T) {
	errors := analyzeSourceRaw(t, `
let a := 1
`)

	expected := []string{
		"missing module declaration",
	}

	assertSemaErrors(t, errors, expected)
}

func TestUnderscoreVisibilityAcrossModules(t *testing.T) {
	input := `
module x.y

type _SharedInt int
type __PrivateInt int

fn _shared() int {
	return 1
}

fn __private() int {
	return 2
}

module x.z

fn UseShared() int {
	let value: _SharedInt := 1
	return _shared() + value
}

fn UsePrivateFunction() int {
	return __private()
}

fn UsePrivateType() int {
	let value: __PrivateInt := 1
	return 0
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"function __private is not accessible from module x.z at 23:9",
		"type __PrivateInt is not accessible from module x.z at 27:13",
	}

	assertSemaErrors(t, errors, expected)
}

func TestDoubleUnderscoreIsVisibleInExactModule(t *testing.T) {
	input := `
module x.y

type __PrivateInt int

fn __private() int {
	return 2
}

fn UsePrivate() int {
	let value: __PrivateInt := 1
	return __private() + value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestExecutableCodeIsNotAllowedAtModuleScope(t *testing.T) {
	input := `
module main

let mut i := 0
i += 1
return i
`

	errors := analyzeSource(t, input)

	expected := []string{
		"assignment is not allowed at module scope at 5:1",
		"return is not allowed at module scope at 6:1",
	}

	assertSemaErrors(t, errors, expected)
}

func TestImmutableTypedLetWithoutInitializer(t *testing.T) {
	input := `
module main

fn Test() void {
	let a: int
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"immutable variable a requires initializer at 5:6",
	}
	assertSemaErrors(t, errors, expected)
}

func TestLetInitializerTypeMismatches(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "int from bool",
			input:    "let i: int := true",
			expected: "cannot initialize int with bool at 1:15",
		},
		{
			name:     "bool from int",
			input:    "let b: bool := 1",
			expected: "cannot initialize bool with int at 1:16",
		},
		{
			name:     "bool from string",
			input:    `let b: bool := "hello"`,
			expected: "cannot initialize bool with string at 1:16",
		},
		{
			name:     "string from int",
			input:    "let s: string := 42",
			expected: "cannot initialize string with int at 1:18",
		},
		{
			name:     "int from float",
			input:    "let i: int := 3.14",
			expected: "cannot initialize int with decimal at 1:15",
		},
		{
			name:     "float from bool",
			input:    "let f: float := true",
			expected: "cannot initialize float with bool at 1:17",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := analyzeSource(t, tt.input)
			assertSemaErrors(t, errors, []string{tt.expected})
		})
	}
}

func TestDecimalInitializersAndAssignments(t *testing.T) {
	input := `
let pi: decimal := 3.141592
let neg: decimal := -0.5
let mut p: decimal := 1
fn Test() void {
	p += .1
	p += 0.1
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDecimalDoesNotImplicitlyAcceptFloatVariables(t *testing.T) {
	input := `
let f: float64 := 3.14
let d: decimal := f
let mut p: decimal := 1
fn Test() void {
	p += f
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot initialize decimal with float64 at 3:19",
		"cannot add float64 to decimal at 6:7",
	}

	assertSemaErrors(t, errors, expected)
}

func TestDecimalLiteralInfersDecimalByDefault(t *testing.T) {
	input := `
let x := 3.14
let f: float64 := 3.14
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	x := analyzer.symbols["x"]
	if x.Type.Name != "decimal" {
		t.Fatalf("wrong inferred type. got=%q want=%q", x.Type.Name, "decimal")
	}

	f := analyzer.symbols["f"]
	if f.Type.Name != "float64" {
		t.Fatalf("wrong explicit type. got=%q want=%q", f.Type.Name, "float64")
	}
}

func TestNumericLiteralSuffixesAndBases(t *testing.T) {
	input := `
let i := 10i
let u := 10u
let f := 10f
let d := 10d
let hf := 1.5f
let hd := 1.5d
let b := 0b1000
let o := 0o10
let x := 0x8u
let h := 0x8d
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	expected := map[string]string{
		"i":  "int",
		"u":  "uint",
		"f":  "float",
		"d":  "decimal",
		"hf": "float",
		"hd": "decimal",
		"b":  "int",
		"o":  "int",
		"x":  "uint",
		"h":  "int",
	}
	for name, want := range expected {
		if got := analyzer.symbols[name].Type.Name; got != want {
			t.Fatalf("%s inferred as %q, want %q", name, got, want)
		}
	}
}

func TestDecimalLiteralValueUsesLexeme(t *testing.T) {
	tests := []struct {
		input string
		want  DecimalValue
	}{
		{input: "3.14", want: DecimalValue{Int64: 314, Scale: 2}},
		{input: ".1", want: DecimalValue{Int64: 1, Scale: 1}},
		{input: "-0.5", want: DecimalValue{Int64: -5, Scale: 1}},
		{input: "100", want: DecimalValue{Int64: 100, Scale: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			expr := parseExpressionSource(t, tt.input)
			got, ok := decimalLiteralValue(expr)
			if !ok {
				t.Fatalf("expected decimal literal value for %q", tt.input)
			}
			if got != tt.want {
				t.Fatalf("wrong decimal value. got=%+v want=%+v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeAssignmentsAndRedeclarations(t *testing.T) {
	input := `
module main

let mut a: uint := 5
let u: uint := 6
fn Test() void {
	a = u - 6
	let u: uint := 7
	u = 1
	missing = 1
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"variable \"u\" already declared at 8:6, previous declaration at 5:5",
		"cannot assign to immutable variable u at 9:2",
		"undefined variable missing at 10:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestAnalyzeLetGroups(t *testing.T) {
	input := `
int mut: a, b, c
float: f := 5.4, pi := 3.14
let x := 9, s := "hello", ok := true
let mut ma := 9, ms := "hello", mok := false
fn Test() void {
	a = x
	ma = 10
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	for _, name := range []string{"a", "b", "c", "ma", "ms", "mok"} {
		if !analyzer.symbols[name].Mutable {
			t.Fatalf("%s should be mutable", name)
		}
	}

	for _, name := range []string{"f", "pi", "x", "s", "ok"} {
		if analyzer.symbols[name].Mutable {
			t.Fatalf("%s should be immutable", name)
		}
	}

	if analyzer.symbols["f"].Type.Name != "float" {
		t.Fatalf("wrong f type. got=%q want=float", analyzer.symbols["f"].Type.Name)
	}
	if analyzer.symbols["pi"].Type.Name != "float" {
		t.Fatalf("wrong pi type. got=%q want=float", analyzer.symbols["pi"].Type.Name)
	}
}

func TestNamedIntegerRangeChecksConstantExpressions(t *testing.T) {
	input := `
type Percent int range 0..100

let mut p: Percent := 50
fn Test() void {
	try p = 100
	try p = 101
	try p = 10 * 10
	try p = 50 + 51
	try p = 50
	try p += 20
	try p += 60
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"value 101 violates range contract Percent 0..100 at 7:10",
		"value 101 violates range contract Percent 0..100 at 9:13",
		"value 130 violates range contract Percent 0..100 at 12:11",
	}

	assertSemaErrors(t, errors, expected)
}

func TestContractedVariableAssignmentRequiresTry(t *testing.T) {
	input := `
type Percent int range 0..100

let mut p: Percent := 50
fn Test() void {
	p = 60
	p += 1
	try p = 70
	try p += 1
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"assigning variable p requires try because Percent has contracts at 6:2",
		"assigning variable p requires try because Percent has contracts at 7:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestNamedTypeRegistryRangeContractAndInitialization(t *testing.T) {
	input := `
type Percent int range 0..100

let p1: Percent := 90
let p2: Percent := 101
let x := 90
let p3: Percent := x
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)

	expected := []string{
		"value 101 violates range contract Percent 0..100 at 5:20",
		"cannot initialize Percent with int at 7:20",
	}

	assertSemaErrors(t, errors, expected)

	percent := analyzer.types["Percent"]
	if !percent.Named {
		t.Fatal("Percent should be registered as named type")
	}
	if percent.Underlying != "int" {
		t.Fatalf("wrong underlying type. got=%q want=%q", percent.Underlying, "int")
	}
}

func TestContractTypeRequiresExplicitConversionFromVariable(t *testing.T) {
	input := `
type Percent int range 0..100

let _a: int := 90
let _tooMuch: int := 101
let mut precent: Percent := 0
fn Test() void {
	try precent += 50
	try precent += _a
	try precent = Percent(_a)
	try precent = Percent(_tooMuch)
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot add int to Percent at 9:17",
		"value 101 violates range contract Percent 0..100 at 11:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestNamedRangeTypeStoresContract(t *testing.T) {
	input := `
module main

type Percent int range 0..100
`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	analyzer := NewAnalyzer()
	errors := analyzer.Analyze(program)
	assertSemaErrors(t, errors, nil)

	percent := analyzer.types["Percent"]
	if !percent.Named {
		t.Fatal("Percent should be named")
	}
	if percent.Underlying != "int" {
		t.Fatalf("wrong underlying type. got=%q want=%q", percent.Underlying, "int")
	}
	if len(percent.Contracts) != 1 {
		t.Fatalf("wrong contract count. got=%d want=1", len(percent.Contracts))
	}

	contract, ok := percent.Contracts[0].(RangeContract)
	if !ok {
		t.Fatalf("contract is not RangeContract. got=%T", percent.Contracts[0])
	}

	if contract.Min.String() != "0" || contract.Max.String() != "100" {
		t.Fatalf("wrong range contract. got=%s..%s want=0..100", contract.Min, contract.Max)
	}
}

func TestNamedUnitTypeRegistryStoresUnit(t *testing.T) {
	input := `
type Money decimal<SEK>
type Speed decimal<m/s>
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	money := analyzer.types["Money"]
	if !money.Named {
		t.Fatal("Money should be named")
	}
	if money.Underlying != "decimal" {
		t.Fatalf("wrong underlying type. got=%q want=%q", money.Underlying, "decimal")
	}
	if money.Unit != "SEK" {
		t.Fatalf("wrong unit. got=%q want=%q", money.Unit, "SEK")
	}

	speed := analyzer.types["Speed"]
	if speed.Unit != "m/s" {
		t.Fatalf("wrong unit. got=%q want=%q", speed.Unit, "m/s")
	}
}

func TestUnitDeclarationRegistersSemanticUnit(t *testing.T) {
	input := `
module main

unit Hertz decimal<Hz>
unit Packet uint other
type Frequency decimal<hertz>

let hz: Hertz := 10
let f: Frequency := 10
let p: Packet := 3u
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	hertz := analyzer.types["Hertz"]
	if !hertz.Named || hertz.Kind != DecimalType || hertz.Unit != "Hertz" {
		t.Fatalf("wrong Hertz type: %+v", hertz)
	}
	if !hertz.Dimension.Equal(analyzer.types["Frequency"].Dimension) {
		t.Fatalf("Hertz and hertz should share physical dimension. got=%+v want=%+v", hertz.Dimension, analyzer.types["Frequency"].Dimension)
	}

	if unit := analyzer.units["Hertz"]; unit.Category != PhysicalUnit {
		t.Fatalf("Hertz should be a physical unit. got=%q", unit.Category)
	}

	packet := analyzer.types["Packet"]
	if !packet.Named || packet.Kind != UintType || packet.Unit != "Packet" {
		t.Fatalf("wrong Packet type: %+v", packet)
	}
	if unit := analyzer.units["Packet"]; unit.Category != OtherUnit {
		t.Fatalf("Packet should be an other unit. got=%q", unit.Category)
	}
	if packet.Dimension.Base["Packet"] != 1 {
		t.Fatalf("Packet should keep semantic base dimension. got=%+v", packet.Dimension)
	}
}

func TestUnitDeclarationRejectsNonNumericStorage(t *testing.T) {
	input := `
module main

unit Bad string
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unit Bad must use numeric storage, got string at 4:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestRegisterDeclarationAndAddressedInstance(t *testing.T) {
	input := `
module main

unit rpm uint physical

type MotorProtocol register[8] {
	Speed: bit[4]<rpm>,
	Enabled: bit,
	_: bit[3],
}

@address(0x40021000)
let mut motorProtocol: MotorProtocol

fn StartMotor(speed: rpm) void {
	motorProtocol.Speed = speed
	motorProtocol.Enabled = true
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	register := analyzer.types["MotorProtocol"]
	if register.Kind != RegisterType || register.RegisterWidth != 8 {
		t.Fatalf("wrong register type: %+v", register)
	}
	if len(register.RegisterFields) != 3 || register.RegisterFields[0].Name != "Speed" || register.RegisterFields[0].Width != 4 {
		t.Fatalf("wrong register fields: %+v", register.RegisterFields)
	}
	symbol := analyzer.symbols["motorProtocol"]
	if !symbol.Addressed || !symbol.Volatile || symbol.Address != "0x40021000" {
		t.Fatalf("wrong addressed symbol: %+v", symbol)
	}
}

func TestRegisterValidationErrors(t *testing.T) {
	input := `
module main

unit rpm uint physical

type InvalidProtocol register[8] {
	Speed: bit[4],
	Enabled: bit,
	Mode: bit[2],
}

type BadField register[8] {
	Value: bit[0],
	_: bit[8],
}

type MotorStatus register[8] {
	Running: bit,
	Fault: bit,
	_: bit[6],
}

type MotorProtocol register[8] {
	Speed: bit[4]<rpm>,
	Enabled: bit,
	_: bit[3],
}

@address(0x40021000)
let motorStatus: MotorStatus

@address(0x40021001)
let mut motorProtocol: MotorProtocol

fn Test() void {
	let reserved := motorStatus._
	motorStatus.Running = true
	motorProtocol.Speed = 19
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"register InvalidProtocol declares 8 bits but its fields occupy 7 bits at 6:22",
		"register field BadField.Value width must be positive at 13:2",
		"reserved register field _ cannot be accessed at 36:30",
		"cannot assign to field Running on read-only addressed register motorStatus at 37:14",
		"value 19 overflows rpm at 38:24",
	}

	assertSemaErrors(t, errors, expected)
}

func TestUnitAliasTypesAreNominal(t *testing.T) {
	input := `
type Money decimal<SEK>
type Speed decimal<m/s>

let mut mo: Money := 5.90
let mut sp: Speed := 50
fn Test() void {
	mo += sp
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot add Speed to Money at 8:8",
	}

	assertSemaErrors(t, errors, expected)
}

func TestUnitDivisionCanInferDeclaredDimensionType(t *testing.T) {
	input := `
type Meter decimal<m>
type Second decimal<s>
type Speed decimal<m/s>

let d: Meter := 100
let t: Second := 9.58
let v := d / t
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	v := analyzer.symbols["v"]
	if v.Type.Name != "Speed" {
		t.Fatalf("wrong inferred type. got=%q want=%q", v.Type.Name, "Speed")
	}
}

func TestUnitMultiplicationCanInferDeclaredDimensionType(t *testing.T) {
	input := `
type Meter decimal<m>
type Second decimal<s>
type Speed decimal<m/s>

let v: Speed := 10
let t: Second := 2
let d := v * t
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	d := analyzer.symbols["d"]
	if d.Type.Name != "Meter" {
		t.Fatalf("wrong inferred type. got=%q want=%q", d.Type.Name, "Meter")
	}
}

func TestUnitTypeMultipliedByIntKeepsUnitType(t *testing.T) {
	input := `
type Money decimal<SEK>

let m: Money := 10
let doubled := m * 2
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	doubled := analyzer.symbols["doubled"]
	if doubled.Type.Name != "Money" {
		t.Fatalf("wrong inferred type. got=%q want=%q", doubled.Type.Name, "Money")
	}
}

func TestMoneyTimesMoneyIsInvalid(t *testing.T) {
	input := `
type Money decimal<SEK>

let a: Money := 10
let b: Money := 2
let invalid := a * b
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot multiply Money by Money at 6:18",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSameUnitDivisionInfersDecimal(t *testing.T) {
	input := `
type Money decimal<SEK>

let a: Money := 10
let b: Money := 2
let ratio := a / b
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	ratio := analyzer.symbols["ratio"]
	if ratio.Type.Name != "decimal" {
		t.Fatalf("wrong inferred type. got=%q want=%q", ratio.Type.Name, "decimal")
	}
}

func TestImplicitLetDefinesImmutableSymbol(t *testing.T) {
	input := `
let mut a := 10
let b := 9
fn Test() void {
	b = a
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot assign to immutable variable b at 5:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestBuiltinTypes(t *testing.T) {
	analyzer := NewAnalyzer()

	if decimal := analyzer.types["decimal"]; decimal.Kind != DecimalType {
		t.Fatalf("decimal has wrong type kind: %q", decimal.Kind)
	}

	for _, name := range []string{"bytes", "date", "datetime", "duration", "time"} {
		if _, exists := analyzer.types[name]; exists {
			t.Errorf("%s must not be a builtin type", name)
		}
	}
}

func TestRedeclarationDoesNotReplaceExistingSymbol(t *testing.T) {
	input := `
let mut a: uint := 5
let a: int := 1
fn Test() void {
	a = -1
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"variable \"a\" already declared at 3:5, previous declaration at 2:9",
		"value -1 overflows uint at 5:6",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIntegerLiteralRanges(t *testing.T) {
	input := `
let a: int8 := 200
let b: uint8 := 255
let c: uint8 := 256
let d: uint := -1
let e: int8 := -128
let f: int8 := -129
`

	errors := analyzeSource(t, input)

	expected := []string{
		"value 200 overflows int8 at 2:16",
		"value 256 overflows uint8 at 4:17",
		"value -1 overflows uint at 5:16",
		"value -129 overflows int8 at 7:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestTypeRangeBoundsFitBaseType(t *testing.T) {
	input := `
type Valid int8 range -128..127
type MaxOverflow int8 range 0..1000
type MinOverflow int8 range -129..100
type UintMinOverflow uint8 range -1..
type OpenRange uint8 range ..255
`

	errors := analyzeSource(t, input)

	expected := []string{
		"value 1000 overflows int8 at 3:32",
		"value -129 overflows int8 at 4:29",
		"value -1 overflows uint8 at 5:34",
	}

	assertSemaErrors(t, errors, expected)
}

func TestStructTypeDeclarationRegistersNamedType(t *testing.T) {
	input := `
type Meter decimal<m>

type Coordinate struct {
	x: Meter,
	y: Meter,
	z: Meter,
}

let mut c: Coordinate
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	coordinate := analyzer.types["Coordinate"]
	if !coordinate.Named {
		t.Fatal("Coordinate should be registered as named type")
	}
	if coordinate.Kind != StructType {
		t.Fatalf("wrong type kind. got=%q want=%q", coordinate.Kind, StructType)
	}
	if len(coordinate.Fields) != 3 {
		t.Fatalf("wrong field count. got=%d want=3", len(coordinate.Fields))
	}
}

func TestStructDuplicateFields(t *testing.T) {
	input := `
type Bad struct {
	x: int,
	x: int,
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate field "x" in struct Bad at 4:2`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestStructUnknownFieldType(t *testing.T) {
	input := `
type Bad struct {
	x: UnknownType,
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown type UnknownType at 3:5",
	}

	assertSemaErrors(t, errors, expected)
}

func TestStructFieldTagsArePreserved(t *testing.T) {
	input := `
type User struct {
	ID: int ` + "`json:\"id\" db:\"user_id\"`" + `,
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	user := analyzer.types["User"]
	if len(user.Fields) != 1 {
		t.Fatalf("wrong field count. got=%d want=1", len(user.Fields))
	}
	if len(user.Fields[0].Tags) != 2 {
		t.Fatalf("wrong tag count. got=%d want=2", len(user.Fields[0].Tags))
	}
	if user.Fields[0].Tags[1].Key != "db" || user.Fields[0].Tags[1].Value != "user_id" {
		t.Fatalf("wrong db tag: %+v", user.Fields[0].Tags[1])
	}
}

func TestStructFieldRangeContract(t *testing.T) {
	input := `
type User struct {
	Active: bool,
	Name: string,
	Age: int range 0..130,
}

let ok := User{ Active: true, Name: "Ada", Age: 42 }
let bad := User{ Active: true, Name: "Ada", Age: 131 }
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)

	expected := []string{
		"value 131 violates range contract int 0..130 at 9:50",
	}

	assertSemaErrors(t, errors, expected)

	user := analyzer.types["User"]
	if len(user.Fields) != 3 {
		t.Fatalf("wrong field count. got=%d want=3", len(user.Fields))
	}
	if len(user.Fields[2].Type.Contracts) != 1 {
		t.Fatalf("Age should have one range contract, got %d", len(user.Fields[2].Type.Contracts))
	}
}

func TestImplTargetMustExist(t *testing.T) {
	input := `
impl MissingType {
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown impl target MissingType at 2:6",
	}

	assertSemaErrors(t, errors, expected)
}

func TestMultipleImplBlocksAllowed(t *testing.T) {
	input := `
type Vehicle struct {
}

impl Vehicle {
}

impl Vehicle {
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDuplicateNestedTypeAcrossImplBlocks(t *testing.T) {
	input := `
type Vehicle struct {
}

impl Vehicle {
	enum FuelType {
		petrol,
	}
}

impl Vehicle {
	enum FuelType {
		diesel,
	}
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"duplicate nested type \"FuelType\" in impl Vehicle at 12:7",
	}
	assertSemaErrors(t, errors, expected)
}

func TestNestedEnumForwardReference(t *testing.T) {
	input := `
type Vehicle struct {
	fuel_type: Vehicle.FuelType,
}

impl Vehicle {
	enum FuelType {
		petrol,
		diesel,
		electric,
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	typ := analyzer.types["Vehicle"]
	if len(typ.Fields) != 1 || typ.Fields[0].Type.Name != "Vehicle.FuelType" {
		t.Fatalf("wrong field type: %+v", typ.Fields)
	}
	enumType := analyzer.types["Vehicle.FuelType"]
	if enumType.Kind != EnumType || len(enumType.EnumValues) != 3 {
		t.Fatalf("wrong enum type: %+v", enumType)
	}
}

func TestNestedTypeRequiresQualifiedNameOutsideImpl(t *testing.T) {
	input := `
type Vehicle struct {
	fuel_type: FuelTypes,
}

impl Vehicle {
	enum FuelTypes {
		Petrol,
	}
}

let mut ft: FuelTypes := FuelTypes.Petrol
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown type FuelTypes at 3:13",
		"unknown type FuelTypes at 12:13",
	}

	assertSemaErrors(t, errors, expected)
}

func TestNestedTypeShortNameInsideImpl(t *testing.T) {
	input := `
type Vehicle struct {
	fuel_type: Vehicle.FuelTypes,
}

impl Vehicle {
	enum FuelTypes {
		Petrol,
	}

	property FuelType: FuelTypes {
		get {
			return FuelTypes.Petrol
		}
	}
}

let mut ft: Vehicle.FuelTypes := Vehicle.FuelTypes.Petrol
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestNestedEnumDuplicateValue(t *testing.T) {
	input := `
type Vehicle struct {
}

impl Vehicle {
	enum FuelType {
		petrol,
		petrol,
	}
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate enum value "petrol" in enum Vehicle.FuelType at 8:3`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestEnumValuesAndNamespaceConstants(t *testing.T) {
	input := `
enum Color {
	red,
	green,
	blue,
}

enum Status int {
	unknown = 0,
	active = 10,
	paused,
	disabled = 99,
}

let c: Color := Color.red
let s: Status := Status.paused
let explicitColor: Color := Color(1)
let explicitInt: int := int(Color.green)
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	color := analyzer.types["Color"]
	if color.Kind != EnumType || color.EnumConsts["green"].Value.String() != "1" {
		t.Fatalf("wrong Color enum: %+v", color)
	}

	status := analyzer.types["Status"]
	if status.Kind != EnumType || status.EnumConsts["paused"].Value.String() != "11" {
		t.Fatalf("wrong Status enum: %+v", status)
	}

	if analyzer.symbols["c"].Type.Name != "Color" {
		t.Fatalf("wrong c type: %+v", analyzer.symbols["c"])
	}
	if analyzer.symbols["explicitInt"].Type.Name != "int" {
		t.Fatalf("wrong explicitInt type: %+v", analyzer.symbols["explicitInt"])
	}
}

func TestEnumImplicitIntegerAssignmentsAreInvalid(t *testing.T) {
	input := `
enum Color {
	red,
	green,
}

let c: Color := 1
let i: int := Color.red
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot initialize Color with int at 7:17",
		"cannot initialize int with Color at 8:20",
	}

	assertSemaErrors(t, errors, expected)
}

func TestEnumUnderlyingTypeErrors(t *testing.T) {
	input := `
enum BadUnknown Missing {
	a,
}

enum BadString string {
	a,
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown type Missing at 2:17",
		"enum BadString underlying type must be integer, got string at 6:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestEnumInitializerMustBeIntegerConstant(t *testing.T) {
	input := `
let x := 1

enum Bad {
	a = x,
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"enum value Bad.a initializer must be integer constant at 5:6",
	}

	assertSemaErrors(t, errors, expected)
}

func TestImplPropertyChecks(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
	}

	property TopSpeed: Missing {
		set value {
			_speed = value
		}
	}
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate property "TopSpeed" in impl Vehicle at 15:11`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestImplPropertyUnknownType(t *testing.T) {
	input := `
type Vehicle struct {
}

impl Vehicle {
	property Missing: UnknownType {
		get {
		}
	}
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown type UnknownType at 6:20",
	}

	assertSemaErrors(t, errors, expected)
}

func TestStructLiteralAndFieldAccess(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

let speed: Speed := 10
let vehicle := Vehicle{ _speed: speed }
let current := vehicle._speed
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	current := analyzer.symbols["current"]
	if current.Type.Name != "Speed" {
		t.Fatalf("wrong member type. got=%q want=Speed", current.Type.Name)
	}
}

func TestStructLiteralFieldErrors(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

let bad := Vehicle{ missing: Speed(1), _speed: "fast" }
`

	errors := analyzeSource(t, input)

	expected := []string{
		`unknown field "missing" in struct Vehicle at 8:21`,
		"cannot initialize field _speed with string at 8:48",
	}

	assertSemaErrors(t, errors, expected)
}

func TestPropertyAccessAndFallibleAssignment(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

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

let speed: Speed := 10
let mut vehicle := Vehicle{ _speed: speed }
let current := vehicle.TopSpeed
fn Test() void {
	vehicle.TopSpeed = speed
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)

	expected := []string{
		"assigning fallible property TopSpeed requires try at 23:10",
	}

	assertSemaErrors(t, errors, expected)

	current := analyzer.symbols["current"]
	if current.Type.Name != "Speed" {
		t.Fatalf("wrong property type. got=%q want=Speed", current.Type.Name)
	}
}

func TestTryAssignmentHandlersUseImplicitOk(t *testing.T) {
	input := `
type Speed decimal<m/s>

enum IOError {
	InvalidValue,
}

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
		try set value {
			return Err(IOError.InvalidValue)
		}
	}
}

fn Log(error: IOError) void {
	return
}

fn Test(car: Vehicle, current_speed: Speed) void {
	try car.TopSpeed = current_speed {
		Err(error) => Log(error)
	}
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestTryAssignmentAllowsExplicitOkHandler(t *testing.T) {
	input := `
type Speed decimal<m/s>

enum IOError {
	InvalidValue,
}

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
		try set value {
			return Err(IOError.InvalidValue)
		}
	}
}

fn Log(error: IOError) void {
	return
}

fn Test(car: Vehicle, current_speed: Speed) void {
	try car.TopSpeed = current_speed {
		Ok(_) => {}
		Err(error) => Log(error)
	}
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestTryExpressionAllowsExplicitOkHandler(t *testing.T) {
	input := `
enum IOError {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(10)
}

fn Test() int {
	let value := try Read() {
		Ok(v) => v
		Err(error) => 0
	}

	return value
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPropertyAccessBeforeImplDeclaration(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

let speed: Speed := 10
let vehicle := Vehicle{ _speed: speed }
let current := vehicle.TopSpeed

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	current := analyzer.symbols["current"]
	if current.Type.Name != "Speed" {
		t.Fatalf("wrong property type. got=%q want=Speed", current.Type.Name)
	}
}

func TestPropertyBodyCanReferenceLaterPropertyInSameImpl(t *testing.T) {
	input := `
type Speed decimal<m/s>

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property Current: Speed {
		get {
			return TopSpeed
		}
	}

	property TopSpeed: Speed {
		get {
			return _speed
		}
	}
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPropertyGetterCanReturnSelf(t *testing.T) {
	input := `
type Counter struct {
	value: int,
}

impl Counter {
	property Whole: Counter {
		get {
			return self
		}
	}
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestPropertyBodyChecks(t *testing.T) {
	input := `
type Speed decimal<m/s>
type Money decimal<SEK>

type Vehicle struct {
	_speed: Speed,
}

impl Vehicle {
	property TopSpeed: Speed {
		get {
			return _speed
		}
	}

	property MoneyValue: Money {
		get {
			return Money(5.90)
		}
	}

	property BadGet: Speed {
		get {
			return Money(5.90)
		}
	}

	property MissingGet: Speed {
		get {
			_speed = _speed
		}
	}

	property UnknownGet: Speed {
		get {
			return missing
		}
	}

	property BadSet: Speed {
		set value {
			return Err(IOError.InvalidValue)
		}
	}
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"getter BadGet must return Speed, got Money at 24:11",
		"getter MissingGet must return Speed at 28:11",
		"non-fallible setter BadSet cannot return Err at 41:3",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionDeclarations(t *testing.T) {
	input := `
fn add(a: int, b: int) int {
	return a + b
}

fn noop() void {
	return
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	add := analyzer.functions["add"][0]
	if add.ReturnType.Name != "int" || len(add.Parameters) != 2 {
		t.Fatalf("wrong add function: %+v", add)
	}

	noop := analyzer.functions["noop"][0]
	if noop.ReturnType.Name != "void" || len(noop.Parameters) != 0 {
		t.Fatalf("wrong noop function: %+v", noop)
	}
}

func TestFunctionOverloads(t *testing.T) {
	input := `
fn Pick(a: int) int {
	return a
}

fn Pick(a: string) string {
	return a
}

let i := Pick(1)
let s := Pick("hello")
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	if len(analyzer.functions["Pick"]) != 2 {
		t.Fatalf("wrong overload count. got=%d want=2", len(analyzer.functions["Pick"]))
	}
	if analyzer.symbols["i"].Type.Name != "int" {
		t.Fatalf("wrong i type: %+v", analyzer.symbols["i"])
	}
	if analyzer.symbols["s"].Type.Name != "string" {
		t.Fatalf("wrong s type: %+v", analyzer.symbols["s"])
	}
}

func TestDuplicateFunctionSignature(t *testing.T) {
	input := `
fn Pick(a: int) int {
	return a
}

fn Pick(value: int) int {
	return value
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate function "Pick" with same signature at 6:4`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestReturnTypeCannotDistinguishOverload(t *testing.T) {
	input := `
fn Pick(a: int) int {
	return a
}

fn Pick(value: int) string {
	return "value"
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate function "Pick" with same signature at 6:4`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestAmbiguousFunctionOverload(t *testing.T) {
	input := `
type Percent int range 0..100
type Score int range 0..100

fn Pick(a: Percent) Percent {
	return a
}

fn Pick(a: Score) Score {
	return a
}

let p := Pick(10)
`

	errors := analyzeSource(t, input)

	expected := []string{
		"ambiguous call to Pick at 13:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestOverloadExactMatchPreferredOverConversion(t *testing.T) {
	input := `
fn Print(value: int) int {
	return value
}

fn Print(value: int64) int64 {
	return value
}

let x := Print(10)
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	if analyzer.symbols["x"].Type.Name != "int" {
		t.Fatalf("wrong x type. got=%q want=int", analyzer.symbols["x"].Type.Name)
	}
}

func TestOverloadNamedTypesRemainDistinct(t *testing.T) {
	input := `
type Percent int range 0..100

fn Set(value: int) int {
	return value
}

fn Set(value: Percent) Percent {
	return value
}

let p: Percent := 50
let selectedNamed := Set(p)
let selectedInt := Set(50)
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	if analyzer.symbols["selectedNamed"].Type.Name != "Percent" {
		t.Fatalf("wrong selectedNamed type. got=%q want=Percent", analyzer.symbols["selectedNamed"].Type.Name)
	}
	if analyzer.symbols["selectedInt"].Type.Name != "int" {
		t.Fatalf("wrong selectedInt type. got=%q want=int", analyzer.symbols["selectedInt"].Type.Name)
	}
}

func TestFunctionCalls(t *testing.T) {
	input := `
fn Add(a: int, b: int) int {
	return a + b
}

let x := Add(1, 2)
let y: int := Add(3, 4)
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	if analyzer.symbols["x"].Type.Name != "int" {
		t.Fatalf("wrong x type: %+v", analyzer.symbols["x"])
	}
	if analyzer.symbols["y"].Type.Name != "int" {
		t.Fatalf("wrong y type: %+v", analyzer.symbols["y"])
	}
}

func TestFunctionCallTypeErrors(t *testing.T) {
	input := `
fn Add(a: int, b: int) int {
	return a + b
}

let v: float := Add(2, 4)
let w: int := Add(1.5, 1.5)
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot initialize float with int at 6:17",
		"argument 1 to Add must be int, got decimal at 7:19",
		"argument 2 to Add must be int, got decimal at 7:24",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionCallWrongArgumentCount(t *testing.T) {
	input := `
fn Add(a: int, b: int) int {
	return a + b
}

let wrongCount := Add(1)
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function Add expects 2 arguments, got 1 at 6:19",
	}

	assertSemaErrors(t, errors, expected)
}

func TestCallSyntaxStillSupportsConversions(t *testing.T) {
	input := `
enum Color {
	red,
	green,
}

let explicitColor: Color := Color(1)
let explicitInt: int := int(Color.green)
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestExplicitNumericToBoolConversions(t *testing.T) {
	input := `
let a: bool := bool(0)
let b: bool := bool(1)
let c: bool := bool(-1)
let d: bool := bool(0.0)
let e: bool := bool(0.1)
let i8: int8 := -1
let u8: uint8 := 1
let f32: float32 := 0.0
let x: bool := bool(i8)
let y: bool := bool(u8)
let z: bool := bool(f32)

fn Test(value: int) void {
	if value {
	}
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"if condition must be bool, got int at 15:5",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionDuplicateParameter(t *testing.T) {
	input := `
fn bad(a: int, a: int) int {
	return a
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		`duplicate parameter "a" at 2:16`,
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionMustReturn(t *testing.T) {
	input := `
fn bad() int {
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function bad must return int at 2:4",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionReturnTypeMismatch(t *testing.T) {
	input := `
fn bad() int {
	return true
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function bad must return int, got bool at 3:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionBoolExpressions(t *testing.T) {
	input := `
fn IsPositive(a: int) bool {
	return a > 0
}

fn Logic(a: bool, b: bool) bool {
	return !a || (a && b)
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestLogicalOperatorsRequireBool(t *testing.T) {
	input := `
fn BadAnd(a: int) bool {
	return a && true
}

fn BadNot(a: int) bool {
	return !a
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"operator && requires bool operands at 3:11",
		"operator ! requires bool operand at 7:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestEqualityRequiresCompatibleTypes(t *testing.T) {
	input := `
fn ValidBoolEquality(value: bool, number: int) void {
	if value == bool(number) {
	}
	return
}

fn InvalidBoolEquality(value: bool, number: int) void {
	if value == number {
	}
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot compare bool and int at 9:11",
	}

	assertSemaErrors(t, errors, expected)
}

func TestFunctionUnknownReturnTypeDoesNotCascade(t *testing.T) {
	input := `
fn UnknownReturn() UnknownType {
	return 0
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"unknown type UnknownType at 2:20",
	}

	assertSemaErrors(t, errors, expected)
}

func TestResultReturnExpressions(t *testing.T) {
	input := `
enum IOError {
	InvalidValue,
}

fn OkResult() Result[int, IOError] {
	return Ok(1)
}

fn VoidOkResult() Result[void, IOError] {
	return Ok()
}

fn ErrResult() Result[int, IOError] {
	return Err(IOError.InvalidValue)
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestResultReturnExpressionErrors(t *testing.T) {
	input := `
enum IOError {
	InvalidValue,
}

fn BadOk() Result[int, IOError] {
	return Ok(IOError.InvalidValue)
}

fn MissingOkValue() Result[int, IOError] {
	return Ok()
}

fn BadErr() Result[int, IOError] {
	return Err(1)
}

fn Plain() Result[int, IOError] {
	return 1
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function BadOk must return Ok(int), got Ok(IOError) at 7:19",
		"function MissingOkValue must return Ok(int), got Ok() at 11:9",
		"function BadErr must return Err(IOError), got Err(int) at 15:13",
		"function Plain returning Result[int, IOError] must return Ok(...) or Err(...) at 19:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestTryResultExpression(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>

enum IOError {
	FileNotFound,
	AccessDenied,
	InvalidValue,
}

fn CalculateSpeed() Result[Speed, IOError] {
	return Ok(Speed(42.5))
}

fn FailCalculation() Result[Speed, IOError] {
	return Err(IOError.InvalidValue)
}

fn UseResult() Result[Speed, IOError] {
	let speed := try CalculateSpeed()

	return Ok(speed)
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestResultAndTryErrors(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>

enum IOError {
	FileNotFound,
	AccessDenied,
	InvalidValue,
}

fn WrongOkType() Result[Speed, IOError] {
	return Ok(IOError.InvalidValue)
}

fn WrongErrType() Result[Speed, IOError] {
	return Err(Speed(10))
}

fn PlainReturn() Result[Speed, IOError] {
	return Speed(10)
}

fn InvalidTry() Speed {
	let speed := try Speed(10)

	return speed
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function WrongOkType must return Ok(Speed), got Ok(IOError) at 13:19",
		"function WrongErrType must return Err(IOError), got Err(Speed) at 17:13",
		"function PlainReturn returning Result[Speed, IOError] must return Ok(...) or Err(...) at 21:9",
		"try requires Result expression at 25:15",
	}

	assertSemaErrors(t, errors, expected)
}

func TestTryRequiresCompatibleFunctionResultContext(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>

enum IOError {
	InvalidValue,
}

enum ParseError {
	InvalidNumber,
}

fn ReadSpeed() Result[Speed, IOError] {
	return Ok(Speed(10))
}

fn ParseSpeed() Result[Speed, ParseError] {
	return Err(ParseError.InvalidNumber)
}

fn WrongPropagation() Result[Speed, IOError] {
	let speed := try ParseSpeed()
	return Ok(speed)
}

fn CannotPropagate() Speed {
	return try ReadSpeed()
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot propagate ParseError from function returning Result[Speed, IOError] at 23:15",
		"cannot use try in function returning Speed at 28:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestTryHandlersCanHandleLocally(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>

enum IOError {
	InvalidValue,
}

enum ParseError {
	InvalidNumber,
}

fn ParseSpeed() Result[Speed, ParseError] {
	return Err(ParseError.InvalidNumber)
}

fn UseFallback() Speed {
	let speed := try ParseSpeed() {
		Err(error) => Speed(0)
	}
	return speed
}

fn ConvertError() Result[Speed, IOError] {
	let speed := try ParseSpeed() {
		Err(error) => return Err(IOError.InvalidValue)
	}
	return Ok(speed)
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestTryHandlersCanUseExplicitMatchWrapper(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>

enum IOError {
	InvalidValue,
}

fn ReadSpeed() Result[Speed, IOError] {
	return Err(IOError.InvalidValue)
}

fn UseFallback() Speed {
	let speed := try ReadSpeed() {
		match {
			Err(IOError.InvalidValue) => Speed(0)
			Err(error) => Speed(1)
		}
	}
	return speed
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestTryHandlerErrors(t *testing.T) {
	input := `
module main

type Speed decimal<m/s>
type Money decimal<SEK>

enum IOError {
	InvalidValue,
	AccessDenied,
}

fn ReadSpeed() Result[Speed, IOError] {
	return Err(IOError.InvalidValue)
}

fn WrongFallback() Speed {
	let speed := try ReadSpeed() {
		Err(error) => Money(0)
	}
	return speed
}

fn MissingHandler() Speed {
	let speed := try ReadSpeed() {
		Err(IOError.InvalidValue) => Speed(0)
	}
	return speed
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"try handler must produce Speed, got Money at 18:17",
		"non-exhaustive try handlers for IOError at 24:15",
	}

	assertSemaErrors(t, errors, expected)
}

func TestErrorHandlingValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/errorhandling_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestDeferStatementValid(t *testing.T) {
	input := `
fn Cleanup() void {
	return
}

fn Test() void {
	let mut value := 1
	defer {
		value = 2
		Cleanup()
	}
	value = 3
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDeferInvalidControlFlow(t *testing.T) {
	input := `
fn Test() void {
	defer {
		break
	}
	defer {
		continue
	}
	defer {
		fallthrough
	}
	defer {
		defer {
		}
	}
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"break is not allowed inside defer at 4:3",
		"continue is not allowed inside defer at 7:3",
		"fallthrough is not allowed inside defer at 10:3",
		"defer is not allowed inside defer at 13:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestDeferReturnWarnsAsSuperfluous(t *testing.T) {
	input := `
fn Test() void {
	defer {
		return
	}
	defer return
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	warnings := analyzer.Warnings()
	expected := []string{
		"superfluous defer return at 3:2",
		"superfluous defer return at 6:2",
	}
	if len(warnings) != len(expected) {
		t.Fatalf("wrong warning count. got=%d warnings=%v", len(warnings), warnings)
	}
	for i, want := range expected {
		if warnings[i].Error() != want {
			t.Fatalf("wrong warning %d. got=%q want=%q", i, warnings[i].Error(), want)
		}
	}
}

func TestDeferRejectsTryPropagation(t *testing.T) {
	input := `
enum IOError {
	Failed,
}

fn Close() Result[int, IOError] {
	return Ok(0)
}

fn Test() Result[int, IOError] {
	defer {
		try Close()
	}
	return Ok(1)
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"try cannot propagate from inside defer at 12:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestDeferRejectsUnhandledResultExpression(t *testing.T) {
	input := `
enum CleanupError {
	failed,
}

type Resource struct {
	id: int,
}

fn CloseResource(resource: Resource) Result[void, CleanupError] {
	return Ok()
}

fn Test(resource: Resource) void {
	defer {
		CloseResource(resource)
	}
}
`

	errors := analyzeSource(t, input)
	expected := []string{
		"unhandled Result inside defer; handle it or discard it explicitly at 16:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestDeferOutsideFunction(t *testing.T) {
	input := `
module main

defer {
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"defer is only valid inside functions at 4:1",
	}
	assertSemaErrors(t, errors, expected)
}

func TestDeferInsideLoopWarning(t *testing.T) {
	input := `
fn Test() void {
	for {
		defer {
		}
		break
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	warnings := analyzer.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("wrong warning count. got=%d warnings=%v", len(warnings), warnings)
	}
	want := "defer inside loop registers once per execution and runs at function exit at 4:3"
	if warnings[0].Error() != want {
		t.Fatalf("wrong warning. got=%q want=%q", warnings[0].Error(), want)
	}
}

func TestEnumValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/enum_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestGenericStructAndFunctionHeaders(t *testing.T) {
	input := `
module main

type Stack[T] struct {
	value: T,
}

type Pair[A, B] struct {
	first: A,
	second: B,
}

fn Identity[T](value: T) T {
	return value
}

fn Use(value: Stack[int], pair: Pair[string, int]) void {
}

fn Read(value: Stack[int]) int {
	return value.value
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	stack := analyzer.types["Stack"]
	if len(stack.GenericParameters) != 1 || stack.GenericParameters[0] != "T" {
		t.Fatalf("wrong Stack generic params: %+v", stack.GenericParameters)
	}
	if len(stack.Fields) != 1 || stack.Fields[0].Type.Kind != GenericType || stack.Fields[0].Type.Name != "T" {
		t.Fatalf("wrong Stack field type: %+v", stack.Fields)
	}

	functions := analyzer.functions["Identity"]
	if len(functions) != 1 {
		t.Fatalf("wrong Identity function count: %d", len(functions))
	}
	if functions[0].Parameters[0].Type.Kind != GenericType || functions[0].ReturnType.Kind != GenericType {
		t.Fatalf("Identity did not preserve generic types: %+v", functions[0])
	}

	read := analyzer.functions["Read"][0]
	paramType := read.Parameters[0].Type
	if typeDisplayName(paramType) != "Stack[int]" {
		t.Fatalf("wrong instantiated Stack type: %+v display=%s", paramType, typeDisplayName(paramType))
	}
	if len(paramType.Fields) != 1 || paramType.Fields[0].Type.Name != "int" {
		t.Fatalf("Stack[int] field was not substituted: %+v", paramType.Fields)
	}
}

func TestGenericHeaderErrors(t *testing.T) {
	input := `
module main

type Duplicate[T, T] struct {
	value: T,
}

type Box[T] struct {
	value: T,
}

fn BadArity(value: Box[int, string]) void {
}

fn NonGeneric(value: int[string]) void {
}

fn MissingArgs(value: Box) void {
}
`

	_, errors := analyzeSourceWithAnalyzerRaw(t, input)
	expected := []string{
		`duplicate generic parameter "T" at 4:19`,
		"Box requires 1 generic arguments, got 2 at 12:20",
		"int is not generic at 15:22",
		"Box requires 1 generic arguments, got 0 at 18:23",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericStructInstantiationsAreDistinct(t *testing.T) {
	input := `
module main

type Box[T] struct {
	value: T,
}

fn Bad(value: Box[int]) Box[string] {
	return value
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"function Bad must return Box[string], got Box[int] at 9:9",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericFunctionCallInference(t *testing.T) {
	input := `
module main

type Box[T] struct {
	value: T,
}

fn Identity[T](value: T) T {
	return value
}

fn Unbox[T](box: Box[T]) T {
	return box.value
}

fn UseIdentity() int {
	let value := Identity(10)
	return value
}

fn UseNested(box: Box[string]) string {
	return Unbox(box)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestGenericFunctionInferenceErrors(t *testing.T) {
	input := `
module main

fn Same[T](left: T, right: T) T {
	return left
}

fn Test() int {
	return Same(1, "hello")
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot infer generic arguments for Same at 9:9",
	}
	assertSemaErrors(t, errors, expected)
}

func TestExplicitGenericFunctionCall(t *testing.T) {
	input := `
module main

fn Identity[T](value: T) T {
	return value
}

fn UseInt() int {
	return Identity[int](10)
}

fn UseString() string {
	return Identity[string]("hello")
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestExplicitGenericFunctionCallErrors(t *testing.T) {
	input := `
module main

fn Identity[T](value: T) T {
	return value
}

fn NonGeneric(value: int) int {
	return value
}

fn WrongArgument() int {
	return Identity[int]("hello")
}

fn WrongArity() int {
	return Identity[int, string](10)
}

fn GenericArgumentsOnNonGenericFunction() int {
	return NonGeneric[int](10)
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"argument 1 to Identity must be int, got string at 13:23",
		"Identity requires 1 explicit generic arguments, got 2 at 17:9",
		"function NonGeneric is not generic at 21:9",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericInstanceCaching(t *testing.T) {
	input := `
module main

type Box[T] struct {
	value: T,
}

fn TakeBoxes(first: Box[int], second: Box[int], third: Box[string]) void {
}

fn Identity[T](value: T) T {
	return value
}

fn Use() void {
	let a := Identity(1)
	let b := Identity(2)
	let c := Identity("hello")
	let d := Identity[int](3)
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	if got := len(analyzer.genericTypeInstances); got != 2 {
		t.Fatalf("wrong generic type instance count. got=%d instances=%+v", got, analyzer.genericTypeInstances)
	}
	if got := len(analyzer.genericFuncInstances); got != 2 {
		t.Fatalf("wrong generic function instance count. got=%d instances=%+v", got, analyzer.genericFuncInstances)
	}
}

func TestGenericDirectRecursiveStorageErrors(t *testing.T) {
	input := `
module main

type Node[T] struct {
	next: Node[T],
}

fn Use(value: Node[int]) void {
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"recursive generic type Node[T] has infinite size at 5:2",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericArrayRecursiveStorageErrors(t *testing.T) {
	input := `
module main

type Node[T] struct {
	children: [2]Node[T],
}

fn Use(value: Node[int]) void {
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"recursive generic type Node[T] has infinite size at 5:2",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericChangingRecursiveInstantiationErrors(t *testing.T) {
	input := `
module main

type Infinite[T] struct {
	next: Infinite[[]T],
}

fn Use(value: Infinite[int]) void {
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"recursive generic instantiation does not converge for Infinite at 5:2",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericSliceRecursiveStorageAllowed(t *testing.T) {
	input := `
module main

type Node[T] struct {
	children: []Node[T],
}

fn Use(value: Node[int]) void {
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestGenericImplProperty(t *testing.T) {
	input := `
module main

type Box[T] struct {
	value: T,
}

impl Box[T] {
	property Value: T {
		get {
			return value
		}
	}
}

fn Use(box: Box[int]) int {
	return box.Value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestGenericImplErrors(t *testing.T) {
	input := `
module main

type Box[T] struct {
	value: T,
}

impl Box[U] {
}

impl Box {
}

impl Box[T] {
	fn Map[U](value: U) void {
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"unknown generic parameter U in impl target Box at 8:10",
		"Box requires 1 generic arguments, got 0 at 11:6",
		"generic methods with additional type parameters are not supported yet at 15:5",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericExpectedResultInference(t *testing.T) {
	input := `
module main

enum IOError {
	failed,
}

fn Fail[T]() Result[T, IOError] {
	return Err(IOError.failed)
}

fn UseLet() void {
	let value: Result[int, IOError] := Fail()
}

fn UseAssignment() void {
	let mut value: Result[string, IOError]
	value = Fail()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestGenericExpectedResultInferenceDoesNotOverrideArguments(t *testing.T) {
	input := `
module main

enum IOError {
	failed,
}

fn Wrap[T](value: T) Result[T, IOError] {
	return Ok(value)
}

fn Bad() void {
	let value: Result[string, IOError] := Wrap(1)
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"cannot initialize Result[string, IOError] with Result[int, IOError] at 13:40",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericConstraintDeclarationErrors(t *testing.T) {
	input := `
module main

type BadTypeConstraint[T: int] struct {
	value: T,
}

type UnknownTypeConstraint[T: MissingConstraint] struct {
	value: T,
}

fn BadFunctionConstraint[T: string](value: T) void {
	discard value
}

fn UnknownFunctionConstraint[T: MissingConstraint](value: T) void {
	discard value
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"generic constraint int is not an interface at 4:27",
		"unknown generic constraint MissingConstraint for T at 8:31",
		"generic constraint string is not an interface at 12:29",
		"unknown generic constraint MissingConstraint for T at 16:33",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericUnionTypeReferences(t *testing.T) {
	input := `
module main

type Maybe[T] union {
	Some(T)
	None
}

fn Empty[T]() Maybe[T] {
	return Maybe.None
}

fn Use() Maybe[int] {
	return Empty[int]()
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	maybe := analyzer.types["Maybe"]
	if maybe.Kind != UnionType || len(maybe.GenericParameters) != 1 {
		t.Fatalf("wrong Maybe type: %+v", maybe)
	}
	use := analyzer.functions["Use"][0]
	if typeDisplayName(use.ReturnType) != "Maybe[int]" {
		t.Fatalf("wrong Use return type: %+v display=%s", use.ReturnType, typeDisplayName(use.ReturnType))
	}
	if len(use.ReturnType.UnionVariants) != 2 || use.ReturnType.UnionVariants[0].Payload == nil || use.ReturnType.UnionVariants[0].Payload.Name != "int" {
		t.Fatalf("wrong instantiated union variants: %+v", use.ReturnType.UnionVariants)
	}
}

func TestGenericUnionPayloadVariantRequiresPayload(t *testing.T) {
	input := `
module main

type Maybe[T] union {
	Some(T)
	None
}

fn Bad() Maybe[int] {
	return Maybe.Some
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"union variant Maybe[int].Some requires payload at 10:15",
	}
	assertSemaErrors(t, errors, expected)
}

func TestGenericUnionPayloadConstructor(t *testing.T) {
	input := `
module main

type Maybe[T] union {
	Some(T)
	None
}

fn SomeInt() Maybe[int] {
	return Maybe.Some(10)
}

fn LetSome() void {
	let value: Maybe[string] := Maybe.Some("hello")
	let inferred := Maybe.Some(10)
	discard value
	discard inferred
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestGenericUnionPayloadConstructorErrors(t *testing.T) {
	input := `
module main

type Maybe[T] union {
	Some(T)
	None
}

fn WrongPayload() Maybe[int] {
	return Maybe.Some("hello")
}

fn MissingPayload() Maybe[int] {
	return Maybe.Some()
}

fn ExtraPayload() Maybe[int] {
	return Maybe.None(1)
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"union variant Maybe[int].Some payload must be int, got string at 10:20",
		"union variant Maybe[int].Some expects 1 argument, got 0 at 14:14",
		"union variant Maybe[int].None expects 0 arguments, got 1 at 18:14",
	}
	assertSemaErrors(t, errors, expected)
}

func TestUnionValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/union_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestUnionInvalidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/union_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	if len(errors) == 0 {
		t.Fatal("expected union_invalid.sec to produce semantic errors")
	}
}

func TestGenericsValidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/generics_valid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))
	assertSemaErrors(t, errors, nil)
}

func TestGenericsInvalidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/generics_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, supportedGenericsInvalidFixture(string(input)))
	expected := []string{
		`duplicate generic parameter "T" at 11:19`,
		"unknown generic parameter U in impl target Box at 105:10",
		"Box requires 1 generic arguments, got 0 at 108:6",
		"recursive generic type Node[T] has infinite size at 98:5",
		"recursive generic instantiation does not converge for Infinite at 102:5",
		"generic methods with additional type parameters are not supported yet at 112:8",
		"Box requires 1 generic arguments, got 0 at 61:35",
		"Box requires 1 generic arguments, got 2 at 64:35",
		"int is not generic at 67:44",
		"function DifferentInstantiations must return Box[string], got Box[int] at 71:12",
		"cannot infer generic arguments for Same at 76:12",
		"argument 1 to Identity must be int, got string at 82:26",
		"Identity requires 1 explicit generic arguments, got 2 at 86:12",
		"function NonGeneric is not generic at 94:12",
	}
	assertSemaErrors(t, errors, expected)
}

func supportedGenericsInvalidFixture(input string) string {
	const marker = "// -----------------------------------------------------------------------------\n// Duplicate generic parameters\n// -----------------------------------------------------------------------------"
	idx := strings.LastIndex(input, marker)
	if idx < 0 {
		return input
	}
	return input[:idx]
}

func TestErrorHandlingMatchInvalidFixture(t *testing.T) {
	input, err := os.ReadFile("../../testdata/errorhandling_match_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}

	errors := analyzeSourceRaw(t, string(input))

	expected := []string{
		"non-exhaustive match for Result[Speed, IOError]: missing Err at 17:18",
		"non-exhaustive match for IOError at 25:17",
		"match arms must produce compatible types, got int and string at 36:29",
		"match pattern must match Result[Speed, IOError], got IOError at 45:16",
		"catch-all pattern may not hide Err at 44:18",
		"duplicate match arm for IOError.InvalidValue at 55:9",
		"unreachable match arm at 65:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestMatchRequiresAtLeastOneBranch(t *testing.T) {
	input := `
module main

enum IOError {
	InvalidValue,
}

fn Test(error: IOError) void {
	match error {
	}
	return
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"match requires at least one branch at 9:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestMatchCatchAllMayNotHideResultErr(t *testing.T) {
	input := `
module main

enum IOError {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() int {
	let value := match Read() {
		Ok(value) => value
		_ => 0
	}
	return value
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"catch-all pattern may not hide Err at 13:15",
	}

	assertSemaErrors(t, errors, expected)
}

func TestMatchGuardMustBeBool(t *testing.T) {
	input := `
module main

enum IOError {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() int {
	let value := match Read() {
		Ok(value) where value => value
		Ok(value) => value
		Err(error) => 0
	}
	return value
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"match guard must be bool, got int at 14:19",
	}

	assertSemaErrors(t, errors, expected)
}

func TestGuardedMatchArmDoesNotExhaustPattern(t *testing.T) {
	input := `
module main

enum IOError {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test(flag: bool) int {
	let value := match Read() {
		Ok(value) where flag => value
		Err(error) => 0
	}
	return value
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"non-exhaustive match for Result[int, IOError]: missing Ok at 13:15",
	}

	assertSemaErrors(t, errors, expected)
}

func TestMatchDuplicateResultArms(t *testing.T) {
	input := `
module main

enum IOError {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn DuplicateOk() int {
	return match Read() {
		Ok(value) => value
		Ok(other) => other
		Err(error) => 0
	}
}

fn DuplicateErr() int {
	return match Read() {
		Ok(value) => value
		Err(error) => 0
		Err(other) => 1
	}
}
`

	errors := analyzeSourceRaw(t, input)
	expected := []string{
		"duplicate match arm for Result[int, IOError].Ok at 15:3",
		"duplicate match arm for Result[int, IOError].Err at 24:3",
	}
	assertSemaErrors(t, errors, expected)
}

func TestMatchDiscardSuccessPayloadAndExplicitErrorDiscard(t *testing.T) {
	input := `
module main

enum IOError {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() void {
	match Read() {
		Ok(_) => {
		}
		Err(error) => {
			discard error
		}
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMatchStatementReturnArmsSatisfyFunctionReturn(t *testing.T) {
	input := `
module main

enum IOError {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() int {
	match Read() {
		Ok(value) => return value
		Err(error) => return 0
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMatchStatementAssignmentsMergeAcrossExhaustiveArms(t *testing.T) {
	input := `
module main

enum Direction {
	North,
	East,
}

enum IOError {
	InvalidValue,
}

fn AllEnumArmsAssign(direction: Direction) int {
	let mut result: int

	match direction {
		Direction.North => {
			result = 1
		}
		Direction.East => {
			result = 2
		}
	}

	return result
}

fn ResultReturningArmAssigns(result: Result[int, IOError]) int {
	let mut value: int

	match result {
		Ok(number) => {
			value = number
		}
		Err(error) => {
			return 0
		}
	}

	return value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestMatchErrDiscardPatternIsInvalid(t *testing.T) {
	input := `
module main

enum IOError {
	InvalidValue,
}

fn Read() Result[int, IOError] {
	return Ok(1)
}

fn Test() int {
	return match Read() {
		Err(_) => 0
		Ok(value) => value
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"Err payload must be named; use discard name inside the handler at 14:7",
	}

	assertSemaErrors(t, errors, expected)
}

func TestDiscardRequiresDefinedName(t *testing.T) {
	input := `
module main

fn Test() void {
	discard error
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"undefined variable error at 5:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIfConditionsAndBranchScopes(t *testing.T) {
	input := `
module main

fn Test(ready: bool, score: int) int {
	let mut result := 0
	if ready && score >= 10 {
		let branchOnly := 1
		result = branchOnly
	} else if !ready {
		result = 2
	} else {
		result = 3
	}
	return result
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestIfConditionMustBeBool(t *testing.T) {
	input := `
module main

enum Status {
	Active,
}

fn Count() int {
	return 10
}

fn Test(value: int, name: string, status: Status) void {
	if value {
	}
	if name {
	}
	if status {
	}
	if Count() {
	}
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"if condition must be bool, got int at 13:5",
		"if condition must be bool, got string at 15:5",
		"if condition must be bool, got Status at 17:5",
		"if condition must be bool, got int at 19:5",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIfBranchesCanSatisfyFunctionReturn(t *testing.T) {
	input := `
module main

fn Sign(value: int) int {
	if value < 0 {
		return -1
	} else {
		return 1
	}
}

fn Grade(score: int) int {
	if score >= 90 {
		return 1
	} else if score >= 80 {
		return 2
	} else {
		return 3
	}
}

fn Early(value: int) int {
	if value < 0 {
		return 0
	}
	return value
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestIfWithoutElseDoesNotSatisfyFunctionReturn(t *testing.T) {
	input := `
module main

fn Missing(value: int) int {
	if value < 0 {
		return 0
	}
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function Missing must return int at 4:4",
	}

	assertSemaErrors(t, errors, expected)
}

func TestElseIfWithoutFinalElseDoesNotSatisfyFunctionReturn(t *testing.T) {
	input := `
module main

fn MissingReturnAfterElseIf(value: int) int {
	if value < 0 {
		return -1
	} else if value == 0 {
		return 0
	}
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"function MissingReturnAfterElseIf must return int at 4:4",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIfRangeMembershipCondition(t *testing.T) {
	input := `
module main

type Percent int range 0..100

fn Test(score: int, percent: Percent) void {
	if score in 80..<100 {
	}
	if score in 1.. {
	}
	if score in ..100 {
	}
	if percent in Percent(1)..<Percent(100) {
	}
	return
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestIfRangeMembershipTypeMismatch(t *testing.T) {
	input := `
module main

type Percent int range 0..100

fn Test(percent: Percent) void {
	let lower := 1
	if percent in lower..100 {
	}
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot test Percent in range of int at 8:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestInvalidNegatedRangeDoesNotCascade(t *testing.T) {
	input := `
module main

fn InvalidNegatedRange(score: string) void {
	if !(score in 0..100) {
	}
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot test string in range of int at 5:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIfBranchLocalDoesNotLeakToFollowingCall(t *testing.T) {
	input := `
module main

fn ScopeTest(value: bool) void {
	if value {
		let local: int := 10
	}

	println(local)
}

fn println(s: string) void {
	return
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"undefined variable local at 9:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIfDefiniteAssignment(t *testing.T) {
	input := `
module main

fn AssignedInBothBranches(value: bool) int {
	let mut result: int

	if value {
		result = 10
	} else {
		result = 20
	}

	return result
}

fn MissingAssignment(value: bool) int {
	let mut result: int

	if value {
		result = 10
	}

	return result
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"variable result is unassigned at 23:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestIfDefiniteAssignmentOnlyRequiresContinuingPaths(t *testing.T) {
	input := `
module main

fn AssignmentInAllContinuingPaths(value: bool) int {
	let mut result: int

	if value {
		result = 10
	} else {
		return 20
	}

	return result
}

fn AssignmentInElseOnlyWhenThenReturns(value: bool) int {
	let mut result: int

	if value {
		return 20
	} else {
		result = 10
	}

	return result
}
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSwitchValidCasesAndDefiniteAssignment(t *testing.T) {
	input := `
module main

fn Select(value: int) int {
	let mut result: int

	switch value {
	case < 0:
		result = -1
	case 0, 1, 2..<10:
		result = 10
	default:
		result = 20
	}

	return result
}

fn Subjectless(value: int) int {
	switch {
	case value < 0:
		return -1
	case value == 0:
		return 0
	default:
		return 1
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSwitchFallthroughIntoReturningCaseSatisfiesReturn(t *testing.T) {
	input := `
module main

fn FallthroughIntoReturningCase(value: int) int {
	switch value {
	case 1:
		fallthrough
	case 2:
		return 20
	default:
		return 30
	}
}

fn MultipleFallthrough(value: int) int {
	switch value {
	case 1:
		fallthrough
	case 2:
		fallthrough
	case 3:
		return 10
	default:
		return 20
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSwitchSemanticErrors(t *testing.T) {
	input := `
module main

fn Invalid(value: int, name: string) void {
	switch value {
	case "one":
		return
	case 1:
		return
	case 1:
		return
	}

	switch {
	case 10:
		return
	}

	switch value {
	case 3:
		fallthrough
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"switch case must be compatible with subject type int, got string at 6:7",
		"duplicate switch case value 1 at 10:7",
		"subjectless switch case must be bool, got int at 15:7",
		"fallthrough is not allowed in the final switch case at 21:3",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSwitchCaseReportsUnreachableAfterReturn(t *testing.T) {
	input := `
module main

fn UnreachableAfterReturn(value: int) int {
	switch value {
	case 1:
		return 10
		let local: int := 20
	default:
		return 30
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unreachable code at 8:3",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSwitchDefaultPlacementErrorsAreSemaErrors(t *testing.T) {
	input := `
module main

fn MultipleDefault(value: int) void {
	switch value {
	case 1:
	default:
	default:
	}
}

fn DefaultNotFinal(value: int) void {
	switch value {
	default:
	case 1:
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"default must be the final switch clause at 8:2",
		"switch may contain only one default clause at 8:2",
		"default must be the final switch clause at 15:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSwitchConstantCoverageErrors(t *testing.T) {
	input := `
module main

fn Invalid(value: int) void {
	switch value {
	case 0..100:
		return
	case 50:
		return
	case 40..120:
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"switch case value 50 is already covered by previous case at 8:7",
		"switch case range overlaps previous case at 10:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSwitchDescendingRangeIsNormalized(t *testing.T) {
	input := `
module main

fn Valid(value: int) void {
	switch value {
	case 10..0:
		return
	}
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)
	if len(analyzer.Warnings()) != 0 {
		t.Fatalf("expected no warnings. got=%v", analyzer.Warnings())
	}
}

func TestSwitchRelationalCoveredCasesAreUnreachable(t *testing.T) {
	input := `
module main

fn Invalid(value: int) void {
	switch value {
	case >= 0:
		return
	case > 10:
		return
	}

	switch value {
	case <= 100:
		return
	case <= 5:
		return
	}

	switch value {
	case 0..100:
		return
	case > 50:
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unreachable switch case; previous case already covers this condition at 8:7",
		"unreachable switch case; previous case already covers this condition at 15:7",
		"unreachable switch case; previous case already covers this condition at 22:7",
	}

	assertSemaErrors(t, errors, expected)
}

func TestSwitchPartialRelationalOverlapIsAllowed(t *testing.T) {
	input := `
module main

fn Valid(value: int) int {
	switch value {
	case <= -10:
		return -2
	case < 0:
		return -1
	case 0:
		return 0
	case < 10:
		return 1
	case >= 10:
		return 2
	default:
		return 3
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSwitchAdjacentExclusiveRangesAreAllowed(t *testing.T) {
	input := `
module main

fn Valid(value: int) void {
	switch value {
	case 0..<10:
		return
	case 10..<20:
		return
	case < 0:
		return
	case >= 20:
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestBoolSwitchCanBeExhaustiveWithoutDefault(t *testing.T) {
	input := `
module main

fn BoolReturn(value: bool) int {
	switch value {
	case true:
		return 1
	case false:
		return 0
	}
}

fn BoolAssignment(value: bool) int {
	let mut result: int

	switch value {
	case true:
		result = 1
	case false:
		result = 0
	}

	return result
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestBoolSwitchDuplicateCase(t *testing.T) {
	input := `
module main

fn Invalid(value: bool) void {
	switch value {
	case true:
		return
	case true:
		return
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"duplicate switch case value true at 8:7",
	}

	assertSemaErrors(t, errors, expected)
}

func TestAsmRequiresUnsafe(t *testing.T) {
	input := `
module main

fn Valid() void {
	unsafe {
		asm "nop"
	}
	return
}

fn Invalid() void {
	asm "nop"
	return
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"asm is only allowed inside unsafe at 12:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestExternFunctionRequiresUnsafeCall(t *testing.T) {
	input := `
module main

extern "C" fn write(fd: int32, buffer: RawPtr[byte], length: uint) int64

fn Bad(buffer: RawPtr[byte]) void {
	let result := write(1, buffer, 4u)
}

fn Good(buffer: RawPtr[byte]) void {
	unsafe {
		let result := write(1, buffer, 4u)
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"calling extern function write requires unsafe at 7:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestExternFunctionValidation(t *testing.T) {
	input := `
module main

extern "Rust" fn badABI(value: int32) int32
extern "C" fn badParam(value: string) int32
extern "C" fn badReturn() string
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unknown extern ABI \"Rust\" at 4:1",
		"extern C parameter 1 value has non-ABI-compatible type string at 5:24",
		"extern C function badReturn has non-ABI-compatible return type string at 6:15",
	}

	assertSemaErrors(t, errors, expected)
}

func TestRawPtrConversionRequiresUnsafe(t *testing.T) {
	input := `
module main

fn Bad(address: uint) RawPtr[byte] {
	return RawPtr[byte](address)
}

fn Good(address: uint) RawPtr[byte] {
	unsafe {
		return RawPtr[byte](address)
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"conversion involving RawPtr requires unsafe at 5:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestInlineAsmBlockCanReturnInt64(t *testing.T) {
	input := `
module main

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

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestUnsafeFunctionAllowsAsmAndNamedOutput(t *testing.T) {
	input := `
module platform_linux_amd64

type ErrorNumber int

fn _decodeSyscallResult(result: int) Result[uint, ErrorNumber] {
	if result < 0 {
		return Err(ErrorNumber(-result))
	}

	return Ok(uint(result))
}

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

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestStringPtrAndLenMembers(t *testing.T) {
	input := `
module main

fn _sysWrite(fd: int64, ref ptr: byte, len: int64) int64 {
	unsafe {
		asm {
			"syscall"
			inputs: rax(1), rdi(fd), rsi(ptr), rdx(len)
			outputs: rax
		}
	}
}

fn Println(s: string) void {
	_sysWrite(1, s.ptr, s.len)

	let nl := "\n"
	_sysWrite(1, nl.ptr, 1)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestResultTypeArgumentCountErrors(t *testing.T) {
	input := `
module main

enum IOError {
	InvalidValue,
}

fn MissingResultArgument() Result[int] {
	return Ok(1)
}

fn TooManyResultArguments() Result[int, IOError, string] {
	return Ok(1)
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"Result requires exactly 2 type arguments, got 1 at 8:28",
		"Result requires exactly 2 type arguments, got 3 at 12:29",
	}

	assertSemaErrors(t, errors, expected)
}

func TestForRangeLoopIsValid(t *testing.T) {
	input := `
module main

fn Test() void {
	for i in 0..<10 {
		let x := i
	}

	return
}

fn DecimalRange() void {
	let start: decimal := 0.001
	let end: decimal := 0.002
	let increment: decimal := 0.00001

	for value in start..<end step increment {
		let copy: decimal := value
	}
}

fn FloatRange() void {
	let start: float := 0.0
	let end: float := 1.0
	let increment: float := 0.1

	for value in start..<end step increment {
		let copy: float := value
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestForArrayAndSliceLoopBindings(t *testing.T) {
	input := `
module main

fn ArrayLoop(values: [3]int) void {
	for value in values {
		let copy: int := value
	}
}

fn SliceLoop(values: []int) void {
	for value in values {
		let copy: int := value
	}
}

fn SliceIndexValueLoop(values: []int) void {
	for index, value in values {
		let i: int := index
		let copy: int := value
	}
}

fn StringIndexValueLoop(values: string) void {
	for index, value in values {
		let i: int := index
		let r: rune := value
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestForTooManySequentialBindings(t *testing.T) {
	input := `
module main

fn Test(values: []int) void {
	for a, b, c in values {
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"sequential iteration supports one or two loop bindings, got 3 at 5:6",
	}

	assertSemaErrors(t, errors, expected)
}

func TestWhileConditionMustBeBool(t *testing.T) {
	input := `
module main

fn Test() void {
	while 1 {
	}
	return
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"while condition must be bool, got int at 5:8",
	}

	assertSemaErrors(t, errors, expected)
}

func TestWhileBodyAssignmentsDoNotLeak(t *testing.T) {
	input := `
module main

fn Test(running: bool) int {
	let mut result: int

	while running {
		result = 1
	}

	return result
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"variable result is unassigned at 11:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestWhileTrueWithoutBreakSatisfiesReturn(t *testing.T) {
	input := `
module main

fn Test() int {
	while true {
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestWhileTrueWithBreakRequiresReturnAfterLoop(t *testing.T) {
	input := `
module main

fn Test() int {
	while true {
		break
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"function Test must return int at 4:4",
	}

	assertSemaErrors(t, errors, expected)
}

func TestWhileTrueBreakCarriesDefiniteAssignment(t *testing.T) {
	input := `
module main

fn Test() int {
	let mut result: int

	while true {
		result = 10
		break
	}

	return result
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestWhileTrueBreakWithoutAssignmentDoesNotSatisfyDefiniteAssignment(t *testing.T) {
	input := `
module main

fn Test() int {
	let mut result: int

	while true {
		break
	}

	return result
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"variable result is unassigned at 11:9",
	}

	assertSemaErrors(t, errors, expected)
}

func TestUnreachableStatementsInsideWhileBody(t *testing.T) {
	input := `
module main

fn UnreachableAfterBreak() void {
	while true {
		break
		let value: int := 10
	}
}

fn UnreachableAfterContinue() void {
	while true {
		continue
		let value: int := 10
	}
}

fn UnreachableAfterReturn() int {
	while true {
		return 10
		let value: int := 20
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unreachable code at 7:3",
		"unreachable code at 14:3",
		"unreachable code at 21:3",
	}

	assertSemaErrors(t, errors, expected)
}

func TestComparisonChainingIsRejected(t *testing.T) {
	input := `
module main

fn InvalidComparisonChain(value: int) void {
	while 0 <= value < 100 {
		break
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"comparison chaining is not supported at 5:19",
	}

	assertSemaErrors(t, errors, expected)
}

func TestForLoopVariableIsImmutableAndScoped(t *testing.T) {
	input := `
module main

fn Test() void {
	for i in 0..<10 {
		i = 1
	}

	let x := i
	return
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"cannot assign to immutable variable i at 6:3",
		"undefined variable i at 9:11",
	}

	assertSemaErrors(t, errors, expected)
}

func TestForOpenEndedRangeIsRejected(t *testing.T) {
	input := `
module main

fn Test() void {
	for i in 0.. {
	}
	return
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"range used in for loop must be finite at 5:12",
	}

	assertSemaErrors(t, errors, expected)
}

func TestForRangeBoundsMustHaveSameType(t *testing.T) {
	input := `
module main

fn InvalidStringBounds(end: string) void {
	for i in 0..<end {
	}
}

fn InvalidMixedBounds(start: int, end: uint) void {
	for i in start..<end {
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"cannot create range with bounds int and string at 5:12",
		"cannot create range with bounds int and uint at 10:16",
	}

	assertSemaErrors(t, errors, expected)
}

func TestForRangeStep(t *testing.T) {
	input := `
module main

fn ValidStep() void {
	for i in 0..<10 step 2 {
		let copy: int := i
	}
}

fn InvalidZeroStep() void {
	for i in 0..<10 step 0 {
	}
}

fn InvalidNegativeStep() void {
	for i in 0..<10 step -1 {
	}
}

fn InvalidStepOnSlice(values: []int) void {
	for value in values step 2 {
	}
}

fn InvalidDecimalZeroStep() void {
	for value in 0.0..<1.0 step 0.0 {
	}
}

fn InvalidDecimalNegativeStep() void {
	for value in 0.0..<1.0 step -0.1 {
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"for range step must be greater than zero at 11:23",
		"for range step must be greater than zero at 16:23",
		"for step is only valid for range iteration at 21:27",
		"for range step must be greater than zero at 26:30",
		"for range step must be greater than zero at 31:30",
	}

	assertSemaErrors(t, errors, expected)
}

func TestBreakAndContinueRequireLoop(t *testing.T) {
	input := `
module main

fn Test() void {
	break
	continue
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"break is only valid inside a loop at 5:2",
		"continue is only valid inside a loop at 6:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestInfiniteForWithoutBreakSatisfiesReturn(t *testing.T) {
	input := `
module main

fn Test() int {
	for {
	}
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestInfiniteForWithContinueMakesFollowingReturnUnreachable(t *testing.T) {
	input := `
module main

fn Test() int {
	let mut value: int

	for {
		value = 10
		continue
	}

	return value
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"unreachable code at 12:2",
	}

	assertSemaErrors(t, errors, expected)
}

func TestLambdaFunctionValueCall(t *testing.T) {
	input := `
module main

fn Test() int {
	let double := fn(value: int) int {
		return value * 2
	}

	return double(10)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestTypedLambdaVariableAndFunctionValueCall(t *testing.T) {
	input := `
module main

fn Test() bool {
	let positive: fn(int) bool := fn(value: int) bool {
		return value > 0
	}

	return positive(10)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestLambdaArgumentToFunctionValueParameter(t *testing.T) {
	input := `
module main

fn Apply(value: int, callback: fn(int) int) int {
	return callback(value)
}

fn Test() int {
	return Apply(
		10,
		fn(value: int) int {
			return value * 2
		},
	)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestLambdaInvalidReturnType(t *testing.T) {
	input := `
module main

fn Test() void {
	let operation := fn() int {
		return true
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"lambda must return int, got bool at 6:10",
	}

	assertSemaErrors(t, errors, expected)
}

func TestLambdaFunctionAssignmentMismatch(t *testing.T) {
	input := `
module main

fn Test() void {
	let operation: fn(int) bool := fn(value: string) bool {
		return value != ""
	}
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"cannot initialize fn(int) bool with fn(string) bool at 5:33",
	}

	assertSemaErrors(t, errors, expected)
}

func TestLambdaImplicitCaptureIsRejected(t *testing.T) {
	input := `
module main

fn Test(factor: int) int {
	let multiply := fn(value: int) int {
		return value * factor
	}

	return multiply(10)
}
`

	errors := analyzeSourceRaw(t, input)

	expected := []string{
		"lambda cannot access outer variable factor without explicit capture at 6:18",
	}

	assertSemaErrors(t, errors, expected)
}

func TestNamedFunctionValueIsAssignable(t *testing.T) {
	input := `
module main

fn IsPositive(value: int) bool {
	return value > 0
}

fn Test() bool {
	let predicate := IsPositive
	return predicate(10)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestOverloadedNamedFunctionValueUsesExplicitTargetType(t *testing.T) {
	input := `
module main

fn Convert(value: int) string {
	return "int"
}

fn Convert(value: string) int {
	return 1
}

fn Test() string {
	let converter: fn(int) string := Convert
	return converter(10)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func analyzeSource(t *testing.T, input string) []Error {
	t.Helper()

	_, errors := analyzeSourceWithAnalyzer(t, input)
	return errors
}

func analyzeSourceWithAnalyzer(t *testing.T, input string) (*Analyzer, []Error) {
	t.Helper()
	return analyzeSourceWithAnalyzerRaw(t, ensureModuleForTest(input))
}

func analyzeSourceRaw(t *testing.T, input string) []Error {
	t.Helper()

	_, errors := analyzeSourceWithAnalyzerRaw(t, input)
	return errors
}

func analyzeSourceWithAnalyzerRaw(t *testing.T, input string) (*Analyzer, []Error) {
	t.Helper()

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	analyzer := NewAnalyzer()
	return analyzer, analyzer.Analyze(program)
}

func ensureModuleForTest(input string) string {
	if strings.Contains(input, "module ") {
		return input
	}
	return input + "\nmodule test\n"
}

func parseExpressionSource(t *testing.T, input string) ast.Expression {
	t.Helper()

	l := lexer.New("let value := " + input)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("wrong statement count. got=%d want=1", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("statement is not LetStatement. got=%T", program.Statements[0])
	}

	return stmt.Value
}

func assertSemaErrors(t *testing.T, errors []Error, expected []string) {
	t.Helper()

	if len(errors) != len(expected) {
		t.Fatalf("wrong sema error count. got=%d want=%d errors=%v", len(errors), len(expected), errors)
	}

	for i, expectedError := range expected {
		if errors[i].Error() != expectedError {
			t.Fatalf("wrong sema error %d. got=%q want=%q", i, errors[i].Error(), expectedError)
		}
	}
}
