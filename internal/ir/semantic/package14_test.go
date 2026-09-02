package semantic

import (
	"errors"
	"fmt"
	"math/big"
	"os"
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

@noCopy
type Token struct { value: int }

fn Build() Token[1] {
  return [Token{ value: 1 }]
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
	if !errors.As(err, &unsupported) || unsupported.Package != 14 || unsupported.Feature != "non-trivial array literal element" {
		t.Fatalf("error = %#v, want Package 14 non-trivial array-literal UnsupportedFeatureError", err)
	}
}

func TestPackage14BuildsTrivialMutableFixedArrayStorage(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn Explicit(source: int32[2]) int32[4] {
    let mut values: int32[4] := [1, source..., 4]
    return values
}

fn Defaulted() int128[3] {
    let mut values: int128[3]
    return values
}

fn Nested() int32[2][2] {
    let mut values: int32[2][2] := [[1, 2], [3, 4]]
    return values
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify mutable fixed-array storage: %v\n%s", err, Format(module))
	}
	formatted := Format(module)
	for _, forbidden := range []string{"memref", "llvm.array", "undef", "poison"} {
		if strings.Contains(strings.ToLower(formatted), forbidden) {
			t.Fatalf("mutable fixed-array storage introduced %q:\n%s", forbidden, formatted)
		}
	}

	for _, function := range module.Functions {
		if len(function.Storages) != 1 {
			t.Fatalf("%s storages = %#v, want one high-level array storage", function.Name, function.Storages)
		}
		storageType, ok := module.Types.Lookup(function.Storages[0].Type)
		if !ok || storageType.Kind != TypeArray {
			t.Fatalf("%s storage type = %#v, want TypeArray", function.Name, storageType)
		}
		counts := map[OpKind]int{}
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				counts[operation.Kind]++
			}
		}
		for _, kind := range []OpKind{OpStorageDeclare, OpStorageInit, OpStorageLoad} {
			if counts[kind] != 1 {
				t.Fatalf("%s %s count = %d, want 1\n%s", function.Name, kind, counts[kind], formatted)
			}
		}
		if counts[OpStorageStore] != 0 {
			t.Fatalf("%s unexpectedly stored after initialization", function.Name)
		}
	}

	var explicit *Function
	for _, function := range module.Functions {
		if function.Name == "Explicit" {
			explicit = function
		}
	}
	if explicit == nil {
		t.Fatal("missing Explicit function")
	}
	constructs := []Operation{}
	for _, operation := range explicit.Blocks[0].Operations {
		if operation.Kind == OpArrayConstruct {
			constructs = append(constructs, operation)
		}
	}
	if len(constructs) != 1 || len(constructs[0].Operands) != 3 ||
		constructs[0].ArraySegmentKinds[1] != ArraySegmentSpread ||
		constructs[0].ArraySegmentLengths[1] != "2" ||
		constructs[0].ArrayActions[1] != ArrayActionCopyTrivial {
		t.Fatalf("explicit compact construction = %#v", constructs)
	}
}

func TestPackage14RejectsNonTrivialFixedArrayStorageWithoutPartialModule(t *testing.T) {
	p := parser.New(lexer.NewWithFile(`module main

@noCopy
type Token struct { value: int }

fn Rejected() void {
    let mut values: Token[0]
}
`, "package14-nontrivial-storage.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if semaErrors := analyzer.Analyze(parsed.Program); len(semaErrors) != 0 {
		t.Fatalf("zero-length non-defaultable array should pass Sema: %v", semaErrors)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 14})
	if module != nil {
		t.Fatalf("unsupported storage returned partial module:\n%s", Format(module))
	}
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Package != 14 || unsupported.Feature != "non-trivial mutable local storage" {
		t.Fatalf("error = %#v, want Package 14 non-trivial storage boundary", err)
	}
}

