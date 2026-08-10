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

func analyzedModule(t *testing.T, source string, maxPackage uint8) (*Module, error) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "sample.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	a := sema.NewAnalyzer()
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatalf("sema errors: %v", errs)
	}
	return Build(parsed.Program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{"sample.sec"}, MaxPackage: maxPackage})
}

func TestTypeTableInternsStructuralIdentity(t *testing.T) {
	table := NewTypeTable()
	base := table.Intern(Type{Kind: TypeInt, Name: "int", TargetSize: true})
	if got := table.Intern(Type{Kind: TypeInt, Name: "int", TargetSize: true}); got != base {
		t.Fatalf("same type got %d and %d", base, got)
	}
	a := table.Intern(Type{Kind: TypeNamed, Name: "A", Identity: "main::A", Base: base})
	b := table.Intern(Type{Kind: TypeNamed, Name: "B", Identity: "main::B", Base: base})
	if a == b || a == base || b == base {
		t.Fatal("named type identities were erased")
	}
}

func TestBuiltinTypeCoversWideScalars(t *testing.T) {
	for _, test := range []struct {
		typ    sema.Type
		kind   TypeKind
		width  uint16
		signed bool
	}{
		{sema.Type{Name: "int128", Kind: sema.IntType}, TypeInt, 128, true},
		{sema.Type{Name: "int256", Kind: sema.IntType}, TypeInt, 256, true},
		{sema.Type{Name: "uint128", Kind: sema.UintType}, TypeUint, 128, false},
		{sema.Type{Name: "uint256", Kind: sema.UintType}, TypeUint, 256, false},
		{sema.Type{Name: "decimal128", Kind: sema.DecimalType}, TypeDecimal128, 128, true},
	} {
		kind, signed, width, targetSized, ok := builtinType(test.typ)
		if !ok || kind != test.kind || signed != test.signed || width != test.width || targetSized {
			t.Errorf("builtinType(%s) = (%s, %t, %d, %t, %t)", test.typ.Name, kind, signed, width, targetSized, ok)
		}
	}
}

func TestPackage3BuildsWideScalarConstantsStorageAndCalls(t *testing.T) {
	module, err := analyzedModule(t, `
module main
fn Max() int128 { return 170141183460469231731687303715884105727 }
fn Hold(value: int128) int128 {
    let mut current: int128 := value
    return current
}
fn Identity(value: uint256) uint256 { return value }
fn Call(value: uint256) uint256 { return Identity(value) }
fn Exact() decimal128 { return 1.25 }
`, 3)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, typ := range module.Types.All() {
		if typ.Name == "int128" && typ.Kind == TypeInt && typ.BitWidth == 128 {
			found["int128"] = true
		}
		if typ.Name == "uint256" && typ.Kind == TypeUint && typ.BitWidth == 256 {
			found["uint256"] = true
		}
		if typ.Name == "decimal128" && typ.Kind == TypeDecimal128 {
			found["decimal128"] = true
		}
	}
	for _, name := range []string{"int128", "uint256", "decimal128"} {
		if !found[name] {
			t.Errorf("missing Semantic IR type %s", name)
		}
	}
	var hasWideConstant, hasStorage, hasWideCall, hasDecimal128 bool
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				switch operation.Kind {
				case OpConstInt:
					hasWideConstant = operation.Integer != nil && operation.Integer.BitLen() == 127
				case OpStorageDeclare:
					hasStorage = true
				case OpDirectCall:
					hasWideCall = true
				case OpConstDecimal:
					if len(operation.Results) == 1 {
						typ, _ := module.Types.Lookup(operation.Results[0].Type)
						hasDecimal128 = typ.Kind == TypeDecimal128
					}
				}
			}
		}
	}
	if !hasWideConstant || !hasStorage || !hasWideCall || !hasDecimal128 {
		t.Fatalf("coverage constant=%t storage=%t call=%t decimal128=%t", hasWideConstant, hasStorage, hasWideCall, hasDecimal128)
	}
}

