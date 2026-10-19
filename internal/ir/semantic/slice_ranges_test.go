package semantic

import (
	"math/big"
	"strings"
	"testing"
)

// Range operations preserve wide original endpoint SSA, distinguish omitted
// bounds and proofs, and produce canonical normalized/failure tuples.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§35,78–80.
func TestSliceRangeOperations(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, checked := range []bool{false, true} {
			module := sliceRangeModule(checked)
			module.PointerWidthBits = width
			if err := Verify(module); err != nil {
				t.Fatal(err)
			}
			text := Format(module)
			want := "slice.range-normalize %1 start-present=false end-present=false"
			if checked {
				want = "slice.range-check %1 %2 %3 start-present=true end-present=true"
			}
			if !strings.Contains(text, want) || Format(module) != text {
				t.Fatalf("range metadata printing:\n%s", text)
			}
			op := module.Functions[0].Blocks[0].Operations[0]
			if checked && (op.SliceRange.StartType != module.Functions[0].Parameters[1].Value.Type || op.SliceRange.EndType != module.Functions[0].Parameters[2].Value.Type) {
				t.Fatal("original wide endpoint types were erased")
			}
		}
	}
	// Inclusive/exclusive and each missing-bound combination remain explicit;
	// checked validation receives the original int128/uint256 values without
	// an increment, subtraction or target-width conversion before the check.
	for _, start := range []bool{false, true} {
		for _, end := range []bool{false, true} {
			for _, exclusive := range []bool{false, true} {
				module := sliceRangeModule(true)
				op := &module.Functions[0].Blocks[0].Operations[0]
				op.Operands = []ValueID{1}
				op.SliceRange = &SliceRangeMetadata{StartPresent: start, EndPresent: end, EndExclusive: exclusive}
				if start {
					op.Operands = append(op.Operands, 2)
					op.SliceRange.StartType = 2
					op.SliceRange.StartSigned = true
				}
				if end {
					op.Operands = append(op.Operands, 3)
					op.SliceRange.EndType = 3
				}
				if err := Verify(module); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	// Source byte and named integer endpoints retain their own SSA identity.
	for _, named := range []bool{false, true} {
		module := sliceRangeModule(true)
		endpoint := module.Types.Intern(Type{Kind: TypeByte, Name: "byte", BitWidth: 8})
		if named {
			endpoint = module.Types.Intern(Type{Kind: TypeNamed, Name: "Octet", Identity: "main::Octet", Base: endpoint})
		}
		module.Functions[0].Parameters[2].Value.Type = endpoint
		module.Functions[0].Blocks[0].Operations[0].SliceRange.EndType = endpoint
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
	}
}

// A checked operation accepts negative and full-width inclusive inputs as
// original integers. No potentially overflowing end+1 or narrowing operation
// is required before this total range check.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§15–19,79.
func TestSliceRangeCheckRetainsExactExtremeEndpoints(t *testing.T) {
	module := sliceRangeModule(true)
	block := module.Functions[0].Blocks[0]
	op, branch := block.Operations[0], block.Operations[1]
	minimum := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127))
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	op.Operands = []ValueID{1, 98, 99}
	op.SliceRange.EndExclusive = false
	block.Operations = []Operation{
		{Kind: OpConstInt, Integer: minimum, Results: []Value{{ID: 98, Type: 2, Ownership: OwnershipImmediate}}},
		{Kind: OpConstInt, Integer: maximum, Results: []Value{{ID: 99, Type: 3, Ownership: OwnershipImmediate}}},
		op, branch,
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if !strings.Contains(text, minimum.String()) || !strings.Contains(text, maximum.String()) || !strings.Contains(text, "exclusive=false") {
		t.Fatalf("wide endpoints were changed:\n%s", text)
	}
}

// Branch arguments may transport each result only on its matching outcome;
// block parameters preserve the distinction without forcing a physical tuple.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§79–80.
func TestSliceRangeCheckForwardsGuardedBlockArguments(t *testing.T) {
	for _, wrongEdge := range []bool{false, true} {
		module := sliceRangeModule(true)
		fn := module.Functions[0]
		fn.Blocks[1].Parameters = []Value{{ID: 77, Type: 6, Ownership: OwnershipImmediate}}
		fn.Blocks[2].Parameters = []Value{{ID: 78, Type: 1, Ownership: OwnershipImmediate}}
		fn.Blocks[0].Operations[1].Successors[0].Arguments = []ValueID{7}
		fn.Blocks[0].Operations[1].Successors[1].Arguments = []ValueID{5}
		fn.Blocks[1].Operations[0].Operands = []ValueID{77}
		fn.Blocks[2].Operations[0].Operands = []ValueID{78}
		if wrongEdge {
			fn.Blocks[1].Parameters[0].Type = 1
			fn.Blocks[0].Operations[1].Successors[0].Arguments = []ValueID{4}
			fn.Blocks[1].Operations[0].Kind = OpResultOk
		}
		err := Verify(module)
		if wrongEdge && err == nil || !wrongEdge && err != nil {
			t.Fatalf("guarded transport (wrong=%t): %v", wrongEdge, err)
		}
	}
}

// Every malformed type, tuple, proof or failure-edge use must fail before
// further lowering. Error values belong only to the true failure edge;
// normalized values belong only to the false success edge.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§78–80,83–84.
func TestSliceRangeVerifierRejectsMalformedOperations(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Module)
	}{
		{"missing metadata", func(m *Module) { m.Functions[0].Blocks[0].Operations[0].SliceRange = nil }},
		{"missing operand", func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Operands = []ValueID{1, 2} }},
		{"swapped endpoint types", func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Operands = []ValueID{1, 3, 2} }},
		{"wrong signedness", func(m *Module) { m.Functions[0].Blocks[0].Operations[0].SliceRange.StartSigned = false }},
		{"omitted endpoint type", func(m *Module) {
			op := &m.Functions[0].Blocks[0].Operations[0]
			op.SliceRange.StartPresent = false
			op.Operands = []ValueID{1, 3}
		}},
		{"noninteger endpoint", func(m *Module) {
			m.Functions[0].Parameters[1].Value.Type = 4
			m.Functions[0].Blocks[0].Operations[0].SliceRange.StartType = 4
		}},
		{"non-native length", func(m *Module) { m.Types.types[0].TargetSize = false }},
		{"wrong normalized type", func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Results[0].Type = 2 }},
		{"wrong tuple arity", func(m *Module) {
			m.Functions[0].Blocks[0].Operations[0].Results = m.Functions[0].Blocks[0].Operations[0].Results[:3]
		}},
		{"non-bool failure", func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Results[2].Type = 1 }},
		{"assumed runtime proof", func(m *Module) { m.Functions[0].Blocks[0].Operations[0].SliceRange.ProofKind = ArrayIndexProofAnalysis }},
		{"wrong error identity", func(m *Module) { m.Types.types[5].Identity = "core::IndexError" }},
		{"wrong error domain", func(m *Module) { m.Enums[0].Cases[0].Name = "Unrelated" }},
		{"guard bypass", func(m *Module) {
			m.Functions[0].Blocks[1].Operations[0] = Operation{Kind: OpResultOk, Operands: []ValueID{4}, Results: []Value{{ID: 8, Type: 7, Ownership: OwnershipImmediate}}}
		}},
		{"error on success", func(m *Module) {
			m.Functions[0].Blocks[2].Operations[0] = Operation{Kind: OpResultErr, Operands: []ValueID{7}, Results: []Value{{ID: 9, Type: 7, Ownership: OwnershipImmediate}}}
		}},
		{"late guard", func(m *Module) {
			b := m.Functions[0].Blocks[0]
			b.Operations = append(b.Operations[:1], Operation{Kind: OpConstInt, Integer: big.NewInt(0), Results: []Value{{ID: 99, Type: 1, Ownership: OwnershipImmediate}}}, b.Operations[1])
		}},
		{"wrong guard", func(m *Module) {
			m.Functions[0].Parameters = append(m.Functions[0].Parameters, Parameter{Name: "other", Value: Value{ID: 99, Type: 4, Ownership: OwnershipImmediate}})
			m.Functions[0].Blocks[0].Operations[1].Operands[0] = 99
		}},
		{"merged edges", func(m *Module) { m.Functions[0].Blocks[0].Operations[1].Successors[0].Block = 3 }},
		{"failed edge reaches normalized use", func(m *Module) {
			m.Functions[0].Blocks[1].Operations = []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 3}}}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := sliceRangeModule(true)
			test.change(module)
			if err := Verify(module); err == nil {
				t.Fatal("accepted malformed slice range")
			}
		})
	}
	for _, proof := range []ArrayIndexProofKind{"", "invented", SliceRangeProofOpenBound} {
		module := sliceRangeModule(false)
		op := &module.Functions[0].Blocks[0].Operations[0]
		op.SliceRange.ProofKind = proof
		if proof == SliceRangeProofOpenBound {
			op.SliceRange.StartPresent = true
			op.SliceRange.StartType = 2
			op.SliceRange.StartSigned = true
			op.Operands = []ValueID{1, 2}
		}
		if err := Verify(module); err == nil {
			t.Fatalf("accepted invalid normalization proof %q", proof)
		}
	}
}

