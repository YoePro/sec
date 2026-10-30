package semantic

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"sec/internal/codegen/targetplan"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// package15Fixtures are the successful source fixtures of the Package 15
// Place/reference tests.
func package15Fixtures() map[string]string {
	return map[string]string{
		"payload":       payloadReferenceFixture,
		"remaining":     remainingPayloadFixture,
		"field-subject": fieldSubjectPayloadFixture,
		"borrow-end":    payloadBorrowEndFixture,
		"place-borrow":  placeBorrowIntegrationFixture,
		"parameters":    referenceParameterFixture,
		"reference-use": referenceUseFixture,
		"returned":      returnedReferenceFixture,
		"matrix":        package15MatrixFixture,
	}
}

// buildPackage15Source parses, analyzes with the given target plan (the host
// plan when triple is empty) and builds source at package 15, returning Sema
// rejections as errors instead of failing the test.
func buildPackage15Source(t *testing.T, source, triple string) (*Module, error) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "sample.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if triple != "" {
		plan, err := targetplan.Plan(triple)
		if err != nil {
			t.Fatal(err)
		}
		analyzer = sema.NewAnalyzerWithScalarPlan(plan)
	}
	if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
		return nil, fmt.Errorf("sema: %v", errs)
	}
	return Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"sample.sec"}, MaxPackage: 15})
}

// Successful Package 15 modules stay at the semantic level (P15-66): no
// physical offset/stride, GEP, pointer arithmetic, MemRef/LLVM reference
// form, raw-pointer collapse, address-plus-epoch aggregate, side table,
// capability, hidden ABI argument, runtime borrow counter, generation
// manager, hidden allocation, undef or poison. The checks are structural —
// operation kinds, value classes, reference/Place metadata fields and
// parameters — and textual over the deterministic printer.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 2, 63, 131, 155
//   - rules/compiler/semantic_ir.md — §§ 45(6), 96(5)
func TestPackage15ModulesHaveNoPhysicalVocabulary(t *testing.T) {
	allowed := map[OpKind]bool{}
	for _, kind := range []OpKind{
		OpConstInt, OpConstBool, OpConstString, OpReturn, OpStorageDeclare, OpStorageInit, OpStorageLoad, OpStorageStore,
		OpDirectCall, OpBranch, OpCondBranch, OpUnreachable, OpIntBinaryChecked, OpIntCompare, OpArithmeticFailure,
		OpResultIsErr, OpUnionConstruct, OpUnionIsVariant, OpStructConstruct, OpStructExtractField, OpArrayConstruct,
		OpBoundsFailure, OpBoolNot, OpPanic,
		OpPlaceValueRoot, OpPlaceStorageRoot, OpPlaceField, OpPlaceArrayIndexInBounds, OpPlaceArrayElement, OpPlaceUnionPayload,
		OpReferenceDerefPlace, OpPlaceRead, OpPlaceWrite, OpReferenceBorrowShared, OpReferenceBorrowMutable, OpReferenceCopyShared,
		OpReferenceMove, OpReferenceReborrowShared, OpReferenceReborrowMutable, OpReferenceIsValid, OpReferenceGenerationFailure,
		OpReferenceEndBorrow, OpReferenceCompare,
	} {
		allowed[kind] = true
	}
	forbidden := []string{"offset", "stride", "gep", "getelementptr", "memref", "llvm.", "!llvm", "ptr<", "inttoptr", "ptrtoint",
		"undef", "poison", "malloc", "alloca", "side-table", "capability", "borrow-count", "generation-manager", "epoch-pair"}
	for name, source := range package15Fixtures() {
		module, err := buildPackage15Source(t, source, "")
		if err != nil {
			t.Fatalf("%s: build: %v", name, err)
		}
		if err := Verify(module); err != nil {
			t.Fatalf("%s: verify: %v", name, err)
		}
		text := strings.ToLower(Format(module))
		for _, word := range forbidden {
			if strings.Contains(text, word) {
				t.Errorf("%s: module mentions physical vocabulary %q", name, word)
			}
		}
		for index := 1; ; index++ {
			typ, ok := module.Types.Lookup(TypeID(index))
			if !ok {
				break
			}
			if typ.Kind == TypeRawPointer {
				t.Errorf("%s: a reference collapsed into a raw pointer type", name)
			}
		}
		for _, fn := range module.Functions {
			for _, parameter := range fn.Parameters {
				if parameter.Name == "" || strings.HasPrefix(parameter.Name, "$") || strings.HasPrefix(parameter.Name, "__") {
					t.Errorf("%s: %s has a hidden ABI parameter %q", name, fn.ID, parameter.Name)
				}
			}
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					if !allowed[op.Kind] {
						t.Errorf("%s: %s uses operation %s outside the Package 15 semantic set", name, fn.ID, op.Kind)
					}
					for _, result := range op.Results {
						if typ, _ := module.Types.Lookup(result.Type); typ.Kind == TypeReference && result.Ownership != OwnershipSharedReference && result.Ownership != OwnershipMutableReference {
							t.Errorf("%s: reference %%%d has ownership %s", name, result.ID, result.Ownership)
						}
					}
				}
			}
		}
	}
	// The reference and Place fact records carry semantic identities only.
	for _, record := range []any{ReferenceMetadata{}, PlaceMetadata{}} {
		typ := reflect.TypeOf(record)
		for index := 0; index < typ.NumField(); index++ {
			field := strings.ToLower(typ.Field(index).Name)
			physical := false
			for _, word := range []string{"offset", "stride", "pointer", "address", "counter", "table", "capability", "abi"} {
				physical = physical || strings.Contains(field, word)
			}
			if physical && field != "addressspace" && field != "addressstability" {
				t.Errorf("%s.%s is a physical representation field", typ.Name(), typ.Field(index).Name)
			}
		}
	}
}

