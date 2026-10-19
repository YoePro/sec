package semantic

import (
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Slice type identity is exactly element identity plus shared/mutable mode;
// neither runtime extent nor origin/proof metadata may specialize the type.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§31–32,39,75–76;
// rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md — §§1–3.
func TestSliceTypesPreserveElementAndAuthority(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		table := NewTypeTable()
		integer := table.Intern(Type{Kind: TypeInt, Name: "int32", Signed: true, BitWidth: 32})
		other := table.Intern(Type{Kind: TypeInt, Name: "int256", Signed: true, BitWidth: 256})
		named := table.Intern(Type{Kind: TypeNamed, Name: "Score", Identity: "main::Score", Base: integer})
		reference := table.Intern(Type{Kind: TypeReference, Element: integer})
		array := table.Intern(Type{Kind: TypeArray, Element: integer, Length: "4"})
		shared, err := table.InternSlice(integer, false)
		if err != nil {
			t.Fatal(err)
		}
		mutable, err := table.InternSlice(integer, true)
		if err != nil {
			t.Fatal(err)
		}
		wide, err := table.InternSlice(other, false)
		if err != nil {
			t.Fatal(err)
		}
		nominal, err := table.InternSlice(named, false)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[TypeID]bool{}
		for _, id := range []TypeID{reference, array, shared, mutable, wide, nominal} {
			if seen[id] {
				t.Fatal("slice erased element, authority, reference or array distinction")
			}
			seen[id] = true
		}
		for _, test := range []struct {
			mutable bool
			want    TypeID
			kind    TypeKind
		}{
			{false, shared, TypeSharedSlice}, {true, mutable, TypeMutableSlice},
		} {
			id, err := table.InternSlice(integer, test.mutable)
			if err != nil || id != test.want {
				t.Fatalf("reintern: %d %v", id, err)
			}
			typ, _ := table.Lookup(id)
			if typ.Kind != test.kind || typ.Element != integer || typ.Length != "" || typ.BitWidth != 0 || typ.TargetSize {
				t.Fatalf("slice selected a physical or length-dependent representation: %+v", typ)
			}
		}
		// Distinct runtime and normalized ranges remain facts of two values
		// with exactly the same semantic type.
		short := SliceFacts{Element: integer, RuntimeLength: 10, StaticRange: &StaticSliceRange{big.NewInt(0), big.NewInt(2)}}
		long := SliceFacts{Element: integer, RuntimeLength: 11, DynamicRange: 20}
		for _, facts := range []SliceFacts{short, long} {
			id, err := table.InternSlice(facts.Element, facts.Mutable)
			if err != nil || id != shared {
				t.Fatal("value range changed slice type")
			}
		}
		module := &Module{Version: Version, Identity: "slice-types", Types: table, PointerWidthBits: width}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		text := Format(module)
		for _, want := range []string{"slice<!1>", "slice-mut<!1>", "slice<!2>", "slice<!3>"} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %s:\n%s", want, text)
			}
		}
		if Format(module) != text {
			t.Fatal("nondeterministic type printing")
		}
	}
}

// The model accepts resolved Sema element/mode facts without using nested
// reference/array shape to guess source meaning. The source builder still
// reports the missing slice-operation lowering as a whole-module boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§32–37,76;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestSliceTypesConsumeResolvedPlanFacts(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/sema/slice_creation_plans_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "slice-types.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	module := &Module{Version: Version, Identity: "main", Types: NewTypeTable()}
	b := &builder{module: module, analyzer: analyzer, maxPackage: 16}
	seen := map[sema.ResolvedSliceMode]TypeID{}
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		for _, statement := range function.Body.Statements {
			binding, ok := statement.(*ast.LetStatement)
			if !ok {
				continue
			}
			ref, ok := binding.Value.(*ast.RefExpression)
			if !ok {
				continue
			}
			plan, found := analyzer.ResolvedSliceCreationPlanOf(ref)
			if !found {
				continue
			}
			element, err := b.internType(plan.ElementType)
			if err != nil {
				t.Fatal(err)
			}
			id, err := module.Types.InternSlice(element, plan.Mode == sema.SliceMutable)
			if err != nil {
				t.Fatal(err)
			}
			if previous := seen[plan.Mode]; previous != 0 && previous != id {
				t.Fatal("source length/origin changed slice identity")
			}
			seen[plan.Mode] = id
		}
	}
	if seen[sema.SliceShared] == 0 || seen[sema.SliceMutable] == 0 || seen[sema.SliceShared] == seen[sema.SliceMutable] {
		t.Fatal("source mode distinction was lost")
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []uint8{14, 15, 16} {
		built, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: pkg})
		if built != nil || err == nil || !strings.Contains(err.Error(), "slice") {
			t.Fatalf("package %d silently accepted incomplete slice lowering: %v", pkg, err)
		}
	}
}