func TestPackage14BuildsTransactionalIndexedLocalReplacement(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn NextIndex() int128 {
    return 1
}

fn NextValue() int32 {
    return 9
}

fn Runtime() int32[4] {
    let mut values: int32[4] := [1, 2, 3, 4]
    values[NextIndex()] = NextValue()
    return values
}

fn Proven() int32[4] {
    let mut values: int32[4] := [1, 2, 3, 4]
    values[2] = NextValue()
    return values
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify indexed local replacement: %v\n%s", err, Format(module))
	}

	functions := map[string]*Function{}
	for _, function := range module.Functions {
		functions[function.Name] = function
	}
	runtime, proven := functions["Runtime"], functions["Proven"]
	if runtime == nil || proven == nil {
		t.Fatalf("missing replacement functions: %#v", functions)
	}
	if len(runtime.Blocks) != 3 {
		t.Fatalf("Runtime blocks = %d, want entry/success/failure", len(runtime.Blocks))
	}
	entry, success, failure := runtime.Blocks[0], runtime.Blocks[1], runtime.Blocks[2]
	if len(entry.Operations) < 4 || entry.Operations[len(entry.Operations)-1].Kind != OpCondBranch ||
		entry.Operations[len(entry.Operations)-2].Kind != OpArrayIndexInBounds ||
		entry.Operations[len(entry.Operations)-3].Kind != OpStorageLoad ||
		entry.Operations[len(entry.Operations)-4].Kind != OpDirectCall {
		t.Fatalf("Runtime did not evaluate index then bounds before RHS:\n%s", Format(module))
	}
	if len(failure.Operations) != 1 || failure.Operations[0].Kind != OpBoundsFailure {
		t.Fatalf("Runtime failure path = %#v", failure.Operations)
	}
	replaceIndex, storeCount, rhsCalls := -1, 0, 0
	for index, operation := range success.Operations {
		switch operation.Kind {
		case OpDirectCall:
			rhsCalls++
		case OpArrayReplace:
			replaceIndex = index
			predicate := entry.Operations[len(entry.Operations)-2]
			if operation.ArrayGuard != predicate.Results[0].ID || operation.Operands[0] != predicate.Operands[0] || operation.Operands[1] != predicate.Operands[1] {
				t.Fatalf("replacement does not reuse its exact guard array/index: %#v", operation)
			}
		case OpStorageStore:
			storeCount++
		}
	}
	if rhsCalls != 1 || replaceIndex < 1 || success.Operations[replaceIndex-1].Kind != OpDirectCall || storeCount != 1 || success.Operations[replaceIndex+1].Kind != OpStorageStore {
		t.Fatalf("Runtime RHS/replace/store order = %#v", success.Operations)
	}

	provenOps := proven.Blocks[0].Operations
	provenReplace, provenStore := -1, 0
	for index, operation := range provenOps {
		if operation.Kind == OpArrayReplace {
			provenReplace = index
			if operation.ArrayCheckKind != ArrayIndexProvenSafe || operation.ArrayProofKind != ArrayIndexProofConstant || operation.ArrayGuard != 0 {
				t.Fatalf("proven replacement metadata = %#v", operation)
			}
		}
		if operation.Kind == OpStorageStore {
			provenStore++
		}
	}
	if provenReplace < 2 || provenOps[provenReplace-2].Kind != OpDirectCall || provenOps[provenReplace-1].Kind != OpStorageLoad || provenStore != 1 || provenOps[provenReplace+1].Kind != OpStorageStore {
		t.Fatalf("Proven RHS/load/replace/store order = %#v", provenOps)
	}
}

