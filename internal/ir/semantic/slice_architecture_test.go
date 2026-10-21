package semantic

import (
	"fmt"
	"maps"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/sema"
)

// Source slices must stay semantic values rather than acquiring a descriptor,
// runtime helper or an ABI expansion. Compare signatures, calls and structures
// against the source, independently of the builder's own operation verifier.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§7,75–76,143;
// rules/compiler/semantic_ir.md — §126.1–4,21,26.
func TestPackage16SourceArchitecture(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, file := range []string{
			"slice_source_fixed.sec", "slice_source_reborrow.sec",
			"slice_source_length.sec", "slice_source_fallible.sec",
			"slice_source_index.sec", "slice_source_wide.sec",
		} {
			t.Run(fmt.Sprintf("%d/%s", width, file), func(t *testing.T) {
				program := sliceBoundaryProgram(t, "semantic_ir/"+file)
				a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				if errs := a.Analyze(program); len(errs) != 0 {
					t.Fatal(errs)
				}
				m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
				if err != nil {
					t.Fatal(err)
				}
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				assertAbstractSliceModule(t, m)
				assertSliceSourceSurface(t, program, m)
			})
		}
	}
}

// Epoch identities remain P15 metadata even when guards, nested reborrows,
// element access and holder copy/move/end compose. These are white-box modules,
// not evidence of source Arena allocation or slice-signature production.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§43–46,75,141,143;
// rules/compiler/semantic_ir.md — §126.8–10,18,26.
func TestPackage16EpochArchitecture(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, mutable := range []bool{false, true} {
			for _, kind := range []string{"guarded-access", "length", "transfer"} {
				t.Run(fmt.Sprintf("%d/mutable=%v/%s", width, mutable, kind), func(t *testing.T) {
					var m *Module
					switch kind {
					case "guarded-access":
						m = sliceGuardMatrixModule(mutable)
					case "length":
						m = sliceLengthModule(mutable, true)
					case "transfer":
						m = sliceTransferModule(mutable, true)
					}
					m.PointerWidthBits = width
					if err := Verify(m); err != nil {
						t.Fatal(err)
					}
					assertAbstractSliceModule(t, m)
					fn := m.Functions[0]
					parameters := 1
					if kind == "guarded-access" {
						parameters = 2 // The source slice and original int128 index.
					}
					if len(m.Functions) != 1 || len(fn.Parameters) != parameters || len(fn.Storages) != 0 ||
						len(m.Structs) != 0 || fn.Parameters[0].Reference.Validity != ReferenceDynamicEpoch {
						t.Fatal("epoch metadata acquired a runtime object, storage or hidden argument")
					}
					for _, block := range fn.Blocks {
						for _, op := range block.Operations {
							if op.Kind == OpDirectCall {
								t.Fatal("epoch semantics introduced a runtime helper", op.Callee)
							}
						}
					}
				})
			}
		}
	}
}

