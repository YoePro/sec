package sema

import (
	"math/big"
	"testing"
)

func TestTriviallyDestructiblePrimitiveAndViewTypes(t *testing.T) {
	for _, typ := range []Type{
		{Name: "bool", Kind: BoolType},
		{Name: "byte", Kind: UintType},
		{Name: "char", Kind: CharType},
		{Name: "rune", Kind: RuneType},
		{Name: "int", Kind: IntType},
		{Name: "uint", Kind: UintType},
		{Name: "float", Kind: FloatType},
		{Name: "decimal", Kind: DecimalType},
		{Name: "string", Kind: StringType},
		{Name: "RawPtr", Kind: RawPtrType, TypeArgs: []Type{{Name: "byte", Kind: UintType}}},
		{Name: "ref int", Kind: ReferenceType, Element: &Type{Name: "int", Kind: IntType}},
		{Name: "fn", Kind: FunctionType},
	} {
		if !TriviallyDestructible(typ) {
			t.Fatalf("%s should be trivially destructible", typ.Name)
		}
	}
}

func TestTriviallyDestructibleAggregates(t *testing.T) {
	intType := Type{Name: "int", Kind: IntType}
	boolType := Type{Name: "bool", Kind: BoolType}
	enumType := Type{Name: "Direction", Kind: EnumType, EnumValues: []string{"north", "south"}}
	structType := Type{
		Name: "Point",
		Kind: StructType,
		Fields: []StructField{
			{Name: "x", Type: intType},
			{Name: "ok", Type: boolType},
			{Name: "dir", Type: enumType},
		},
	}
	arrayType := NewFixedArrayType(intType, big.NewInt(4))
	unionType := Type{
		Name: "MaybePoint",
		Kind: UnionType,
		UnionVariants: []UnionVariant{
			{Name: "Some", Payload: &structType},
			{Name: "None"},
			{
				Name: "Named",
				PayloadFields: []StructField{
					{Name: "value", Type: intType},
				},
			},
		},
	}
	resultType := Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{structType, enumType}}

	for _, typ := range []Type{structType, arrayType, unionType, resultType} {
		if !TriviallyDestructible(typ) {
			t.Fatalf("%s should be trivially destructible", typ.Name)
		}
	}
}

func TestTriviallyDestructibleCurrentResolvedTypes(t *testing.T) {
	input := `
module main

unit m

enum IOError error {
    failed,
}

type Point struct {
    x: int,
    distance: <m>,
}

type MaybePoint union {
    Some(Point),
    None,
}

fn Read() Result[Point, IOError] {
    return Err(IOError.failed)
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	if len(errors) > 0 {
		t.Fatalf("Analyze returned errors: %v", errors)
	}

	for _, name := range []string{"m", "IOError", "Point", "MaybePoint"} {
		typ, ok := analyzer.types[name]
		if !ok {
			t.Fatalf("missing type %s", name)
		}
		if !TriviallyDestructible(typ) {
			t.Fatalf("%s should be trivially destructible: %+v", name, typ)
		}
	}

	if len(analyzer.functions["Read"]) != 1 {
		t.Fatalf("missing Read function: %+v", analyzer.functions["Read"])
	}
	resultType := analyzer.functions["Read"][0].ReturnType
	if !TriviallyDestructible(resultType) {
		t.Fatalf("Result[Point, IOError] should be trivially destructible: %+v", resultType)
	}
}

func TestFreeOperationIsReservedUntilDestructionRulesAreImplemented(t *testing.T) {
	input := `
module main

type File struct {
    handle: int,
}

impl File {
    free {
        Close(handle)
    }
}

fn Close(handle: int) void {
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"free operations are reserved for destruction but are not implemented yet at 9:5",
	})
}

