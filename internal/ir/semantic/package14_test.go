package semantic

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func TestPackage14FixedArrayTypesInternFormatAndVerify(t *testing.T) {
	types := NewTypeTable()
	intType := types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
	huge := "18446744073709551616"
	array := types.Intern(Type{Kind: TypeArray, Name: "int[" + huge + "]", Element: intType, Length: huge})
	if again := types.Intern(Type{Kind: TypeArray, Name: "int[" + huge + "]", Element: intType, Length: huge}); again != array {
		t.Fatalf("fixed array type did not intern by exact identity: %d != %d", again, array)
	}
	nested := types.Intern(Type{Kind: TypeArray, Name: "int[" + huge + "][3]", Element: array, Length: "3"})
	module := &Module{Version: Version, Identity: "main", Types: types}
	if err := Verify(module); err != nil {
		t.Fatalf("verify fixed arrays: %v\n%s", err, Format(module))
	}
	text := Format(module)
	if !strings.Contains(text, `array<!1, "`+huge+`">`) || !strings.Contains(text, `array<!2, "3">`) {
		t.Fatalf("formatted array types missing exact lengths:\n%s", text)
	}
	if nested == array || nested == intType {
		t.Fatalf("nested array identity collapsed: int=%d array=%d nested=%d", intType, array, nested)
	}
}

