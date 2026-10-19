package semantic

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Shared and mutable slices expose the same uint count without a read, move,
// array copy, signed conversion or additional borrow. Dynamic sources retain
// P15's invalid-reference-generation failure independently of spatial checks.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§47–49,110–111;
// rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md — §1.
func TestSliceLengthOperation(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, mutable := range []bool{false, true} {
			for _, dynamic := range []bool{false, true} {
				module := sliceLengthModule(mutable, dynamic)
				module.PointerWidthBits = width
				if err := Verify(module); err != nil {
					t.Fatal(err)
				}
				text := Format(module)
				if !strings.Contains(text, "slice.length %1") || Format(module) != text {
					t.Fatalf("length printing:\n%s", text)
				}
				if dynamic && (!strings.Contains(text, "valid=%2") || !strings.Contains(text, "InvalidReferenceGeneration")) {
					t.Fatal("missing temporal guard")
				}
				if !dynamic {
					module.Functions[0].NoPanic, module.Functions[0].NoPanicSource = true, "written"
					if err := Verify(module); err != nil {
						t.Fatal(err)
					}
				} else {
					module.Functions[0].NoPanic, module.Functions[0].NoPanicSource = true, "written"
					if err := Verify(module); err == nil || !strings.Contains(err.Error(), "InvalidReferenceGeneration") {
						t.Fatalf("dynamic length falsely became panic-free: %v", err)
					}
				}
			}
		}
	}
}

// Both source spellings and authorities have the same compiler-owned uint
// result. Source-to-operation production remains the separate P16-34 step.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§47–49.
func TestSliceLengthSourceFormsAreUint(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/slice_length_forms.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "slice-length.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	count := 0
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		binding := fn.Body.Statements[0].(*ast.LetStatement)
		fact, found := analyzer.ResolvedBindingDeclaredAt(binding.Name.Token.File, binding.Name.Token.Line, binding.Name.Token.Column)
		typ := fact.Type
		if !found || typ.Kind != sema.UintType || typ.Name != "uint" {
			t.Fatalf("%s length type = %+v", fn.Name.Value, typ)
		}
		count++
	}
	if count != 4 {
		t.Fatal("missing source forms")
	}
}

// A guard elsewhere in the function, for another source/epoch, or with a
// recoverable bounds endpoint cannot establish temporal validity for length.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§44–49,83–84,110–111.
func TestSliceLengthRejectsInvalidTypesAndValidity(t *testing.T) {
	tests := []struct {
		name    string
		dynamic bool
		change  func(*Module)
	}{
		{"direct element reference", false, func(m *Module) {
			fn := m.Functions[0]
			fn.Parameters[0].Value.Type = m.Types.Intern(Type{Kind: TypeReference, Element: 1})
		}},
		{"array value", false, func(m *Module) {
			m.Functions[0].Parameters[0].Value.Type = m.Types.Intern(Type{Kind: TypeArray, Element: 1, Length: "4"})
		}},
		{"signed result", false, func(m *Module) {
			m.Functions[0].Blocks[0].Operations[0].Results[0].Type = m.Types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
		}},
		{"fixed-width result", false, func(m *Module) {
			m.Functions[0].Blocks[0].Operations[0].Results[0].Type = m.Types.Intern(Type{Kind: TypeUint, Name: "uint64", BitWidth: 64})
		}},
		{"missing source facts", false, func(m *Module) { m.Functions[0].Parameters[0].Reference = nil }},
		{"new borrow on observation", false, func(m *Module) {
			m.Functions[0].Blocks[0].Operations[0].Reference = cloneReferenceMetadata(m.Functions[0].Parameters[0].Reference)
		}},
		{"missing guard", true, func(m *Module) { m.Functions[0].Blocks[1].Operations[0].ReferenceGuard = 0 }},
		{"wrong epoch", true, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Reference.EpochDependency++ }},
		{"wrong borrow", true, func(m *Module) { m.Functions[0].Blocks[0].Operations[0].Reference.Borrow++ }},
		{"wrong source", true, func(m *Module) {
			fn := m.Functions[0]
			second := fn.Parameters[0]
			second.Value.ID = 99
			second.Name = "other"
			second.Reference = cloneReferenceMetadata(second.Reference)
			second.Reference.Borrow++
			fn.Parameters = append(fn.Parameters, second)
			fn.Blocks[0].Operations[0].Operands[0] = 99
		}},
		{"length before guard", true, func(m *Module) {
			fn := m.Functions[0]
			length := fn.Blocks[1].Operations[0]
			fn.Blocks[0].Operations = append([]Operation{length}, fn.Blocks[0].Operations...)
			fn.Blocks[1].Operations = fn.Blocks[1].Operations[1:]
		}},
		{"bounds failure replaces stale", true, func(m *Module) {
			m.Functions[0].Blocks[2].Operations[0] = Operation{Kind: OpBoundsFailure, ArrayOperation: "fixed-array-index"}
		}},
		{"guard does not dominate", true, func(m *Module) {
			fn := m.Functions[0]
			boolean := fn.Blocks[0].Operations[0].Results[0].Type
			fn.Parameters = append(fn.Parameters, Parameter{Name: "choose", Value: Value{ID: 100, Type: boolean, Ownership: OwnershipImmediate}})
			fn.Entry = 4
			fn.Blocks = append(fn.Blocks, &Block{ID: 4, Operations: []Operation{{Kind: OpCondBranch, Operands: []ValueID{100}, Successors: []BranchTarget{{Block: 1}, {Block: 2}}}}})
		}},
		{"observation on false edge", true, func(m *Module) {
			fn := m.Functions[0]
			fn.Blocks[2].Operations = append([]Operation{{Kind: OpSliceLength, Operands: []ValueID{1}, Results: []Value{{ID: 98, Type: fn.ReturnType, Ownership: OwnershipImmediate}}, ReferenceGuard: 2}}, fn.Blocks[2].Operations...)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := sliceLengthModule(false, test.dynamic)
			test.change(module)
			if err := Verify(module); err == nil {
				t.Fatal("accepted invalid length observation")
			}
		})
	}
}