func TestOpenFileMustBeClosedBeforeScopeExit(t *testing.T) {
	input := `
module io

enum IOError error {
    BadFileDescriptor,
}

type File struct {
    is_closed: bool,
}

fn MakeFile() File {
    return File { is_closed: false }
}

fn LeakFile() void {
    let file := MakeFile()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"owned file file is still open at scope exit; call file.Close() or return it to transfer ownership at 17:9",
	})
}

func TestClosedOrReturnedFileDoesNotLeakAtScopeExit(t *testing.T) {
	input := `
module io

enum IOError error {
    BadFileDescriptor,
}

type File struct {
    is_closed: bool,
}

fn MakeFile() File {
    return File { is_closed: false }
}

impl File {
    fn Close() Result[void, IOError] {
        self.is_closed = true
        return Ok()
    }
}

fn CloseFile() Result[void, IOError] {
    let mut file := MakeFile()
    try file.Close()
    return Ok()
}

fn ReturnFile() File {
    let file := MakeFile()
    return file
}
`

	errors := analyzeSourceRaw(t, input)
	if len(errors) > 0 {
		t.Fatalf("Analyze returned errors: %v", errors)
	}
}

func TestOpenDirectoryMustBeClosedBeforeScopeExit(t *testing.T) {
	input := `
module io

type Directory struct {
    is_closed: bool,
}

fn MakeDirectory() Directory {
    return Directory { is_closed: false }
}

fn LeakDirectory() void {
    let directory := MakeDirectory()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"owned directory directory is still open at scope exit; call directory.Close() or return it to transfer ownership at 13:9",
	})
}

func TestResourceCloseMustReachEveryContinuingBranch(t *testing.T) {
	input := `
module io

enum IOError error { Failed }

type File struct {
	is_closed: bool,
}

fn MakeFile() File {
	return File { is_closed: false }
}

impl File {
	fn Close() Result[void, IOError] {
		self.is_closed = true
		return Ok()
	}
}

fn Test(condition: bool) Result[void, IOError] {
	let mut file := MakeFile()
	if condition {
		try file.Close()
	}
	return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"owned file file is still open at scope exit; call file.Close() or return it to transfer ownership at 22:10",
	})
}

func TestResourceCloseOnEveryBranchSatisfiesCleanup(t *testing.T) {
	input := `
module io

enum IOError error { Failed }

type File struct {
	is_closed: bool,
}

fn MakeFile() File {
	return File { is_closed: false }
}

impl File {
	fn Close() Result[void, IOError] {
		self.is_closed = true
		return Ok()
	}
}

fn Test(condition: bool) Result[void, IOError] {
	let mut file := MakeFile()
	if condition {
		try file.Close()
	} else {
		try file.Close()
	}
	return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestConditionalLoopCloseDoesNotSatisfyZeroIterationExit(t *testing.T) {
	input := `
module io

enum IOError error { Failed }

type File struct {
	is_closed: bool,
}

fn MakeFile() File {
	return File { is_closed: false }
}

impl File {
	fn Close() Result[void, IOError] {
		self.is_closed = true
		return Ok()
	}
}

fn Test(condition: bool) Result[void, IOError] {
	let mut file := MakeFile()
	while condition {
		try file.Close()
		break
	}
	return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"owned file file is still open at scope exit; call file.Close() or return it to transfer ownership at 22:10",
	})
}

func TestWhileTrueCleanupStateComesFromBreakExits(t *testing.T) {
	input := `
module io

enum IOError error { Failed }

type File struct {
	is_closed: bool,
}

fn MakeFile() File {
	return File { is_closed: false }
}

impl File {
	fn Close() Result[void, IOError] {
		self.is_closed = true
		return Ok()
	}
}

fn Test() Result[void, IOError] {
	let mut file := MakeFile()
	while true {
		try file.Close()
		break
	}
	return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestContinueEdgeCanReopenLoopCarriedResource(t *testing.T) {
	input := `
module io

enum IOError error { Failed }

type File struct {
	is_closed: bool,
}

fn MakeFile() File {
	return File { is_closed: false }
}

impl File {
	fn Close() Result[void, IOError] {
		self.is_closed = true
		return Ok()
	}
}

fn Test(condition: bool) Result[void, IOError] {
	let mut file := MakeFile()
	try file.Close()
	while condition {
		file = MakeFile()
		continue
	}
	return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"owned file file is still open at scope exit; call file.Close() or return it to transfer ownership at 22:10",
	})
}

func TestSwitchResourceCloseStateUsesAllContinuingCases(t *testing.T) {
	input := `
module io

enum IOError error { Failed }

type File struct {
	is_closed: bool,
}

fn MakeFile() File {
	return File { is_closed: false }
}

impl File {
	fn Close() Result[void, IOError] {
		self.is_closed = true
		return Ok()
	}
}

fn Test(condition: bool) Result[void, IOError] {
	let mut file := MakeFile()
	switch condition {
	case true:
		try file.Close()
	default:
	}
	return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"owned file file is still open at scope exit; call file.Close() or return it to transfer ownership at 22:10",
	})
}