func TestPackage14ArrayTypeVerifierRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name      string
		array     Type
		wantError string
	}{
		{
			name:      "missing element",
			array:     Type{Kind: TypeArray, Name: "bad", Element: 99, Length: "1"},
			wantError: "invalid element type",
		},
		{
			name:      "empty length",
			array:     Type{Kind: TypeArray, Name: "bad", Element: 1, Length: ""},
			wantError: "non-canonical length",
		},
		{
			name:      "negative length",
			array:     Type{Kind: TypeArray, Name: "bad", Element: 1, Length: "-1"},
			wantError: "non-canonical length",
		},
		{
			name:      "leading zero",
			array:     Type{Kind: TypeArray, Name: "bad", Element: 1, Length: "01"},
			wantError: "non-canonical length",
		},
		{
			name:      "void element",
			array:     Type{Kind: TypeArray, Name: "bad", Element: 1, Length: "1"},
			wantError: "unsupported element type",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			types := NewTypeTable()
			element := types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
			if test.name == "void element" {
				element = types.Intern(Type{Kind: TypeVoid, Name: "void"})
			}
			if test.array.Element == 1 {
				test.array.Element = element
			}
			types.Intern(test.array)
			err := Verify(&Module{Version: Version, Identity: "main", Types: types})
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("verify error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestPackage14BuildsFixedArrayFunctionTypes(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Identity(values: int[2]) int[2] {
  return values
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	if !strings.Contains(text, `array<!`) || !strings.Contains(text, `"2"`) {
		t.Fatalf("fixed array type missing from semantic IR:\n%s", text)
	}
	if len(module.Functions) != 1 || len(module.Functions[0].Parameters) != 1 {
		t.Fatalf("function shape = %#v", module.Functions)
	}
	if module.Functions[0].Parameters[0].Value.Type != module.Functions[0].ReturnType {
		t.Fatalf("parameter and return should share the same fixed-array type ID: %#v", module.Functions[0])
	}
}

func TestPackage14RejectsArrayTypesBeforePackageGate(t *testing.T) {
	_, err := analyzedModule(t, `module main
fn Identity(values: int[2]) int[2] {
  return values
}
`, 13)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Package != 13 || !strings.Contains(unsupported.Feature, "array type") {
		t.Fatalf("error = %#v, want Package 13 array-type UnsupportedFeatureError", err)
	}
}

func TestPackage14UnsupportedArrayLiteralReturnsNoPartialModule(t *testing.T) {
	p := parser.New(lexer.NewWithFile(`module main
fn Build() int[2] {
  return [1, 2]
}
`, "package14-unsupported.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 14})
	if module != nil {
		t.Fatalf("unsupported P14 source lowering returned partial module:\n%s", Format(module))
	}
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Package != 14 || unsupported.Feature != "array literal" {
		t.Fatalf("error = %#v, want Package 14 array-literal UnsupportedFeatureError", err)
	}
}

func TestPackage14ArrayConstructDefaultAndLengthVerifyAndFormat(t *testing.T) {
	module := package14ArrayOpsModule()
	if err := Verify(module); err != nil {
		t.Fatalf("verify array ops: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{
		`array.construct element=!1 length="3" #0=%2[element,1,construct-direct] #1=%1[spread,2,copy-trivial]`,
		`array.len %3 exact="3"`,
		`array.construct element=!1 length="0"`,
		`array.default element=!1 length="0"`,
		`array.default element=!1 length="3"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted Semantic IR missing %q:\n%s", want, text)
		}
	}
}

func TestPackage14ArrayOperationVerifierMatrix(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Module)
		wantError string
	}{
		{
			name: "construct bad total",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[0].Operations[1].ArrayLength = "4"
			},
			wantError: "array.construct result type mismatch",
		},
		{
			name: "element bad action",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[0].Operations[1].ArrayActions[0] = ArrayActionCopyTrivial
			},
			wantError: "element segment 0 mismatch",
		},
		{
			name: "spread bad type",
			mutate: func(module *Module) {
				module.Functions[0].Parameters[0].Value.Type = module.Functions[0].ReturnType
			},
			wantError: "spread segment 1 mismatch",
		},
		{
			name: "default non-defaultable",
			mutate: func(module *Module) {
				resource := module.Types.Intern(Type{Kind: TypeStruct, Name: "Resource", Module: "main", Identity: "main::Resource"})
				module.Structs = append(module.Structs, StructDefinition{TypeID: resource, SymbolID: "main::Resource", Name: "Resource", CopyClassification: "move-only"})
				array := module.Types.Intern(Type{Kind: TypeArray, Name: "Resource[1]", Element: resource, Length: "1"})
				module.Functions[0].Blocks[0].Operations[4].Results[0].Type = array
				module.Functions[0].Blocks[0].Operations[4].ArrayElementType = resource
				module.Functions[0].Blocks[0].Operations[4].ArrayLength = "1"
			},
			wantError: "unsupported default",
		},
		{
			name: "len result not uint",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[0].Operations[2].Results[0].Type = 1
			},
			wantError: "array.len result must be uint",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := package14ArrayOpsModule()
			test.mutate(module)
			err := Verify(module)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("verify error = %v, want %q\n%s", err, test.wantError, Format(module))
			}
		})
	}
}

func TestPackage14ArrayIndexExtractReplaceVerifyAndFormat(t *testing.T) {
	module := package14ArrayIndexModule()
	if err := Verify(module); err != nil {
		t.Fatalf("verify array index ops: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{
		`array.index-in-bounds %1, %2 signed=true`,
		`array.extract %1, %2 bounds=runtime-check proof=guarded action=copy-trivial guard=%4`,
		`array.extract %1, %2 bounds=proven-safe proof=range action=copy-trivial`,
		`array.replace %1, %2 value=%3 bounds=runtime-check proof=guarded guard=%4`,
		`fail.bounds operation="fixed-array-index"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted Semantic IR missing %q:\n%s", want, text)
		}
	}
}

func TestPackage14ArrayIndexVerifierMatrix(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Module)
		wantError string
	}{
		{
			name: "predicate signedness mismatch",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[0].Operations[0].ArrayIndexSigned = false
			},
			wantError: "signedness mismatch",
		},
		{
			name: "runtime extract lacks guard provenance",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[1].Operations[0].ArrayGuard = 99
			},
			wantError: "does not match its array bounds guard",
		},
		{
			name: "runtime extract uses different index",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[1].Operations[0].Operands[1] = 3
			},
			wantError: "does not match its array bounds guard",
		},
		{
			name: "proven extract lacks proof",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[1].Operations[1].ArrayProofKind = ""
			},
			wantError: "invalid proven-safe bounds provenance",
		},
		{
			name: "invalid bounds failure endpoint",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[2].Operations[0].ArrayOperation = "array-index"
			},
			wantError: "invalid fail.bounds",
		},
		{
			name: "failure edge reaches projection",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[2].Operations[0] = Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: 1}}}
			},
			wantError: "invalid failure endpoint",
		},
		{
			name: "replace new value type mismatch",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[1].Operations[2].Operands[2] = 4
			},
			wantError: "array.replace type mismatch",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := package14ArrayIndexModule()
			test.mutate(module)
			err := Verify(module)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("verify error = %v, want %q\n%s", err, test.wantError, Format(module))
			}
		})
	}
}