// Audit semantic types and operations with a closed inventory. New opcodes must
// be reviewed explicitly: arbitrary pointer arithmetic, allocation, atomics,
// destruction, undef and poison cannot evade a substring-based blacklist.
// Slice identity itself must contain only its element and authority.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§7,75–76,143;
// rules/compiler/semantic_ir.md — §126.1–4,26.
func assertAbstractSliceModule(t *testing.T, m *Module) {
	t.Helper()
	if len(m.Statics) != 0 || len(m.Unions) != 0 {
		t.Fatal("slice fixtures acquired hidden global state or a descriptor union")
	}
	slices := 0
	for _, typ := range m.Types.All() {
		switch typ.Kind {
		case TypeSharedSlice, TypeMutableSlice:
			slices++
			if typeKey(typ) != typeKey(Type{Kind: typ.Kind, Element: typ.Element}) {
				t.Fatal("slice type acquired representation or runtime metadata", typ)
			}
		case TypeVoid, TypeBool, TypeInt, TypeUint, TypeDecimal128,
			TypeArray, TypePlace, TypeReference, TypeResult, TypeEnum,
			TypeStruct, TypeArithmeticFailureReason:
		default:
			t.Fatal("unexpected physical/runtime type in slice fixture", typ)
		}
	}
	if slices == 0 {
		t.Fatal("architecture fixture no longer exercises slices")
	}
	for _, fn := range m.Functions {
		if fn.Extern || fn.ABI != "" || fn.LinkName != "" {
			t.Fatal("slice fixture acquired an external ABI", fn.Name)
		}
		for _, storage := range fn.Storages {
			typ, _ := m.Types.Lookup(storage.Type)
			if typ.Kind != TypeArray {
				t.Fatal("slice fixture acquired counter, generation or descriptor storage", storage)
			}
		}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				switch op.Kind {
				case OpDiscardValue:
					if !op.DiscardTrivial {
						t.Fatal("slice fixture introduced nontrivial destruction")
					}
				case OpConstInt, OpConstBool, OpConstDecimal,
					OpReturn, OpBranch, OpCondBranch, OpDirectCall,
					OpIntBinaryChecked, OpArithmeticFailure,
					OpArithmeticErrorFromReason, OpResultOk, OpResultErr,
					OpEnumConstant, OpEnumCompare,
					OpArrayConstruct, OpArrayLength, OpBoundsFailure,
					OpStorageDeclare, OpStorageInit,
					OpPlaceValueRoot, OpPlaceStorageRoot, OpPlaceField,
					OpPlaceArrayIndexInBounds, OpPlaceArrayElement,
					OpPlaceRead, OpPlaceWrite, OpReferenceDerefPlace,
					OpReferenceBorrowShared, OpReferenceBorrowMutable,
					OpReferenceEndBorrow, OpReferenceIsValid, OpReferenceGenerationFailure,
					OpSliceRangeNormalize, OpSliceRangeCheck,
					OpSliceBorrowShared, OpSliceBorrowMutable,
					OpSliceReborrowShared, OpSliceReborrowMutable,
					OpSliceCopyShared, OpSliceMoveMutable,
					OpSliceLength, OpSliceIndexInBounds, OpSliceElementPlace:
				default:
					t.Fatal("unexpected physical/runtime operation in slice fixture", fn.Name, op.Kind)
				}
			}
		}
	}
}

// Source-defined functions and fields are the complete callable/aggregate
// surface: no helper functions, descriptor fields, ABI parameters or additional
// calls may be synthesized for slice creation, observation or cleanup.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§7,75,143;
// rules/compiler/semantic_ir.md — §126.21,26.
func assertSliceSourceSurface(t *testing.T, program *ast.Program, m *Module) {
	t.Helper()
	functions := map[string]*ast.FunctionDeclaration{}
	structs := map[string]*ast.StructType{}
	for _, statement := range program.Statements {
		switch decl := statement.(type) {
		case *ast.FunctionDeclaration:
			functions[decl.Name.Value] = decl
		case *ast.TypeDeclStatement:
			if decl.StructType != nil {
				structs[decl.Name.Value] = decl.StructType
			}
		}
	}
	if len(m.Functions) != len(functions) || len(m.Structs) != len(structs) {
		t.Fatal("slice lowering added or removed source functions/structures")
	}
	for _, definition := range m.Structs {
		decl := structs[definition.Name]
		if decl == nil || len(definition.Fields) != len(decl.Fields) || definition.LayoutRef != "" {
			t.Fatal("slice lowering introduced a descriptor or selected a layout", definition.Name)
		}
		for i, field := range definition.Fields {
			if field.Name != decl.Fields[i].Name.Value {
				t.Fatal("source aggregate acquired a hidden field", field.Name)
			}
		}
	}
	byID := map[FunctionID]*Function{}
	for _, fn := range m.Functions {
		byID[fn.ID] = fn
	}
	for _, fn := range m.Functions {
		decl := functions[fn.Name]
		if decl == nil || len(fn.Parameters) != len(decl.Parameters) {
			t.Fatal("slice lowering introduced a helper or hidden ABI argument", fn.Name)
		}
		for i, parameter := range fn.Parameters {
			if parameter.Name != decl.Parameters[i].Name.Value {
				t.Fatal("slice lowering changed source parameters", fn.Name, parameter.Name)
			}
		}
		expected := map[string]int{}
		if err := walk.Inspect(decl.Body, func(node any) error {
			if call, ok := node.(*ast.CallExpression); ok && call.Function != nil && functions[call.Function.Value] != nil {
				expected[call.Function.Value]++
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		actual := map[string]int{}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind != OpDirectCall {
					continue
				}
				callee := byID[op.Callee]
				if callee == nil || len(op.Operands) != len(callee.Parameters) {
					t.Fatal("slice lowering introduced a runtime helper or hidden call argument", op.Callee)
				}
				actual[callee.Name]++
			}
		}
		if !maps.Equal(actual, expected) {
			t.Fatalf("%s: source calls %v became %v (hidden runtime/allocation/cleanup call)", fn.Name, expected, actual)
		}
	}
}