func TestExactConstantsAndDeterministicFormat(t *testing.T) {
	decimal, err := parseDecimal("0.10")
	if err != nil {
		t.Fatal(err)
	}
	if decimal.Coefficient.Cmp(big.NewInt(10)) != 0 || decimal.Scale != 2 || decimal.Lexeme != "0.10" {
		t.Fatalf("decimal = %#v", decimal)
	}
	module, err := analyzedModule(t, "module main\nfn Huge() int { return 42 }\n", 2)
	if err != nil {
		t.Fatal(err)
	}
	if first, second := Format(module), Format(module); first != second {
		t.Fatal("printer is not deterministic")
	}
}

func TestPackage2RejectsPackage3Construct(t *testing.T) {
	tests := []struct{ name, source, feature string }{
		{"mutable", "module main\nfn F() int {\n  let mut x := 1\n  return x\n}\n", "mutable local storage"},
		{"call", "module main\nfn One() int { return 1 }\nfn F() int { return One() }\n", "function call"},
		{"if", "module main\nfn F(flag: bool) int {\n  if flag { return 1 }\n  return 2\n}\n", "if control flow"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := analyzedModule(t, test.source, 2)
			var unsupported *UnsupportedFeatureError
			if !errors.As(err, &unsupported) || unsupported.Feature != test.feature {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPackage3BuildsStorageCallsAndCFG(t *testing.T) {
	source := `module main
fn One() int { return 1 }
fn Choose(flag: bool) int {
  let mut value := One()
  if flag { value = 2 } else { value = 3 }
  return value
}`
	module, err := analyzedModule(t, source, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"storage.declare", "storage.init", "storage.store", "storage.load", "call.direct", "conditional-branch"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

func TestPackage7BuildsCheckedIntegerGuards(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Calculate(a: int128, b: int128, c: int128) int128 { return a + b * c }
fn Shift(a: uint256, count: int) uint256 { return a >> count }
fn Compare(a: int, b: int) bool { return a >= b }
`, 7)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	counts := map[OpKind]int{}
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				counts[operation.Kind]++
			}
		}
	}
	if counts[OpIntBinaryChecked] != 2 || counts[OpIntShiftChecked] != 1 || counts[OpIntCompare] != 1 || counts[OpArithmeticFailure] != 3 {
		t.Fatalf("unexpected package-7 operations: %#v", counts)
	}
	calculate := module.Functions[0]
	if len(calculate.Blocks) != 5 || calculate.Blocks[0].Operations[0].IntegerBinary != IntegerCheckedMultiply || calculate.Blocks[2].Operations[0].IntegerBinary != IntegerCheckedAdd {
		t.Fatalf("nested evaluation/guard order is not canonical: %#v", calculate.Blocks)
	}
}

func TestPackage9BuildsTypedArithmeticTryFlow(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Add(left: int128, right: int128) Result[int128, ArithmeticError] {
  let total := try left + right
  return Ok(total)
}
fn Divide(left: int, right: int) int { return left / right }
`, 9)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}

	add := module.Functions[0]
	counts := map[OpKind]int{}
	for _, block := range add.Blocks {
		for _, operation := range block.Operations {
			counts[operation.Kind]++
		}
	}
	if counts[OpIntBinaryChecked] != 1 || counts[OpArithmeticErrorFromReason] != 1 ||
		counts[OpResultErr] != 1 || counts[OpResultOk] != 1 || counts[OpArithmeticFailure] != 0 {
		t.Fatalf("fallible operations = %#v\n%s", counts, Format(module))
	}
	checked := add.Blocks[0].Operations[0]
	if len(checked.Results) != 3 {
		t.Fatalf("checked results = %#v", checked.Results)
	}
	failure := add.Blocks[1]
	if len(failure.Parameters) != 1 || failure.Operations[len(failure.Operations)-1].Kind != OpReturn {
		t.Fatalf("fallible failure block = %#v", failure)
	}

	divide := module.Functions[1]
	ordinaryFailures := 0
	for _, block := range divide.Blocks {
		for _, operation := range block.Operations {
			if operation.Kind == OpArithmeticFailure && len(operation.Operands) == 1 {
				ordinaryFailures++
			}
		}
	}
	if ordinaryFailures != 1 {
		t.Fatalf("ordinary failure flow missing: %s", Format(module))
	}

	var hasResult, hasArithmeticError bool
	for _, typ := range module.Types.All() {
		hasResult = hasResult || typ.Kind == TypeResult
		hasArithmeticError = hasArithmeticError || (typ.Kind == TypeCoreError && typ.Identity == "core::ArithmeticError")
	}
	if !hasResult || !hasArithmeticError {
		t.Fatalf("types missing: %s", Format(module))
	}
}

func TestPackage10BuildsCanonicalResultPropagation(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Source(value: int) Result[int, ArithmeticError] { return Ok(value) }
fn Forward(value: int) Result[int, ArithmeticError] {
  let resolved := try Source(value)
  return Ok(resolved)
}
`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}

	forward := module.Functions[1]
	counts := map[OpKind]int{}
	for _, block := range forward.Blocks {
		for _, operation := range block.Operations {
			counts[operation.Kind]++
		}
	}
	if counts[OpResultIsErr] != 1 || counts[OpResultUnwrapErr] != 1 || counts[OpResultUnwrapOk] != 1 || counts[OpResultErr] != 1 {
		t.Fatalf("Result propagation is not canonical: %#v\n%s", counts, Format(module))
	}

	guardBlock := forward.Blocks[0]
	branch := &guardBlock.Operations[len(guardBlock.Operations)-1]
	branch.Successors[0], branch.Successors[1] = branch.Successors[1], branch.Successors[0]
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "Err successor must unwrap Err") {
		t.Fatalf("malformed Result guard accepted: %v", err)
	}
}

func TestPackage10BuildsOrderedLocalResultHandlers(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Source(value: int) Result[int, ArithmeticError] { return Ok(value) }
fn Handle(value: int) int {
  return try Source(value) {
    Ok(found) => found
    Err(ArithmeticError.DivisionByZero) => 0
    Err(error) => 1
  }
}
fn Forward(value: int) Result[int, ArithmeticError] {
  let resolved := try Source(value) {
    Err(error) => return Err(error)
  }
  return Ok(resolved)
}
`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}

	handle := module.Functions[1]
	counts := map[OpKind]int{}
	var variant Operation
	for _, block := range handle.Blocks {
		for _, operation := range block.Operations {
			counts[operation.Kind]++
			if operation.Kind == OpCoreErrorIsVariant {
				variant = operation
			}
		}
	}
	if counts[OpResultIsErr] != 1 || counts[OpCoreErrorIsVariant] != 1 || variant.Variant != "DivisionByZero" || variant.TryHandlerIndex != 1 {
		t.Fatalf("ordered handlers were not preserved: %#v\n%s", counts, Format(module))
	}
	mergeFound := false
	for _, block := range handle.Blocks {
		mergeFound = mergeFound || len(block.Parameters) == 1
	}
	if !mergeFound {
		t.Fatalf("value handlers have no SSA merge: %s", Format(module))
	}
	forward := module.Functions[2]
	if countOperations(forward, OpResultErr) != 1 || countOperations(forward, OpReturn) != 2 {
		t.Fatalf("return handler was not lowered normally: %s", Format(module))
	}
}

func TestPackage10BuildsLocalArithmeticHandlersWithoutTemporaryResult(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Divide(left: int, right: int) int {
  return try left / right {
    Ok(value) => value
    Err(ArithmeticError.DivisionByZero) => 0
    Err(error) => 1
  }
}
`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	function := module.Functions[0]
	if countOperations(function, OpIntBinaryChecked) != 1 || countOperations(function, OpArithmeticErrorFromReason) != 1 ||
		countOperations(function, OpCoreErrorIsVariant) != 1 || countOperations(function, OpResultOk) != 0 || countOperations(function, OpResultErr) != 0 {
		t.Fatalf("arithmetic handlers did not use direct reason dispatch: %s", Format(module))
	}
}

func countOperations(function *Function, kind OpKind) int {
	count := 0
	for _, block := range function.Blocks {
		for _, operation := range block.Operations {
			if operation.Kind == kind {
				count++
			}
		}
	}
	return count
}

func TestPackage7ReportsUnsupportedOperatorBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		feature string
	}{
		{
			name: "try arithmetic",
			source: `module main
enum Failure { failed, }
fn Use() int {
    return try Calculate() {
        Ok(value) => value
        Err(error) => 0
    } + 1
}
fn Calculate() Result[int, Failure] { return Ok(1) }
`,
			feature: "try arithmetic",
		},
		{
			name:    "float arithmetic",
			source:  "module main\nfn Add(a: float32, b: float32) float32 { return a + b }\n",
			feature: "unresolved or unsupported operator",
		},
		{
			name:    "decimal arithmetic",
			source:  "module main\nfn Add(a: decimal, b: decimal) decimal { return a + b }\n",
			feature: "unresolved or unsupported operator",
		},
		{
			name:    "named integer arithmetic",
			source:  "module main\ntype Count int\nfn Add(a: Count, b: Count) Count { return a + b }\n",
			feature: "unresolved or unsupported operator",
		},
		{
			name:    "unit-bearing arithmetic",
			source:  "module main\nunit ticks uint physical\nfn Add(a: ticks, b: ticks) ticks { return a + b }\n",
			feature: "named type contracts or units",
		},
		{
			name: "compound assignment",
			source: `module main
fn Add(value: int) int {
    let mut result := value
    result += 1
    return result
}
`,
			feature: "compound assignment",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := analyzedModule(t, test.source, 7)
			var unsupported *UnsupportedFeatureError
			if !errors.As(err, &unsupported) {
				t.Fatalf("error = %v, want UnsupportedFeatureError", err)
			}
			if unsupported.Feature != test.feature {
				t.Fatalf("feature = %q, want %q", unsupported.Feature, test.feature)
			}
		})
	}
}

func TestPackage7BuildsEveryIntegerOperationAndFailureCategory(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Add(a: int32, b: int32) int32 { return a + b }
fn Subtract(a: uint32, b: uint32) uint32 { return a - b }
fn Multiply(a: int128, b: int128) int128 { return a * b }
fn Divide(a: int64, b: int64) int64 { return a / b }
fn Remainder(a: uint64, b: uint64) uint64 { return a % b }
fn Plus(a: int8) int8 { return +a }
fn Negate(a: int256) int256 { return -a }
fn BitNot(a: uint16) uint16 { return ~a }
fn BitAnd(a: uint16, b: uint16) uint16 { return a & b }
fn BitOr(a: uint16, b: uint16) uint16 { return a | b }
fn BitXor(a: uint16, b: uint16) uint16 { return a ^ b }
fn LeftSigned(a: int256, count: uint8) int256 { return a << count }
fn LeftUnsigned(a: uint128, count: int16) uint128 { return a << count }
fn RightSigned(a: int64, count: uint8) int64 { return a >> count }
fn RightUnsigned(a: uint64, count: int8) uint64 { return a >> count }
fn Eq(a: int32, b: int32) bool { return a == b }
fn Ne(a: int32, b: int32) bool { return a != b }
fn Lt(a: uint32, b: uint32) bool { return a < b }
fn Le(a: uint32, b: uint32) bool { return a <= b }
fn Gt(a: int32, b: int32) bool { return a > b }
fn Ge(a: int32, b: int32) bool { return a >= b }
fn First() int64 { return 1 }
fn Second() int64 { return 2 }
fn Third() int64 { return 3 }
fn Nested() int64 { return First() + Second() * Third() }
`, 7)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}

	type expectation struct {
		kind    OpKind
		binary  IntegerCheckedBinaryKind
		bitwise IntegerBitwiseKind
		shift   IntegerShiftKind
		compare IntegerComparePredicate
		failure ArithmeticFailureCategory
	}
	wants := map[string]expectation{
		"Add":           {kind: OpIntBinaryChecked, binary: IntegerCheckedAdd, failure: ArithmeticFailureOverflow},
		"Subtract":      {kind: OpIntBinaryChecked, binary: IntegerCheckedSubtract, failure: ArithmeticFailureOverflow},
		"Multiply":      {kind: OpIntBinaryChecked, binary: IntegerCheckedMultiply, failure: ArithmeticFailureOverflow},
		"Divide":        {kind: OpIntBinaryChecked, binary: IntegerCheckedDivide, failure: ArithmeticFailureDivision},
		"Remainder":     {kind: OpIntBinaryChecked, binary: IntegerCheckedRemainder, failure: ArithmeticFailureRemainder},
		"Plus":          {kind: OpIntUnaryPlus},
		"Negate":        {kind: OpIntNegChecked, failure: ArithmeticFailureOverflow},
		"BitNot":        {kind: OpIntBitNot},
		"BitAnd":        {kind: OpIntBitwise, bitwise: IntegerBitwiseAnd},
		"BitOr":         {kind: OpIntBitwise, bitwise: IntegerBitwiseOr},
		"BitXor":        {kind: OpIntBitwise, bitwise: IntegerBitwiseXor},
		"LeftSigned":    {kind: OpIntShiftChecked, shift: IntegerShiftLeftSigned, failure: ArithmeticFailureShift},
		"LeftUnsigned":  {kind: OpIntShiftChecked, shift: IntegerShiftLeftUnsigned, failure: ArithmeticFailureShift},
		"RightSigned":   {kind: OpIntShiftChecked, shift: IntegerShiftRightSigned, failure: ArithmeticFailureShift},
		"RightUnsigned": {kind: OpIntShiftChecked, shift: IntegerShiftRightUnsigned, failure: ArithmeticFailureShift},
		"Eq":            {kind: OpIntCompare, compare: IntegerCompareEQ},
		"Ne":            {kind: OpIntCompare, compare: IntegerCompareNE},
		"Lt":            {kind: OpIntCompare, compare: IntegerCompareLT},
		"Le":            {kind: OpIntCompare, compare: IntegerCompareLE},
		"Gt":            {kind: OpIntCompare, compare: IntegerCompareGT},
		"Ge":            {kind: OpIntCompare, compare: IntegerCompareGE},
	}
	for _, function := range module.Functions {
		want, ok := wants[function.Name]
		if !ok {
			continue
		}
		var operation *Operation
		var failure ArithmeticFailureCategory
		for _, block := range function.Blocks {
			for index := range block.Operations {
				candidate := &block.Operations[index]
				if candidate.Kind == want.kind {
					operation = candidate
				}
				if candidate.Kind == OpArithmeticFailure {
					failure = candidate.FailureCategory
				}
			}
		}
		if operation == nil || operation.IntegerBinary != want.binary || operation.IntegerBitwise != want.bitwise || operation.IntegerShift != want.shift || operation.IntegerCompare != want.compare || failure != want.failure {
			t.Errorf("%s operation=%#v failure=%q, want %#v", function.Name, operation, failure, want)
		}
		delete(wants, function.Name)
	}
	if len(wants) != 0 {
		t.Fatalf("missing operation functions: %#v", wants)
	}

	var nested []string
	for _, function := range module.Functions {
		if function.Name != "Nested" {
			continue
		}
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				switch operation.Kind {
				case OpDirectCall:
					nested = append(nested, string(operation.Callee))
				case OpIntBinaryChecked:
					nested = append(nested, string(operation.IntegerBinary))
				}
			}
		}
	}
	if got := strings.Join(nested, ","); !strings.Contains(got, "First") || !strings.Contains(got, "Second") || !strings.Contains(got, "Third") || !strings.HasSuffix(got, "multiply,add") {
		t.Fatalf("nested evaluation order = %q", got)
	}
}