// Build canonical high-level normalization or fallible range CFG; no source
// syntax or physical lowering is enabled by these model/verifier fixtures.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§78–80.
func sliceRangeModule(checked bool) *Module {
	types := NewTypeTable()
	length := types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	start := types.Intern(Type{Kind: TypeInt, Name: "int128", BitWidth: 128, Signed: true})
	end := types.Intern(Type{Kind: TypeUint, Name: "uint256", BitWidth: 256})
	boolean := types.Intern(Type{Kind: TypeBool, Name: "bool"})
	carrier := types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
	rangeError := types.Intern(Type{Kind: TypeEnum, Name: "RangeError", Module: "core", Identity: "core::RangeError", Underlying: carrier})
	result := types.Intern(Type{Kind: TypeResult, Success: length, Error: rangeError})
	module := &Module{Version: Version, Identity: "main", Types: types, Enums: []EnumDefinition{{TypeID: rangeError, SymbolID: "core::RangeError", Name: "RangeError", Underlying: carrier, RepresentationKind: EnumRepresentationInteger,
		Cases: []EnumCase{{ID: 0, Name: "InvalidRange", Value: big.NewInt(0)}, {ID: 1, Name: "StartAfterEnd", Value: big.NewInt(1)}, {ID: 2, Name: "OutOfBounds", Value: big.NewInt(2)}}}}}
	fn := &Function{ID: "main::Range", Name: "Range", ReturnType: length, Entry: 1,
		Parameters: []Parameter{{Name: "length", Value: Value{ID: 1, Type: length, Ownership: OwnershipImmediate}}, {Name: "start", Value: Value{ID: 2, Type: start, Ownership: OwnershipImmediate}}, {Name: "end", Value: Value{ID: 3, Type: end, Ownership: OwnershipImmediate}}}}
	op := Operation{Kind: OpSliceRangeNormalize, Operands: []ValueID{1}, SliceRange: &SliceRangeMetadata{ProofKind: SliceRangeProofOpenBound}, Results: []Value{{ID: 4, Type: length, Ownership: OwnershipImmediate}, {ID: 5, Type: length, Ownership: OwnershipImmediate}}}
	fn.Blocks = []*Block{{ID: 1, Operations: []Operation{op, {Kind: OpReturn, Operands: []ValueID{5}}}}}
	if checked {
		op.Kind = OpSliceRangeCheck
		op.Operands = []ValueID{1, 2, 3}
		op.SliceRange = &SliceRangeMetadata{StartPresent: true, EndPresent: true, EndExclusive: true, StartType: start, StartSigned: true, EndType: end}
		op.Results = append(op.Results, Value{ID: 6, Type: boolean, Ownership: OwnershipImmediate}, Value{ID: 7, Type: rangeError, Ownership: OwnershipImmediate})
		fn.ReturnType = result
		fn.Blocks = []*Block{
			{ID: 1, Operations: []Operation{op, {Kind: OpCondBranch, Operands: []ValueID{6}, Successors: []BranchTarget{{Block: 2}, {Block: 3}}}}},
			{ID: 2, Operations: []Operation{{Kind: OpResultErr, Operands: []ValueID{7}, Results: []Value{{ID: 8, Type: result, Ownership: OwnershipImmediate}}}, {Kind: OpReturn, Operands: []ValueID{8}}}},
			{ID: 3, Operations: []Operation{{Kind: OpResultOk, Operands: []ValueID{5}, Results: []Value{{ID: 9, Type: result, Ownership: OwnershipImmediate}}}, {Kind: OpReturn, Operands: []ValueID{9}}}},
		}
	}
	module.Functions = []*Function{fn}
	return module
}