// Canonical slices have no attached length/layout/nominal metadata and cannot
// contain invalid or non-value element types. Failed interning is transactional.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§31–32,75–76.
func TestSliceTypeVerifierRejectsInvalidRepresentations(t *testing.T) {
	for _, kind := range []TypeKind{TypeSharedSlice, TypeMutableSlice} {
		for _, malformed := range []string{"missing", "void", "never", "place", "length", "width", "name", "authority-flag", "arguments"} {
			table := NewTypeTable()
			element := table.Intern(Type{Kind: TypeInt, Name: "int32", BitWidth: 32, Signed: true})
			typ := Type{Kind: kind, Element: element}
			switch malformed {
			case "missing":
				typ.Element = 999
			case "void":
				typ.Element = table.Intern(Type{Kind: TypeVoid})
			case "never":
				typ.Element = table.Intern(Type{Kind: TypeNever})
			case "place":
				typ.Element = table.Intern(Type{Kind: TypePlace, Element: element})
			case "length":
				typ.Length = "3"
			case "width":
				typ.BitWidth = 64
			case "name":
				typ.Name = "descriptor"
			case "authority-flag":
				typ.Mutable = true
			case "arguments":
				typ.TypeArgs = []TypeID{element}
			}
			table.Intern(typ)
			if err := Verify(&Module{Version: Version, Identity: "invalid-slice", Types: table}); err == nil || !strings.Contains(err.Error(), "slice") {
				t.Fatalf("accepted %s %s: %v", kind, malformed, err)
			}
		}
	}
	table := NewTypeTable()
	table.Intern(Type{Kind: TypeVoid})
	before := len(table.types)
	if _, err := table.InternSlice(1, false); err == nil || len(table.types) != before {
		t.Fatal("invalid slice was interned")
	}
	var absent *TypeTable
	if _, err := absent.InternSlice(1, false); err == nil {
		t.Fatal("nil type table accepted a slice")
	}
}

// Facts retain the existing P15 borrow, epoch, storage and validity contract
// while exposing detached exact ranges and origin slices to later consumers.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§25,31,37,44;
// rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md — §§2–5.
func TestSliceFactsClonePreservesReferenceContract(t *testing.T) {
	start := new(big.Int).Lsh(big.NewInt(1), 100)
	original := SliceFacts{
		Element: 1, Mutable: true, OriginPlaces: []ValueID{2, 3}, RuntimeLength: 4, DynamicRange: 5,
		Reference: ReferenceMetadata{Borrow: 6, Lifetime: 7, SourceBorrow: 8, Validity: ReferenceDynamicEpoch,
			InvalidationDomain: 9, EpochDependency: 10, AddressSpace: AddressSpaceOrdinary,
			AddressStability: "stable", Pinned: true, Provenance: "parameter", OriginStorages: []uint32{11, 12}},
		StaticRange: &StaticSliceRange{Start: start, EndExclusive: new(big.Int).Add(start, big.NewInt(7))},
	}
	snapshot := original.Clone()
	if !reflect.DeepEqual(snapshot, original) {
		t.Fatal("clone lost reference or range facts")
	}
	changed := original.Clone()
	changed.StaticRange.Start.SetInt64(999)
	changed.StaticRange.EndExclusive.SetInt64(999)
	changed.OriginPlaces[0] = 999
	changed.Reference.OriginStorages[0] = 999
	if !reflect.DeepEqual(original, snapshot) {
		t.Fatal("clone exposes original mutable data")
	}
	original.OriginPlaces[1] = 99
	original.StaticRange.Start.SetInt64(0)
	if snapshot.OriginPlaces[1] != 3 || snapshot.StaticRange.Start.BitLen() != 101 {
		t.Fatal("original mutated earlier snapshot")
	}
	empty := SliceFacts{Reference: snapshot.Reference, StaticRange: &StaticSliceRange{big.NewInt(3), big.NewInt(3)}}
	copied := empty.Clone()
	if copied.StaticRange.Start.Cmp(copied.StaticRange.EndExclusive) != 0 || copied.Reference.Borrow == 0 {
		t.Fatal("empty range lost borrow identity")
	}
	if got := (SliceFacts{}).Clone(); got.StaticRange != nil || got.DynamicRange != 0 || len(got.OriginPlaces) != 0 {
		t.Fatal("clone invented facts")
	}
}
