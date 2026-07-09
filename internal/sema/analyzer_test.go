package sema

import (
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
p += .1
p += 0.1
`

	errors := analyzeSource(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDecimalDoesNotImplicitlyAcceptFloatVariables(t *testing.T) {
	input := `
let f: float64 := 3.14
let d: decimal := f
let mut p: decimal := 1
p += f
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot initialize decimal with float64 at 3:19",
		"cannot add float64 to decimal at 5:6",
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
a = u - 6
let u: uint := 7
u = 1
missing = 1
`

	errors := analyzeSource(t, input)

	expected := []string{
		"variable \"u\" already declared at 7:5, previous declaration at 5:5",
		"cannot assign to immutable variable u at 8:1",
		"undefined variable missing at 9:1",
	}

	assertSemaErrors(t, errors, expected)
}

func TestAnalyzeLetGroups(t *testing.T) {
	input := `
int mut: a, b, c
float: f := 5.4, pi := 3.14
let x := 9, s := "hello", ok := true
let mut ma := 9, ms := "hello", mok := false
a = x
ma = 10
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
p = 100
p = 101
p = 10 * 10
p = 50 + 51
p = 50
p += 20
p += 60
`

	errors := analyzeSource(t, input)

	expected := []string{
		"value 101 violates range contract Percent 0..100 at 6:5",
		"value 101 violates range contract Percent 0..100 at 8:8",
		"value 130 violates range contract Percent 0..100 at 11:6",
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
precent += 50
precent += _a
precent = Percent(_a)
precent = Percent(_tooMuch)
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot add int to Percent at 8:12",
		"value 101 violates range contract Percent 0..100 at 10:11",
	}

	assertSemaErrors(t, errors, expected)
}

func TestNamedRangeTypeStoresContract(t *testing.T) {
	input := `
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

func TestUnitAliasTypesAreNominal(t *testing.T) {
	input := `
type Money decimal<SEK>
type Speed decimal<m/s>

let mut mo: Money := 5.90
let mut sp: Speed := 50
mo += sp
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot add Speed to Money at 7:7",
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
b = a
`

	errors := analyzeSource(t, input)

	expected := []string{
		"cannot assign to immutable variable b at 4:1",
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
a = -1
`

	errors := analyzeSource(t, input)

	expected := []string{
		"variable \"a\" already declared at 3:5, previous declaration at 2:9",
		"value -1 overflows uint at 4:5",
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

func TestDuplicateImplBlock(t *testing.T) {
	input := `
type Vehicle struct {
}

impl Vehicle {
}

impl Vehicle {
}
`

	errors := analyzeSource(t, input)

	expected := []string{
		"duplicate impl block for Vehicle at 8:6",
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
vehicle.TopSpeed = speed
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)

	expected := []string{
		"assigning fallible property TopSpeed requires try at 22:9",
	}

	assertSemaErrors(t, errors, expected)

	current := analyzer.symbols["current"]
	if current.Type.Name != "Speed" {
		t.Fatalf("wrong property type. got=%q want=Speed", current.Type.Name)
	}
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

	add := analyzer.functions["add"]
	if add.ReturnType.Name != "int" || len(add.Parameters) != 2 {
		t.Fatalf("wrong add function: %+v", add)
	}

	noop := analyzer.functions["noop"]
	if noop.ReturnType.Name != "void" || len(noop.Parameters) != 0 {
		t.Fatalf("wrong noop function: %+v", noop)
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
		"function BadErr must return Err(IOError), got Err(int) at 11:13",
		"function Plain returning Result[int, IOError] must return Ok(...) or Err(...) at 15:9",
	}

	assertSemaErrors(t, errors, expected)
}

func analyzeSource(t *testing.T, input string) []Error {
	t.Helper()

	_, errors := analyzeSourceWithAnalyzer(t, input)
	return errors
}

func analyzeSourceWithAnalyzer(t *testing.T, input string) (*Analyzer, []Error) {
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