func package14ArrayOpsModule() *Module {
	types := NewTypeTable()
	intType := types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
	uintType := types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	array0 := types.Intern(Type{Kind: TypeArray, Name: "int[0]", Element: intType, Length: "0"})
	array2 := types.Intern(Type{Kind: TypeArray, Name: "int[2]", Element: intType, Length: "2"})
	array3 := types.Intern(Type{Kind: TypeArray, Name: "int[3]", Element: intType, Length: "3"})
	fn := &Function{
		ID:         "main::Build(int[2])",
		Name:       "Build",
		ReturnType: array3,
		Entry:      0,
		Parameters: []Parameter{{Name: "source", Value: Value{ID: 1, Type: array2, Ownership: OwnershipImmediate}}},
	}
	fn.Blocks = []*Block{{
		ID: 0,
		Operations: []Operation{
			{Kind: OpConstInt, Integer: mustBigInt("7"), Results: []Value{{ID: 2, Type: intType, Ownership: OwnershipImmediate}}},
			{
				Kind:                OpArrayConstruct,
				Operands:            []ValueID{2, 1},
				Results:             []Value{{ID: 3, Type: array3, Ownership: OwnershipImmediate}},
				ArrayElementType:    intType,
				ArrayLength:         "3",
				ArraySegmentKinds:   []ArrayConstructSegmentKind{ArraySegmentElement, ArraySegmentSpread},
				ArraySegmentLengths: []string{"1", "2"},
				ArrayActions:        []ArrayTransferAction{ArrayActionConstructDirect, ArrayActionCopyTrivial},
			},
			{Kind: OpArrayLength, Operands: []ValueID{3}, Results: []Value{{ID: 4, Type: uintType, Ownership: OwnershipImmediate}}, ArrayLength: "3"},
			{Kind: OpArrayConstruct, Results: []Value{{ID: 5, Type: array0, Ownership: OwnershipImmediate}}, ArrayElementType: intType, ArrayLength: "0"},
			{Kind: OpArrayDefault, Results: []Value{{ID: 6, Type: array0, Ownership: OwnershipImmediate}}, ArrayElementType: intType, ArrayLength: "0"},
			{Kind: OpArrayDefault, Results: []Value{{ID: 7, Type: array3, Ownership: OwnershipImmediate}}, ArrayElementType: intType, ArrayLength: "3"},
			{Kind: OpReturn, Operands: []ValueID{3}},
		},
	}}
	return &Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}}
}

func package14ArrayIndexModule() *Module {
	types := NewTypeTable()
	intType := types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
	boolType := types.Intern(Type{Kind: TypeBool, Name: "bool"})
	array3 := types.Intern(Type{Kind: TypeArray, Name: "int[3]", Element: intType, Length: "3"})
	fn := &Function{
		ID:         "main::Update(int[3],int,int)",
		Name:       "Update",
		ReturnType: array3,
		Entry:      0,
		Parameters: []Parameter{
			{Name: "values", Value: Value{ID: 1, Type: array3, Ownership: OwnershipImmediate}},
			{Name: "index", Value: Value{ID: 2, Type: intType, Ownership: OwnershipImmediate}},
			{Name: "newValue", Value: Value{ID: 3, Type: intType, Ownership: OwnershipImmediate}},
		},
	}
	fn.Blocks = []*Block{
		{
			ID: 0,
			Operations: []Operation{
				{Kind: OpArrayIndexInBounds, Operands: []ValueID{1, 2}, Results: []Value{{ID: 4, Type: boolType, Ownership: OwnershipImmediate}}, ArrayIndexSigned: true},
				{Kind: OpCondBranch, Operands: []ValueID{4}, Successors: []BranchTarget{{Block: 1}, {Block: 2}}},
			},
		},
		{
			ID: 1,
			Operations: []Operation{
				{Kind: OpArrayExtract, Operands: []ValueID{1, 2}, Results: []Value{{ID: 5, Type: intType, Ownership: OwnershipImmediate}}, ArrayCheckKind: ArrayIndexRuntimeCheck, ArrayProofKind: ArrayIndexProofGuarded, ArrayGuard: 4, ArrayActions: []ArrayTransferAction{ArrayActionCopyTrivial}},
				{Kind: OpArrayExtract, Operands: []ValueID{1, 2}, Results: []Value{{ID: 6, Type: intType, Ownership: OwnershipImmediate}}, ArrayCheckKind: ArrayIndexProvenSafe, ArrayProofKind: ArrayIndexProofRange, ArrayActions: []ArrayTransferAction{ArrayActionCopyTrivial}},
				{Kind: OpArrayReplace, Operands: []ValueID{1, 2, 3}, Results: []Value{{ID: 7, Type: array3, Ownership: OwnershipImmediate}}, ArrayCheckKind: ArrayIndexRuntimeCheck, ArrayProofKind: ArrayIndexProofGuarded, ArrayGuard: 4},
				{Kind: OpReturn, Operands: []ValueID{7}},
			},
		},
		{
			ID:         2,
			Operations: []Operation{{Kind: OpBoundsFailure, ArrayOperation: "fixed-array-index"}},
		},
	}
	return &Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}}
}