// Build the canonical proven or P15-guarded slice parameter observation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§44–49,110–111.
func sliceLengthModule(mutable, dynamic bool) *Module {
	types := NewTypeTable()
	element := types.Intern(Type{Kind: TypeInt, Name: "int32", BitWidth: 32, Signed: true})
	length := types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	boolean := types.Intern(Type{Kind: TypeBool, Name: "bool"})
	slice, _ := types.InternSlice(element, mutable)
	metadata := &ReferenceMetadata{Borrow: 1, Lifetime: 1, Validity: ReferenceProven, ProofProvenance: "lexical", AddressSpace: AddressSpaceOrdinary, AddressStability: "stable", Provenance: "parameter", OriginStorages: []uint32{1}}
	ownership := OwnershipSharedReference
	if mutable {
		ownership = OwnershipMutableReference
	}
	fn := &Function{ID: "main::Count", Name: "Count", ReturnType: length, Entry: 1, Parameters: []Parameter{{Name: "view", Value: Value{ID: 1, Type: slice, Ownership: ownership}, Reference: metadata}}}
	observation := Operation{Kind: OpSliceLength, Operands: []ValueID{1}, Results: []Value{{ID: 3, Type: length, Ownership: OwnershipImmediate}}}
	fn.Blocks = []*Block{{ID: 1, Operations: []Operation{observation, {Kind: OpReturn, Operands: []ValueID{3}}}}}
	if dynamic {
		metadata.Validity, metadata.ProofProvenance = ReferenceDynamicEpoch, ""
		metadata.InvalidationDomain, metadata.EpochDependency = 1, 1
		observation.ReferenceGuard = 2
		fn.Blocks = []*Block{
			{ID: 1, Operations: []Operation{{Kind: OpReferenceIsValid, Operands: []ValueID{1}, Results: []Value{{ID: 2, Type: boolean, Ownership: OwnershipImmediate}}, Reference: cloneReferenceMetadata(metadata)}, {Kind: OpCondBranch, Operands: []ValueID{2}, Successors: []BranchTarget{{Block: 2}, {Block: 3}}}}},
			{ID: 2, Operations: []Operation{observation, {Kind: OpReturn, Operands: []ValueID{3}}}},
			{ID: 3, Operations: []Operation{{Kind: OpReferenceGenerationFailure, PanicReasonID: invalidReferenceGenerationID, PanicReason: invalidReferenceGenerationName}}},
		}
	}
	return &Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}}
}