func TestPackage14BuildsNestedArrayStructReplacementMatrix(t *testing.T) {
	module, err := analyzedModule(t, `module main

type Pair struct {
    left: int32,
    right: int128,
}

type Holder struct {
    values: int128[2],
    tag: uint256,
}

fn Next32() int32 {
    return 9
}

fn Next128() int128 {
    return 99
}

fn NestedArrays(row: uint, column: uint) int32[2][2] {
    let mut matrix: int32[2][2] := [[1, 2], [3, 4]]
    matrix[row][column] = Next32()
    return matrix
}

fn StructInArray(index: uint) Pair[2] {
    let mut pairs: Pair[2] := [Pair{ left: 1, right: 2 }, Pair{ left: 3, right: 4 }]
    pairs[index].right = Next128()
    return pairs
}

fn ArrayInStruct(index: uint) Holder {
    let mut holder: Holder := Holder{ values: [1, 2], tag: 3 }
    holder.values[index] = Next128()
    return holder
}

fn Wide(index: uint) int128[2] {
    let mut values: int128[2] := [1, 2]
    values[index] = Next128()
    return values
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify nested replacement matrix: %v\n%s", err, Format(module))
	}

	definitions := map[string]StructDefinition{}
	for _, definition := range module.Structs {
		definitions[definition.Name] = definition
	}
	pair, pairOK := definitions["Pair"]
	holder, holderOK := definitions["Holder"]
	if !pairOK || !holderOK || pair.SymbolID != "main::Pair" || holder.SymbolID != "main::Holder" {
		t.Fatalf("nominal struct identities = Pair:%#v Holder:%#v", pair, holder)
	}
	if len(pair.Fields) != 2 || len(holder.Fields) != 2 {
		t.Fatalf("nested struct fields = Pair:%#v Holder:%#v", pair.Fields, holder.Fields)
	}
	pairWide, _ := module.Types.Lookup(pair.Fields[1].Type)
	holderArray, _ := module.Types.Lookup(holder.Fields[0].Type)
	holderWide, _ := module.Types.Lookup(holder.Fields[1].Type)
	holderElement, _ := module.Types.Lookup(holderArray.Element)
	if pairWide.Kind != TypeInt || pairWide.BitWidth != 128 ||
		holderArray.Kind != TypeArray || holderArray.Length != "2" ||
		holderElement.Kind != TypeInt || holderElement.BitWidth != 128 ||
		holderWide.Kind != TypeUint || holderWide.BitWidth != 256 {
		t.Fatalf("wide nested types = pair:%#v holder-array:%#v holder-element:%#v holder-wide:%#v", pairWide, holderArray, holderElement, holderWide)
	}

	type counts struct {
		arrayExtract, arrayReplace   int
		structExtract, structReplace int
		bounds, rhsCalls, stores     int
	}
	want := map[string]counts{
		"NestedArrays":  {arrayExtract: 1, arrayReplace: 2, bounds: 2, rhsCalls: 1, stores: 1},
		"StructInArray": {arrayExtract: 1, arrayReplace: 1, structReplace: 1, bounds: 1, rhsCalls: 1, stores: 1},
		"ArrayInStruct": {arrayReplace: 1, structExtract: 1, structReplace: 1, bounds: 1, rhsCalls: 1, stores: 1},
		"Wide":          {arrayReplace: 1, bounds: 1, rhsCalls: 1, stores: 1},
	}
	for _, function := range module.Functions {
		expected, relevant := want[function.Name]
		if !relevant {
			continue
		}
		got := counts{}
		rhsBeforeReplacement := false
		for _, block := range function.Blocks {
			sawRHS := false
			for _, operation := range block.Operations {
				switch operation.Kind {
				case OpDirectCall:
					got.rhsCalls++
					sawRHS = true
				case OpArrayIndexInBounds:
					got.bounds++
				case OpArrayExtract:
					got.arrayExtract++
				case OpArrayReplace:
					got.arrayReplace++
					rhsBeforeReplacement = rhsBeforeReplacement || sawRHS
				case OpStructExtractField:
					got.structExtract++
				case OpStructReplaceField:
					got.structReplace++
					rhsBeforeReplacement = rhsBeforeReplacement || sawRHS
				case OpStorageStore:
					got.stores++
				}
			}
		}
		if got != expected || !rhsBeforeReplacement {
			t.Fatalf("%s replacement counts/order = %+v, want %+v\n%s", function.Name, got, expected, Format(module))
		}
		if len(function.Storages) != 1 {
			t.Fatalf("%s root storage = %#v", function.Name, function.Storages)
		}
		switch function.Name {
		case "StructInArray":
			result, _ := module.Types.Lookup(function.ReturnType)
			if result.Kind != TypeArray || result.Element != pair.TypeID || result.Length != "2" {
				t.Fatalf("StructInArray return type = %#v, want exact main::Pair[2]", result)
			}
		case "ArrayInStruct":
			if function.ReturnType != holder.TypeID {
				t.Fatalf("ArrayInStruct return type = %d, want exact main::Holder type %d", function.ReturnType, holder.TypeID)
			}
		}
	}
}

// TestPackage14BuildsArrayUnionPayloadAndMatchValues covers Package 14
// sections 79-80: P11/P12 retain their ordinary union-variant patterns and
// guarded projections when the copy-trivial payload and match result are fixed
// arrays. No array-specific pattern or physical representation is introduced.
// SEC-MLIR Package 14 sections 36 and 96 require a branch-refined index to
// reach Semantic IR as proven-safe provenance without a bounds-failure CFG.
func TestPackage14BuildsBranchRefinedArrayIndexWithoutBoundsFailure(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn Branch(values: int[4], index: int) int {
    if index == 2 {
        return values[index]
    }
    return 0
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify branch-refined index: %v\n%s", err, Format(module))
	}

	foundExtract := false
	for _, block := range module.Functions[0].Blocks {
		for _, operation := range block.Operations {
			if operation.Kind == OpBoundsFailure || operation.Kind == OpArrayIndexInBounds {
				t.Fatalf("branch-refined index emitted a bounds check:\n%s", Format(module))
			}
			if operation.Kind == OpArrayExtract {
				foundExtract = true
				if operation.ArrayCheckKind != ArrayIndexProvenSafe || operation.ArrayProofKind != ArrayIndexProofBranch || operation.ArrayGuard != 0 {
					t.Fatalf("branch-refined extraction metadata = %#v", operation)
				}
			}
		}
	}
	if !foundExtract {
		t.Fatalf("missing branch-refined extraction:\n%s", Format(module))
	}
}

func TestPackage14BuildsArrayUnionPayloadAndMatchValues(t *testing.T) {
	module, err := analyzedModule(t, `module main

type ArrayChoice union {
    Values(int128[2])
    Empty
}

fn Wrap(values: int128[2]) ArrayChoice {
    return ArrayChoice.Values(values)
}

fn Read(choice: ArrayChoice) int128[2] {
	let empty: int128[2] := [0, 0]
    return match choice {
        Values(values) => values
        Empty => empty
    }
}

fn ReadGuarded(choice: ArrayChoice, enabled: bool) int128[2] {
	let empty: int128[2] := [0, 0]
    return match choice {
        Values(values) where enabled => values
        Values(values) => values
        Empty => empty
    }
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify array union/match integration: %v\n%s", err, Format(module))
	}
	if len(module.Unions) != 1 || module.Unions[0].SymbolID != "main::ArrayChoice" || len(module.Unions[0].Variants) != 2 {
		t.Fatalf("array union definition = %#v", module.Unions)
	}
	payload, ok := module.Types.Lookup(module.Unions[0].Variants[0].Payload)
	element, elementOK := module.Types.Lookup(payload.Element)
	if !ok || !elementOK || payload.Kind != TypeArray || payload.Length != "2" || element.Kind != TypeInt || element.BitWidth != 128 {
		t.Fatalf("array union payload = %#v, element = %#v", payload, element)
	}

	counts := package12OperationCounts(module)
	if counts[OpUnionConstruct] != 1 || counts[OpUnionIsVariant] != 5 || counts[OpUnionUnwrapPayload] != 3 || counts[OpArrayConstruct] != 2 {
		t.Fatalf("array union/match operations = %#v\n%s", counts, Format(module))
	}
	for _, function := range module.Functions {
		if function.Name == "Wrap" {
			for _, operation := range function.Blocks[0].Operations {
				if operation.Kind == OpUnionConstruct && (len(operation.PayloadActions) != 1 || operation.PayloadActions[0] != UnionPayloadCopyTrivial) {
					t.Fatalf("array union construction action = %#v", operation.PayloadActions)
				}
			}
			continue
		}
		if len(function.Matches) != 1 || !function.Matches[0].ValueContext {
			t.Fatalf("%s match facts = %#v", function.Name, function.Matches)
		}
		for _, arm := range function.Matches[0].Arms {
			if arm.PatternKind != string(sema.MatchPatternUnionVariant) {
				t.Fatalf("%s introduced non-union array pattern %q", function.Name, arm.PatternKind)
			}
		}
		result, _ := module.Types.Lookup(function.ReturnType)
		if function.ReturnType != module.Unions[0].Variants[0].Payload {
			t.Fatalf("%s result = %#v, want exact payload array %#v", function.Name, result, payload)
		}
	}

	// The existing P11 guard contract must remain active for array payloads.
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for index := range block.Operations {
				if block.Operations[index].Kind == OpUnionUnwrapPayload {
					unwrap := block.Operations[index]
					for _, candidateBlock := range function.Blocks {
						for candidateIndex := range candidateBlock.Operations {
							candidate := &candidateBlock.Operations[candidateIndex]
							if candidate.Kind == OpUnionIsVariant && candidate.MatchID == unwrap.MatchID && candidate.MatchArmIndex == unwrap.MatchArmIndex {
								candidate.UnionVariant = 1
							}
						}
					}
					if err := Verify(module); err == nil || !strings.Contains(err.Error(), "not on its matching guarded path") {
						t.Fatalf("mismatched array payload guard error = %v", err)
					}
					return
				}
			}
		}
	}
	t.Fatal("missing array payload projection to test guard enforcement")
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