func TestPackage14FallibleBoundsBuildsIndexError(t *testing.T) {
	module := package14FallibleArrayIndexModule()
	if err := Verify(module); err != nil {
		t.Fatalf("verify fallible bounds path: %v\n%s", err, Format(module))
	}
	text := Format(module)
	if !strings.Contains(text, `enum.constant case=#0`) || !strings.Contains(text, `result.err %6`) {
		t.Fatalf("fallible path does not construct IndexError.OutOfBounds:\n%s", text)
	}
}

func package14FallibleArrayIndexModule() *Module {
	types := NewTypeTable()
	intType := types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
	uintType := types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	boolType := types.Intern(Type{Kind: TypeBool, Name: "bool"})
	indexError := types.Intern(Type{Kind: TypeEnum, Name: "IndexError", Module: "core", Identity: "core::IndexError", Underlying: uintType})
	array3 := types.Intern(Type{Kind: TypeArray, Name: "int[3]", Element: intType, Length: "3"})
	resultType := types.Intern(Type{Kind: TypeResult, Name: "Result[int,IndexError]", Success: intType, Error: indexError})
	fn := &Function{
		ID:         "main::Read(int[3],int)",
		Name:       "Read",
		ReturnType: resultType,
		Entry:      0,
		Parameters: []Parameter{
			{Name: "values", Value: Value{ID: 1, Type: array3, Ownership: OwnershipImmediate}},
			{Name: "index", Value: Value{ID: 2, Type: intType, Ownership: OwnershipImmediate}},
		},
	}
	fn.Blocks = []*Block{
		{ID: 0, Operations: []Operation{
			{Kind: OpArrayIndexInBounds, Operands: []ValueID{1, 2}, Results: []Value{{ID: 3, Type: boolType, Ownership: OwnershipImmediate}}, ArrayIndexSigned: true},
			{Kind: OpCondBranch, Operands: []ValueID{3}, Successors: []BranchTarget{{Block: 1}, {Block: 2}}},
		}},
		{ID: 1, Operations: []Operation{
			{Kind: OpArrayExtract, Operands: []ValueID{1, 2}, Results: []Value{{ID: 4, Type: intType, Ownership: OwnershipImmediate}}, ArrayCheckKind: ArrayIndexRuntimeCheck, ArrayProofKind: ArrayIndexProofGuarded, ArrayGuard: 3, ArrayActions: []ArrayTransferAction{ArrayActionCopyTrivial}},
			{Kind: OpResultOk, Operands: []ValueID{4}, Results: []Value{{ID: 5, Type: resultType, Ownership: OwnershipImmediate}}},
			{Kind: OpReturn, Operands: []ValueID{5}},
		}},
		{ID: 2, Operations: []Operation{
			{Kind: OpEnumConstant, EnumCase: 0, Results: []Value{{ID: 6, Type: indexError, Ownership: OwnershipImmediate}}},
			{Kind: OpResultErr, Operands: []ValueID{6}, Results: []Value{{ID: 7, Type: resultType, Ownership: OwnershipImmediate}}},
			{Kind: OpReturn, Operands: []ValueID{7}},
		}},
	}
	return &Module{
		Version: Version, Identity: "main", Types: types, Functions: []*Function{fn},
		Enums: []EnumDefinition{{
			TypeID: indexError, SymbolID: "core::IndexError", Name: "IndexError", Underlying: uintType,
			RepresentationKind: EnumRepresentationInteger,
			Cases:              []EnumCase{{ID: 0, Name: "OutOfBounds", Value: big.NewInt(0)}},
		}},
	}
}

func mustBigInt(value string) *big.Int {
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("invalid test integer " + value)
	}
	return parsed
}