func TestVerifierRejectsUncheckedIntegerFailureFlag(t *testing.T) {
	types := NewTypeTable()
	intID := types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
	boolID := types.Intern(Type{Kind: TypeBool, Name: "bool", BitWidth: 1})
	reasonID := types.Intern(Type{Kind: TypeArithmeticFailureReason, Name: "ArithmeticFailureReason"})
	left := Value{ID: 0, Type: intID, Ownership: OwnershipImmediate}
	right := Value{ID: 1, Type: intID, Ownership: OwnershipImmediate}
	result := Value{ID: 2, Type: intID, Ownership: OwnershipImmediate}
	failed := Value{ID: 3, Type: boolID, Ownership: OwnershipImmediate}
	reason := Value{ID: 4, Type: reasonID, Ownership: OwnershipImmediate}
	fn := &Function{ID: "main::Bad(int,int)", Name: "Bad", ReturnType: intID, Entry: 0,
		Parameters: []Parameter{{Name: "left", Value: left}, {Name: "right", Value: right}},
		Blocks: []*Block{{ID: 0, Operations: []Operation{
			{Kind: OpIntBinaryChecked, Operands: []ValueID{0, 1}, Results: []Value{result, failed, reason}, IntegerBinary: IntegerCheckedAdd, Operator: "+"},
			{Kind: OpReturn, Operands: []ValueID{2}},
		}}},
	}
	err := Verify(&Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}})
	if err == nil || !strings.Contains(err.Error(), "must have exactly one use") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifierRejectsMissingTerminatorAndInvalidType(t *testing.T) {
	types := NewTypeTable()
	intID := types.Intern(Type{Kind: TypeInt, Name: "int"})
	module := &Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{{ID: "main::F()", Name: "F", ReturnType: intID, Blocks: []*Block{{ID: 0}}, Entry: 0}}}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "terminator") {
		t.Fatalf("error = %v", err)
	}
	module.Functions[0].ReturnType = 0
	if err := Verify(module); err == nil {
		t.Fatal("invalid TypeID accepted")
	}
}