// Every construct outside the Package 15 boundary is rejected as a whole,
// with no partially assembled module (P15-67): slice borrows and slice
// reference parameters, references into dynamic arrays (collection
// invalidation), foreign reference parameters (physical reference ABI),
// move-only referents with custom free (non-trivial read, write and
// destruction) and statics without a represented scalar compile-time value.
// Stable/weak handles, pinning and
// RawPtr-to-reference conversion have no Sec 0.1 source form to test
// (reference_model.md §§ 3(3), 10(9), 26(5); storage.md § 24(4); MD-070).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 3, 155
//   - rules/compiler/semantic_ir.md — § 64(4)
func TestPackage15RejectsOutOfScopeReferencesWithoutPartialModule(t *testing.T) {
	const moveOnly = `
type Handle struct {
    id: int,
}

impl Handle {
    free {
        Release(self.id)
    }
}

fn Release(id: int) void {
}

type Holder struct {
    inner: Handle,
    count: int,
}
`
	for name, test := range map[string]struct{ source, want string }{
		"slice borrow": {`
fn Use() int {
    let values := [1, 2, 3]
    let view := ref values[..]
    return 0
}
`, "borrow of reference"},
		"slice reference parameter": {`
fn Sum(values: ref int[]) int {
    return 0
}
`, "type int[]"},
		"dynamic array element": {`
fn First(values: int[]) int {
    let first := ref values[0]
    return 0
}
`, "type int[]"},
		"foreign reference parameter": {`
extern "C" fn Fill(value: ref mut int) void
`, "reference parameter"},
		"move-only referent write": {moveOnly + `
fn Set(holder: ref mut Holder) int {
    holder.inner = Handle { id: 2 }
    return 0
}
`, "custom free"},
		"move-only referent read": {moveOnly + `
fn Count(holder: ref Holder) int {
    return holder.inner.id
}
`, "custom free"},
		"non-scalar static root": {`
let label: string := "fixed"

fn Use() int {
    let fixed := ref label
    return 0
}
`, "has no resolved scalar compile-time value"},
	} {
		t.Run(name, func(t *testing.T) {
			module, err := buildPackage15Source(t, "module main\n"+test.source, "")
			var unsupported *UnsupportedFeatureError
			if module != nil || !errors.As(err, &unsupported) || unsupported.Package != 15 || !strings.Contains(unsupported.Feature, test.want) {
				t.Fatalf("module=%v err=%v, want no module and a package-15 boundary naming %q", module != nil, err, test.want)
			}
		})
	}
}

