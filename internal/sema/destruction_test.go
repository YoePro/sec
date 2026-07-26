package sema

import "testing"

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
	arrayType := Type{Name: "int[4]", Kind: ArrayType, Element: &intType, ArrayLength: 4}
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

enum IOError {
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