func TestVerifierStorageOrderAndSSADominance(t *testing.T) {
	t.Run("init before declare", func(t *testing.T) {
		types := NewTypeTable()
		intID := types.Intern(Type{Kind: TypeInt, Name: "int"})
		value := Value{ID: 0, Type: intID, Ownership: OwnershipImmediate}
		fn := &Function{ID: "main::F()", Name: "F", ReturnType: intID, Entry: 0,
			Storages: []Storage{{ID: 1, Type: intID, Mutable: true, Class: StorageLocalAutomatic}},
			Blocks: []*Block{{ID: 0, Operations: []Operation{
				{Kind: OpConstInt, Results: []Value{value}, Integer: big.NewInt(1)},
				{Kind: OpStorageInit, Storage: 1, Operands: []ValueID{0}},
				{Kind: OpStorageDeclare, Storage: 1},
				{Kind: OpReturn, Operands: []ValueID{0}},
			}}},
		}
		module := &Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}}
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "before declaration") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("non dominating value", func(t *testing.T) {
		types := NewTypeTable()
		intID := types.Intern(Type{Kind: TypeInt, Name: "int"})
		boolID := types.Intern(Type{Kind: TypeBool, Name: "bool"})
		condition := Value{ID: 0, Type: boolID, Ownership: OwnershipImmediate}
		branchValue := Value{ID: 1, Type: intID, Ownership: OwnershipImmediate}
		fn := &Function{ID: "main::F(bool)", Name: "F", ReturnType: intID, Entry: 0, Parameters: []Parameter{{Name: "flag", Value: condition}}, Blocks: []*Block{
			{ID: 0, Operations: []Operation{{Kind: OpCondBranch, Operands: []ValueID{0}, Successors: []BranchTarget{{Block: 1}, {Block: 2}}}}},
			{ID: 1, Operations: []Operation{{Kind: OpConstInt, Results: []Value{branchValue}, Integer: big.NewInt(1)}, {Kind: OpBranch, Successors: []BranchTarget{{Block: 3}}}}},
			{ID: 2, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 3}}}}},
			{ID: 3, Operations: []Operation{{Kind: OpReturn, Operands: []ValueID{1}}}},
		}}
		module := &Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}}
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "does not dominate") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestVerifierAcceptsBlockParameterMerge(t *testing.T) {
	types := NewTypeTable()
	intID := types.Intern(Type{Kind: TypeInt, Name: "int"})
	value := Value{ID: 0, Type: intID, Ownership: OwnershipImmediate}
	merged := Value{ID: 1, Type: intID, Ownership: OwnershipImmediate}
	fn := &Function{ID: "main::F()", Name: "F", ReturnType: intID, Entry: 0, Blocks: []*Block{
		{ID: 0, Operations: []Operation{{Kind: OpConstInt, Results: []Value{value}, Integer: big.NewInt(7)}, {Kind: OpBranch, Successors: []BranchTarget{{Block: 1, Arguments: []ValueID{0}}}}}},
		{ID: 1, Parameters: []Value{merged}, Operations: []Operation{{Kind: OpReturn, Operands: []ValueID{1}}}},
	}}
	if err := Verify(&Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}}); err != nil {
		t.Fatal(err)
	}
}