// Place/reference facts and modules are deterministic (P15-68): rebuilding
// from fresh parses and analyzers reproduces root, borrow and lifetime IDs,
// origin order and printing; a second build from the same analyzer neither
// differs nor mutates the first module; the source file name changes only
// locations; and 32-bit and 64-bit target plans each build their own module
// whose Place/reference structure is the same.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 10, 20, 155
//   - rules/compiler/semantic_ir.md — § 96
func TestPackage15PlaceReferenceFactsAreDeterministic(t *testing.T) {
	for name, source := range package15Fixtures() {
		first, err := buildPackage15Source(t, source, "")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		firstText := Format(first)
		second, err := buildPackage15Source(t, source, "")
		if err != nil || Format(second) != firstText {
			t.Errorf("%s: a fresh rebuild differs (%v)", name, err)
		}

		p := parser.New(lexer.NewWithFile(source, "sample.sec"))
		parsed := p.Parse()
		analyzer := sema.NewAnalyzer()
		if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
			t.Fatalf("%s: %v", name, errs)
		}
		options := BuildOptions{RequestedModule: "main", SourceFiles: []string{"sample.sec"}, MaxPackage: 15}
		shared, err := Build(parsed.Program, analyzer, options)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sharedText := Format(shared)
		again, err := Build(parsed.Program, analyzer, options)
		if err != nil || Format(again) != sharedText || Format(shared) != sharedText || sharedText != firstText {
			t.Errorf("%s: building twice from one analyzer is not stable or mutates the first module", name)
		}

		renamed := parser.New(lexer.NewWithFile(source, "renamed.sec"))
		renamedProgram := renamed.Parse()
		renamedAnalyzer := sema.NewAnalyzer()
		if errs := renamedAnalyzer.Analyze(renamedProgram.Program); len(errs) != 0 {
			t.Fatalf("%s: %v", name, errs)
		}
		moved, err := Build(renamedProgram.Program, renamedAnalyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"renamed.sec"}, MaxPackage: 15})
		if err != nil || strings.ReplaceAll(Format(moved), "renamed.sec", "sample.sec") != firstText {
			t.Errorf("%s: the source file name changed more than locations (%v)", name, err)
		}

		var structure []string
		for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
			module, err := buildPackage15Source(t, source, triple)
			if err != nil {
				t.Fatalf("%s %s: %v", name, triple, err)
			}
			if err := Verify(module); err != nil {
				t.Fatalf("%s %s: verify: %v", name, triple, err)
			}
			structure = append(structure, placeReferenceStructure(module))
		}
		if structure[0] != structure[1] {
			t.Errorf("%s: Place/reference structure differs between 64-bit and 32-bit plans:\n%s\n---\n%s", name, structure[0], structure[1])
		}
	}
}

// placeReferenceStructure lists the Place/reference operations of a module
// with their identities and facts, independent of scalar type numbering.
func placeReferenceStructure(module *Module) string {
	var out strings.Builder
	for _, fn := range module.Functions {
		fmt.Fprintf(&out, "%s %s\n", fn.Name, formatReturnOrigins(fn.ReturnOrigins))
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if !isPlaceReferenceOperation(op.Kind) {
					continue
				}
				fmt.Fprintf(&out, "  %s field=%d variant=%d", op.Kind, op.StructField, op.UnionVariant)
				if op.Place != nil {
					fmt.Fprintf(&out, " place=%+v", *op.Place)
				}
				if op.Reference != nil {
					fmt.Fprintf(&out, " ref=%+v", *op.Reference)
				}
				out.WriteString("\n")
			}
		}
	}
	return out.String()
}