// TestPackage14ArrayConstructionTypeAndWidthMatrix covers the Package 14
// sections 92 and 94 value-layer matrix. The huge spread is represented by one
// operand and one segment regardless of its conceptual element count.
func TestPackage14ArrayConstructionTypeAndWidthMatrix(t *testing.T) {
	module := package14ArrayConstructionMatrixModule()
	if err := Verify(module); err != nil {
		t.Fatalf("verify construction matrix: %v\n%s", err, Format(module))
	}

	wantConstructs := map[string]int{
		"0": 0, "2": 2, "3": 3, "4": 4,
		"18446744073709551616": 1,
	}
	seenLengths := map[string]int{}
	for _, operation := range module.Functions[0].Blocks[0].Operations {
		switch operation.Kind {
		case OpArrayConstruct:
			resultType, ok := module.Types.Lookup(operation.Results[0].Type)
			if !ok || resultType.Kind != TypeArray || resultType.Element != operation.ArrayElementType || resultType.Length != operation.ArrayLength {
				t.Fatalf("construct result does not preserve its exact array type: %#v", operation)
			}
			seenLengths[operation.ArrayLength]++
			if operation.ArrayLength == "18446744073709551616" && (len(operation.Operands) != 1 || len(operation.ArraySegmentLengths) != 1) {
				t.Fatalf("huge construction expanded in Semantic IR: %#v", operation)
			}
		case OpArrayLength:
			resultType, ok := module.Types.Lookup(operation.Results[0].Type)
			if !ok || resultType.Kind != TypeUint || !resultType.TargetSize {
				t.Fatalf("array.len result is not target-sized uint: %#v", operation)
			}
		}
	}
	for length, minimumSegments := range wantConstructs {
		if seenLengths[length] == 0 {
			t.Fatalf("missing array.construct length %s in matrix", length)
		}
		if length == "0" || length == "18446744073709551616" {
			continue
		}
		found := false
		for _, operation := range module.Functions[0].Blocks[0].Operations {
			if operation.Kind == OpArrayConstruct && operation.ArrayLength == length && len(operation.ArraySegmentLengths) == minimumSegments {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing compact %s-element construction", length)
		}
	}

	text := Format(module)
	for _, want := range []string{
		`length="0"`, `length="18446744073709551616"`,
		`array.len %18 exact="18446744073709551616"`, `array.len %26 exact="0"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted construction matrix missing %q:\n%s", want, text)
		}
	}
}

// TestPackage14BuildsCompactFixedArrayDefaultMatrix covers the source-to-IR
// portion of Package 14 sections 24-27 and 93. Every default remains one
// array.default operation regardless of N or recursive trivial element shape.
func TestPackage14BuildsCompactFixedArrayDefaultMatrix(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/fixed_array_defaults.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "fixed_array_defaults.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if semaErrors := analyzer.Analyze(parsed.Program); len(semaErrors) != 0 {
		t.Fatalf("sema: %v", semaErrors)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"fixed_array_defaults.sec"}, MaxPackage: 14})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}

	wantLengths := map[string]string{
		"ScalarDefault": "4", "WideDefault": "3", "StructDefault": "2",
		"NestedArrayDefault": "2", "StructWithArrayDefault": "2",
		"HugeCompactDefault": "1000000",
	}
	for _, function := range module.Functions {
		defaults := []Operation{}
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				if operation.Kind == OpArrayDefault {
					defaults = append(defaults, operation)
				}
			}
		}
		want, relevant := wantLengths[function.Name]
		if !relevant {
			continue
		}
		if len(defaults) != 1 || defaults[0].ArrayLength != want || len(defaults[0].Operands) != 0 {
			t.Fatalf("%s compact defaults = %#v", function.Name, defaults)
		}
		if function.Name == "StructWithArrayDefault" {
			constructs := 0
			for _, block := range function.Blocks {
				for _, operation := range block.Operations {
					if operation.Kind == OpStructConstruct {
						constructs++
					}
				}
			}
			if constructs != 1 {
				t.Fatalf("StructWithArrayDefault struct.construct count = %d, want 1", constructs)
			}
		}
	}
	text := Format(module)
	if strings.Contains(text, "undef") || strings.Contains(text, "poison") {
		t.Fatalf("fixed-array default introduced unreadable state:\n%s", text)
	}
}

// TestPackage14RejectsNonTrivialFixedArrayDefaultWithoutPartialModule proves
// the Package 14 section-26 ownership gate returns an explicit package-tagged
// unsupported result instead of substituting zero/undef or partial IR.
func TestPackage14RejectsNonTrivialFixedArrayDefaultWithoutPartialModule(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/fixed_array_defaults_deferred.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "fixed_array_defaults_deferred.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if semaErrors := analyzer.Analyze(parsed.Program); len(semaErrors) != 0 {
		t.Fatalf("valid source default rejected by Sema: %v", semaErrors)
	}
	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 14})
	if module != nil {
		t.Fatalf("unsupported default returned partial module:\n%s", Format(module))
	}
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Package != 14 || !strings.Contains(unsupported.Feature, "non-trivial fixed-array default") {
		t.Fatalf("error = %#v, want Package 14 non-trivial fixed-array default", err)
	}
}

// TestPackage14RejectsDeferredArraySourcePathsWithoutPartialIR covers the
// source-pipeline ownership and view boundary required by Package 14 section
// 103. Some invalid operations are rejected by Sema before IR construction;
// valid-but-deferred operations must return a package-tagged unsupported error
// and never expose the module assembled before the boundary was encountered.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 88-89, 103
//   - rules/compiler/semantic_ir.txt — "Unsupported lowerings"
func TestPackage14RejectsDeferredArraySourcePathsWithoutPartialIR(t *testing.T) {
	type rejectionStage string
	const (
		semaStage  rejectionStage = "sema"
		buildStage rejectionStage = "semantic-ir"
	)
	tests := []struct {
		name       string
		file       string
		stage      rejectionStage
		wantDetail string
	}{
		{
			name: "move-only literal spread", file: "move_only_spread_invalid.sec",
			stage: semaStage, wantDetail: "unsupported consuming element reads",
		},
		{
			name: "move-only element read", file: "move_only_read_invalid.sec",
			stage: semaStage, wantDetail: "cannot return Token element values[0] by ordinary indexing because it is not implicitly copyable",
		},
		{
			name: "move-only result element read", file: "move_only_result_read_invalid.sec",
			stage: semaStage, wantDetail: "cannot return Token element values[0] by ordinary indexing because it is not implicitly copyable",
		},
		{
			name: "shared element borrow", file: "shared_element_borrow_invalid.sec",
			stage: buildStage, wantDetail: "shared fixed-array element borrow",
		},
		{
			name: "mutable element borrow", file: "mutable_element_borrow_invalid.sec",
			stage: buildStage, wantDetail: "mutable fixed-array element borrow",
		},
		{
			name: "element move-out", file: "element_move_out_invalid.sec",
			stage: semaStage, wantDetail: "explicit indexed extraction is not implemented",
		},
		{
			name: "non-trivial replacement", file: "nontrivial_replacement_invalid.sec",
			stage: buildStage, wantDetail: "non-trivial array literal element",
		},
		{
			name: "non-trivial parameter destruction", file: "nontrivial_destruction_invalid.sec",
			stage: buildStage, wantDetail: "dynamic array type int[]",
		},
		{
			name: "dynamic owning array", file: "dynamic_array_invalid.sec",
			stage: buildStage, wantDetail: "dynamic array type",
		},
		{
			name: "slice value", file: "slice_value_invalid.sec",
			stage: buildStage, wantDetail: "type ref int[]",
		},
		{
			name: "array-to-slice creation", file: "array_to_slice_invalid.sec",
			stage: buildStage, wantDetail: "array-to-slice creation",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := "../../../testdata/semantic_ir/package14_unsupported/" + test.file
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(source), path))
			parsed := p.Parse()
			if parsed.HasErrors {
				t.Fatalf("parse errors: %v", p.Errors())
			}
			analyzer := sema.NewAnalyzer()
			semaErrors := analyzer.Analyze(parsed.Program)
			if test.stage == semaStage {
				if len(semaErrors) == 0 || !strings.Contains(fmt.Sprint(semaErrors), test.wantDetail) {
					t.Fatalf("sema errors = %v, want detail %q", semaErrors, test.wantDetail)
				}
				return
			}
			if len(semaErrors) != 0 {
				t.Fatalf("unexpected sema errors: %v", semaErrors)
			}
			module, err := Build(parsed.Program, analyzer, BuildOptions{
				RequestedModule: "main", SourceFiles: []string{path}, MaxPackage: 14,
			})
			if module != nil {
				t.Fatalf("unsupported P14 build exposed partial IR:\n%s", Format(module))
			}
			var unsupported *UnsupportedFeatureError
			if !errors.As(err, &unsupported) || unsupported.Package != 14 || !strings.Contains(unsupported.Feature, test.wantDetail) {
				t.Fatalf("build error = %#v, want Package 14 UnsupportedFeatureError containing %q", err, test.wantDetail)
			}
		})
	}
}

// TestPackage14RejectsDeferredArraySpreadActions locks the Semantic IR boundary
// for semantic-copy and later ownership actions even though current source Sema
// has no concrete type whose implicit copy classification is CopySemantic.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 21, 89, 103
func TestPackage14RejectsDeferredArraySpreadActions(t *testing.T) {
	if action, ok := semanticArraySpreadAction(sema.ArrayTransferCopyTrivial); !ok || action != ArrayActionCopyTrivial {
		t.Fatalf("copy-trivial spread action = %q, %t", action, ok)
	}
	for _, deferred := range []sema.ResolvedArrayTransferAction{
		sema.ArrayTransferCopySemantic,
		sema.ArrayTransferMove,
		sema.ArrayTransferBorrowShared,
		sema.ArrayTransferBorrowMutable,
	} {
		if action, ok := semanticArraySpreadAction(deferred); ok || action != "" {
			t.Errorf("deferred spread action %q mapped to %q, %t", deferred, action, ok)
		}
	}
}

// TestPackage14RejectsDeferredArrayOperatorsAtSemanticIRBoundary proves that
// equality and membership remain valid frontend semantics but cannot fall
// through the generic operator path or expose partial Package 14 IR.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 82-83, 103
func TestPackage14RejectsDeferredArrayOperatorsAtSemanticIRBoundary(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantDetail string
	}{
		{
			name: "equality",
			source: `module main
fn Equal(left: int[2], right: int[2]) bool {
    return left == right
}`,
			wantDetail: "fixed-array equality lowering",
		},
		{
			name: "inequality",
			source: `module main
fn Different(left: int[2], right: int[2]) bool {
    return left != right
}`,
			wantDetail: "fixed-array equality lowering",
		},
		{
			name: "membership",
			source: `module main
fn Contains(value: int, values: int[2]) bool {
    return value in values
}`,
			wantDetail: "fixed-array membership lowering",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(test.source, "package14-operator-boundary.sec"))
			parsed := p.Parse()
			if parsed.HasErrors {
				t.Fatalf("parse errors: %v", p.Errors())
			}
			analyzer := sema.NewAnalyzer()
			if semaErrors := analyzer.Analyze(parsed.Program); len(semaErrors) != 0 {
				t.Fatalf("valid frontend operator produced errors: %v", semaErrors)
			}
			module, err := Build(parsed.Program, analyzer, BuildOptions{
				RequestedModule: "main", SourceFiles: []string{"package14-operator-boundary.sec"}, MaxPackage: 14,
			})
			if module != nil {
				t.Fatalf("unsupported operator exposed partial IR:\n%s", Format(module))
			}
			var unsupported *UnsupportedFeatureError
			if !errors.As(err, &unsupported) || unsupported.Package != 14 || unsupported.Feature != test.wantDetail {
				t.Fatalf("build error = %#v, want Package 14 UnsupportedFeatureError %q", err, test.wantDetail)
			}
		})
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
			name: "construct bad exact segment sum",
			mutate: func(module *Module) {
				operation := &module.Functions[0].Blocks[0].Operations[1]
				operation.Operands[1] = operation.Operands[0]
				operation.ArraySegmentKinds[1] = ArraySegmentElement
				operation.ArraySegmentLengths[1] = "1"
				operation.ArrayActions[1] = ArrayActionConstructDirect
			},
			wantError: "array.construct length mismatch",
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
			name: "spread semantic copy deferred",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[0].Operations[1].ArrayActions[1] = ArrayActionCopySemanticInfallible
			},
			wantError: "spread segment 1 mismatch",
		},
		{
			name: "spread move deferred",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[0].Operations[1].ArrayActions[1] = ArrayActionMove
			},
			wantError: "spread segment 1 mismatch",
		},
		{
			name: "spread shared borrow deferred",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[0].Operations[1].ArrayActions[1] = ArrayActionBorrowShared
			},
			wantError: "spread segment 1 mismatch",
		},
		{
			name: "spread mutable borrow deferred",
			mutate: func(module *Module) {
				module.Functions[0].Blocks[0].Operations[1].ArrayActions[1] = ArrayActionBorrowMutable
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

func TestPackage14BuildsOrdinaryArrayIndexControlFlowAndOrder(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn Identity(values: int32[3]) int32[3] {
    return values
}

fn Next() int128 {
    return 1
}

fn Proven(values: int32[3]) int32 {
    return values[1]
}

fn Runtime(values: int32[3]) int32 {
    return Identity(values)[Next()]
}

fn RuntimeUnsigned(values: int32[3], index: uint128) int32 {
    return values[index]
}

fn ZeroLength(values: int32[0], index: uint) int32 {
    return values[index]
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify source-built array indexes: %v\n%s", err, Format(module))
	}

	var proven, runtime, runtimeUnsigned, zeroLength *Function
	for _, function := range module.Functions {
		switch function.Name {
		case "Proven":
			proven = function
		case "Runtime":
			runtime = function
		case "RuntimeUnsigned":
			runtimeUnsigned = function
		case "ZeroLength":
			zeroLength = function
		}
	}
	if proven == nil || runtime == nil || runtimeUnsigned == nil || zeroLength == nil {
		t.Fatalf("missing source-built functions: %#v", module.Functions)
	}
	if len(proven.Blocks) != 1 || len(proven.Blocks[0].Operations) != 3 ||
		proven.Blocks[0].Operations[0].Kind != OpConstInt ||
		proven.Blocks[0].Operations[1].Kind != OpArrayExtract ||
		proven.Blocks[0].Operations[1].ArrayCheckKind != ArrayIndexProvenSafe ||
		proven.Blocks[0].Operations[1].ArrayProofKind != ArrayIndexProofConstant ||
		proven.Blocks[0].Operations[2].Kind != OpReturn {
		t.Fatalf("proven-safe index emitted unexpected control flow:\n%s", Format(module))
	}

	if len(runtime.Blocks) != 3 {
		t.Fatalf("runtime index blocks = %d, want entry/success/failure\n%s", len(runtime.Blocks), Format(module))
	}
	entry := runtime.Blocks[0]
	wantEntry := []OpKind{OpDirectCall, OpDirectCall, OpArrayIndexInBounds, OpCondBranch}
	if len(entry.Operations) != len(wantEntry) {
		t.Fatalf("runtime entry operations = %#v", entry.Operations)
	}
	for index, want := range wantEntry {
		if entry.Operations[index].Kind != want {
			t.Fatalf("runtime entry operation %d = %s, want %s", index, entry.Operations[index].Kind, want)
		}
	}
	if entry.Operations[0].Callee == entry.Operations[1].Callee {
		t.Fatal("array and index calls collapsed into one evaluation")
	}
	predicate := entry.Operations[2].Results[0].ID
	success := runtime.Blocks[1]
	failure := runtime.Blocks[2]
	if len(success.Operations) != 2 || success.Operations[0].Kind != OpArrayExtract || success.Operations[0].ArrayGuard != predicate || success.Operations[1].Kind != OpReturn {
		t.Fatalf("runtime success path = %#v", success.Operations)
	}
	if len(failure.Operations) != 1 || failure.Operations[0].Kind != OpBoundsFailure || failure.Operations[0].ArrayOperation != "fixed-array-index" {
		t.Fatalf("runtime failure path = %#v", failure.Operations)
	}
	if !entry.Operations[2].ArrayIndexSigned {
		t.Fatal("int128 source index signedness was not preserved")
	}
	for _, function := range []*Function{runtimeUnsigned, zeroLength} {
		if len(function.Blocks) != 3 || len(function.Blocks[0].Operations) != 2 ||
			function.Blocks[0].Operations[0].Kind != OpArrayIndexInBounds ||
			function.Blocks[0].Operations[0].ArrayIndexSigned ||
			len(function.Blocks[2].Operations) != 1 || function.Blocks[2].Operations[0].Kind != OpBoundsFailure {
			t.Fatalf("%s did not preserve unsigned runtime bounds flow:\n%s", function.Name, Format(module))
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

func package14ArrayConstructionMatrixModule() *Module {
	types := NewTypeTable()
	voidType := types.Intern(Type{Kind: TypeVoid, Name: "void"})
	int32Type := types.Intern(Type{Kind: TypeInt, Name: "int32", Signed: true, BitWidth: 32})
	int128Type := types.Intern(Type{Kind: TypeInt, Name: "int128", Signed: true, BitWidth: 128})
	uint256Type := types.Intern(Type{Kind: TypeUint, Name: "uint256", BitWidth: 256})
	uintType := types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	pairType := types.Intern(Type{Kind: TypeStruct, Name: "Pair", Module: "main", Identity: "main::Pair"})
	modeType := types.Intern(Type{Kind: TypeEnum, Name: "Mode", Module: "main", Identity: "main::Mode", Underlying: uint256Type})
	int32Array4 := types.Intern(Type{Kind: TypeArray, Name: "int32[4]", Element: int32Type, Length: "4"})
	int128Array2 := types.Intern(Type{Kind: TypeArray, Name: "int128[2]", Element: int128Type, Length: "2"})
	uint256Array2 := types.Intern(Type{Kind: TypeArray, Name: "uint256[2]", Element: uint256Type, Length: "2"})
	int32Array2 := types.Intern(Type{Kind: TypeArray, Name: "int32[2]", Element: int32Type, Length: "2"})
	nestedArray := types.Intern(Type{Kind: TypeArray, Name: "int32[2][3]", Element: int32Array2, Length: "3"})
	pairArray2 := types.Intern(Type{Kind: TypeArray, Name: "Pair[2]", Element: pairType, Length: "2"})
	modeArray4 := types.Intern(Type{Kind: TypeArray, Name: "Mode[4]", Element: modeType, Length: "4"})
	hugeLength := "18446744073709551616"
	hugeArray := types.Intern(Type{Kind: TypeArray, Name: "int32[" + hugeLength + "]", Element: int32Type, Length: hugeLength})
	zeroArray := types.Intern(Type{Kind: TypeArray, Name: "int32[0]", Element: int32Type, Length: "0"})

	parameterTypes := []TypeID{
		int32Type, int32Type, int32Type, int32Type,
		int128Type, int128Type,
		uint256Type, uint256Type,
		int32Array2, int32Array2, int32Array2,
		pairType, pairType,
		modeType, modeType, modeType, modeType,
		hugeArray,
	}
	parameters := make([]Parameter, len(parameterTypes))
	for index, typ := range parameterTypes {
		parameters[index] = Parameter{Name: fmt.Sprintf("value%d", index+1), Value: Value{ID: ValueID(index + 1), Type: typ, Ownership: OwnershipImmediate}}
	}
	nextResult := ValueID(len(parameters) + 1)
	construct := func(element TypeID, result TypeID, length string, operands []ValueID, kinds []ArrayConstructSegmentKind, lengths []string, actions []ArrayTransferAction) Operation {
		op := Operation{
			Kind: OpArrayConstruct, Operands: operands,
			Results:          []Value{{ID: nextResult, Type: result, Ownership: OwnershipImmediate}},
			ArrayElementType: element, ArrayLength: length,
			ArraySegmentKinds: kinds, ArraySegmentLengths: lengths, ArrayActions: actions,
		}
		nextResult++
		return op
	}
	elements := func(first ValueID, count int) ([]ValueID, []ArrayConstructSegmentKind, []string, []ArrayTransferAction) {
		operands := make([]ValueID, count)
		kinds := make([]ArrayConstructSegmentKind, count)
		lengths := make([]string, count)
		actions := make([]ArrayTransferAction, count)
		for index := 0; index < count; index++ {
			operands[index] = first + ValueID(index)
			kinds[index] = ArraySegmentElement
			lengths[index] = "1"
			actions[index] = ArrayActionConstructDirect
		}
		return operands, kinds, lengths, actions
	}
	operations := []Operation{}
	for _, test := range []struct {
		element TypeID
		result  TypeID
		length  string
		first   ValueID
		count   int
	}{
		{int32Type, int32Array4, "4", 1, 4},
		{int128Type, int128Array2, "2", 5, 2},
		{uint256Type, uint256Array2, "2", 7, 2},
		{int32Array2, nestedArray, "3", 9, 3},
		{pairType, pairArray2, "2", 12, 2},
		{modeType, modeArray4, "4", 14, 4},
	} {
		operands, kinds, lengths, actions := elements(test.first, test.count)
		operations = append(operations, construct(test.element, test.result, test.length, operands, kinds, lengths, actions))
	}
	operations = append(operations,
		construct(int32Type, hugeArray, hugeLength, []ValueID{18}, []ArrayConstructSegmentKind{ArraySegmentSpread}, []string{hugeLength}, []ArrayTransferAction{ArrayActionCopyTrivial}),
		construct(int32Type, zeroArray, "0", nil, nil, nil, nil),
		Operation{Kind: OpArrayLength, Operands: []ValueID{18}, Results: []Value{{ID: nextResult, Type: uintType, Ownership: OwnershipImmediate}}, ArrayLength: hugeLength},
	)
	nextResult++
	operations = append(operations,
		Operation{Kind: OpArrayLength, Operands: []ValueID{nextResult - 2}, Results: []Value{{ID: nextResult, Type: uintType, Ownership: OwnershipImmediate}}, ArrayLength: "0"},
		Operation{Kind: OpReturn},
	)

	return &Module{
		Version: Version, Identity: "main", Types: types,
		Structs: []StructDefinition{{
			TypeID: pairType, SymbolID: "main::Pair", Name: "Pair",
			Fields:             []StructFieldDefinition{{ID: 0, Name: "value", Type: int32Type}},
			CopyClassification: "trivial", TriviallyDestructible: true, Defaultable: true,
		}},
		Enums: []EnumDefinition{{
			TypeID: modeType, SymbolID: "main::Mode", Name: "Mode", Underlying: uint256Type,
			RepresentationKind: EnumRepresentationInteger,
			Cases:              []EnumCase{{ID: 0, Name: "Off", Value: big.NewInt(0)}, {ID: 1, Name: "On", Value: big.NewInt(1)}},
		}},
		Functions: []*Function{{
			ID: "main::ConstructionMatrix", Name: "ConstructionMatrix", ReturnType: voidType,
			Entry: 0, Parameters: parameters, Blocks: []*Block{{ID: 0, Operations: operations}},
		}},
	}
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

func TestPackage14BuildsFallibleBoundsPropagationAndLocalHandler(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Read(values: int[4], index: int) Result[int, IndexError] {
  return Ok(try values[index])
}
fn Local(values: int[4], index: int) int {
  return try values[index] {
    Err(IndexError.OutOfBounds) => 0
  }
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	for _, function := range module.Functions {
		kinds := map[OpKind]int{}
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				kinds[operation.Kind]++
			}
		}
		if kinds[OpArrayIndexInBounds] != 1 || kinds[OpArrayExtract] != 1 || kinds[OpEnumConstant] == 0 || kinds[OpBoundsFailure] != 0 {
			t.Fatalf("%s fallible bounds operations = %#v\n%s", function.Name, kinds, Format(module))
		}
		if function.Name == "Read" && kinds[OpResultErr] != 1 {
			t.Fatalf("Read must propagate exactly one typed Result.err: %#v", kinds)
		}
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
