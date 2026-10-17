package secmlir

import (
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// package15EmitFixtures are successful Package 15 programs covering storage,
// field, element and payload Places, borrows and reborrows, reference
// parameters and calls, reference equality, borrow ends and returned
// references with their origin summaries.
var package15EmitFixtures = map[string]string{
	"returned": `module main

type Cell struct {
    v: int,
}

type Pair struct {
    left: Cell,
    right: Cell,
    cells: Cell[3],
}

impl Pair {
    fn First() ref Cell {
        return ref self.left
    }
}

fn Whole(pair: ref Pair) ref Pair {
    return pair
}

fn Left(pair: ref Pair) ref Cell {
    return ref pair.left
}

fn RightMut(pair: ref mut Pair) ref mut Cell {
    return ref mut pair.right
}

fn Pick(a: ref Cell, b: ref Cell, first: bool) ref Cell {
    if first {
        return a
    }
    return b
}

fn Element(pair: ref Pair, at: int) ref Cell {
    return ref pair.cells[at]
}

fn Constant(pair: ref Pair) ref Cell {
    return ref pair.cells[1]
}

fn Again(pair: ref Pair) ref Cell {
    return Left(pair)
}

fn Use() int {
    let mut pair := Pair { left: Cell { v: 1 }, right: Cell { v: 2 }, cells: [Cell { v: 3 }, Cell { v: 4 }, Cell { v: 5 }] }
    let left := Left(ref pair)
    let x := left.v
    let right := RightMut(ref mut pair)
    right.v = 5
    return x
}
`,
	"place-borrow": `module main

type Cell struct {
    v: int,
    w: int,
}

type Inner struct {
    count: Cell,
    limit: Cell,
}

type Wide struct {
    small: int128,
    large: uint256,
}

type Outer struct {
    inner: Inner,
    wide: Wide,
}

fn Disjoint() int {
    let mut outer := Outer { inner: Inner { count: Cell { v: 1, w: 1 }, limit: Cell { v: 2, w: 2 } }, wide: Wide { small: 3, large: 4 } }
    let count := ref mut outer.inner.count
    let limit := ref mut outer.inner.limit
    count.v = 5
    limit.v = 6
    return 0
}

fn Nested(outer: ref mut Outer) int {
    let count := ref mut outer.inner.count
    count.v = 7
    let limit := ref outer.inner.limit
    return limit.v
}

fn Elements() int {
    let mut grid := [Cell { v: 1, w: 1 }, Cell { v: 2, w: 2 }, Cell { v: 3, w: 3 }]
    let first := ref mut grid[0]
    let second := ref mut grid[1]
    first.v = 4
    second.v = 5
    return 0
}

fn Index(at: int) int {
    let values := [Cell { v: 1, w: 1 }, Cell { v: 2, w: 2 }, Cell { v: 3, w: 3 }]
    let a := ref values[at]
    return a.v
}
`,
	"payload": `module main

enum Problem error {
    Bad,
}

type Point struct {
    x: int,
    y: int,
}

type Figure union {
    Dot(Point)
    Empty
}

fn Read(figure: Figure) int {
    match figure {
        Dot(ref point) => {
            return point.x
        }
        Empty => {
            return 0
        }
    }
}

fn Move() int {
    let mut figure := Figure.Dot(Point { x: 1, y: 2 })
    match figure {
        Dot(ref mut point) => {
            point.y = 7
        }
        Empty => {
        }
    }
    return 0
}

fn Failure(value: Result[Point, Problem]) int {
    match value {
        Ok(_) => {
            return 1
        }
        Err(ref problem) => {
            return 0
        }
    }
}

fn ResultWrites(input: Result[Point, Problem]) int {
    let mut value := input
    match value {
        Ok(ref mut point) => {
            point.x = 3
        }
        Err(_) => {
        }
    }
    return 0
}
`,
	"parameters": `module main

type Pair struct {
    left: int,
    right: int,
}

fn Peek(pair: ref Pair) int {
    return pair.left
}

fn Bump(pair: ref mut Pair) void {
    pair.right = 3
}

fn Use() int {
    let pair := Pair { left: 1, right: 2 }
    let a := Peek(pair)
    let p := ref pair
    let b := Peek(p)
    let same := p == ref pair
    let mut state := Pair { left: 1, right: 2 }
    Bump(state)
    let w := ref mut state
    Bump(w)
    return a + b
}
`,
}

func buildPackage15EmitModule(t *testing.T, name, source string) *semantic.Module {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, name+".sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatalf("sema: %v", errs)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{name + ".sec"}, MaxPackage: 15})
	if err != nil {
		t.Fatalf("semantic build: %v", err)
	}
	return module
}

// verifyPackage15MLIR runs the three schema-11 verifier passes over emitted
// text when SEC_MLIR_BIN names the built tools.
func verifyPackage15MLIR(t *testing.T, name string, output []byte) {
	t.Helper()
	binDir := os.Getenv("SEC_MLIR_BIN")
	if binDir == "" {
		return
	}
	tool, err := testsupport.ConfiguredSecMLIROptPath(binDir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name+".mlir")
	if err := os.WriteFile(path, output, 0600); err != nil {
		t.Fatal(err)
	}
	verifiers := []string{"--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics"}
	command := exec.Command(tool, append([]string{path}, verifiers...)...)
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sec-mlir-opt: %v\n%s\n%s", err, combined, output)
	}
	// P15-59/P15-60: the P5/P6 scalar pipeline keeps reference-dependent
	// storage and every Place/reference operation high-level, resolving only
	// the referents, and the result still verifies.
	scalar := exec.Command(tool, append([]string{path, "--sec-lower-scalar-core"}, verifiers...)...)
	lowered, err := scalar.CombinedOutput()
	if err != nil {
		t.Fatalf("sec-lower-scalar-core: %v\n%s", err, lowered)
	}
	if strings.Contains(string(lowered), "!sec.int") || strings.Contains(string(lowered), "!sec.uint") {
		t.Errorf("target-sized integers survived scalar resolution:\n%s", lowered)
	}
	for _, line := range strings.Split(string(lowered), "\n") {
		if strings.Contains(line, "memref.alloca") && strings.Contains(line, "sec.address_taken") {
			t.Errorf("reference-dependent storage was lowered to MemRef: %s", line)
		}
	}
	// P8 rejects remaining Place/reference semantics instead of normalizing
	// through the wrappers.
	integer := exec.Command(tool, path, "--sec-lower-integer-core")
	if rejected, err := integer.CombinedOutput(); err == nil || !strings.Contains(string(rejected), "Place/reference semantics must be discharged") {
		t.Fatalf("sec-lower-integer-core accepted Place/reference semantics: %v\n%s", err, rejected)
	}
}

// Verified Semantic IR Places and references lower to schema-11 Sec MLIR on
// 32- and 64-bit plans: semantic Place roots and projections, complete
// reference facts, reference parameter facts, returned-origin summaries and
// address-taken storage, with no physical representation; the output is
// deterministic and passes the three schema-11 verifiers (P15-57).
//
// Rules:
//   - rules/mlir/lowering-versions/sec_mlir_lowering_v11.md — sections 1-28
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — sections 59-67, 155
func TestPackage15EmitsSchema11PlacesAndReferences(t *testing.T) {
	expected := map[string][]string{
		"returned": {
			`sec.return_origins = ["param#0.field#0"]`,
			`sec.return_origins = ["param#0", "param#1"]`,
			`sec.return_origins = ["param#0.field#2[*]"]`,
			`sec.return_origins = ["param#0.field#2[1]"]`,
			`sec.return_origins = ["receiver.field#0"]`,
			`: !sec.ref<!sec.struct<identity = "main::Pair"`,
			`{sec.reference = {borrow_id = `,
			`validity_policy = "proven", validity_proof = "caller-owned-borrow", address_stability = "Stable", provenance = "parameter"}`,
			`"sec.place.deref"(`,
			`"sec.ref.reborrow_mut"(`,
			`"sec.ref.copy_shared"(`,
			`"sec.place.array_index_in_bounds"(`,
			`<{bounds_kind = "runtime-check", bounds_proof = "guarded"}>`,
			`<{bounds_kind = "proven-safe", bounds_proof = "constant"}>`,
			`sec.address_taken = true`,
			`"sec.place.storage"(%s`,
			`sec.argument_actions = ["shared-reference"], sec.reference = {`,
			`sec.argument_actions = ["mutable-reference"], sec.reference = {`,
			`{sec.origin_storages = [`,
			`"sec.place.write"(`,
		},
		"place-borrow": {`"sec.place.field"(`, `"sec.ref.borrow_mut"(`, `"sec.place.read"(`, `<{action = "copy-trivial"}>`},
		"payload":      {`"sec.ref.end_borrow"(`, `"sec.place.union_payload"(`, `<{variant = 1 : i32}>`, `"sec.place.value"(`},
		"parameters":   {`"sec.ref.compare"(`, `<{predicate = "eq"}>`, `"sec.ref.borrow_shared"(`},
	}
	for name, source := range package15EmitFixtures {
		for _, width := range []uint16{32, 64} {
			t.Run(name+"/"+strconv.Itoa(int(width)), func(t *testing.T) {
				module := buildPackage15EmitModule(t, name, source)
				before := semantic.Format(module)
				output, err := Emit(module, testPlan(width))
				if err != nil {
					t.Fatalf("emit: %v\n%s", err, before)
				}
				if semantic.Format(module) != before {
					t.Fatal("emission mutated the Semantic IR module")
				}
				again, err := Emit(buildPackage15EmitModule(t, name, source), testPlan(width))
				if err != nil || string(again) != string(output) {
					t.Fatalf("emission is not deterministic: %v", err)
				}
				text := string(output)
				for _, want := range append([]string{`sec.dialect_version = 11 : i32`}, expected[name]...) {
					if !strings.Contains(text, want) {
						t.Errorf("missing %q in:\n%s", want, text)
					}
				}
				for _, physical := range []string{"memref", "llvm.", "getelementptr", "ptr<", "offset", "stride", "epoch_field"} {
					if strings.Contains(text, physical) {
						t.Errorf("schema-11 output names physical representation %q", physical)
					}
				}
				verifyPackage15MLIR(t, name, output)
			})
		}
	}
}

// A dynamic-epoch reference parameter and a dynamic local borrow lower with
// their invalidation domain and epoch dependency, the validity guard CFG and
// the invalid-reference-generation endpoint; mutable moves, shared copies and
// reference equality keep their semantic operations (P15-57).
func TestPackage15EmitsDynamicReferenceGuards(t *testing.T) {
	types := semantic.NewTypeTable()
	intType := types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int", Signed: true, TargetSize: true})
	boolType := types.Intern(semantic.Type{Kind: semantic.TypeBool, Name: "bool"})
	placeRO := types.Intern(semantic.Type{Kind: semantic.TypePlace, Element: intType})
	placeRW := types.Intern(semantic.Type{Kind: semantic.TypePlace, Element: intType, Mutable: true})
	refInt := types.Intern(semantic.Type{Kind: semantic.TypeReference, Element: intType})
	refMutInt := types.Intern(semantic.Type{Kind: semantic.TypeReference, Element: intType, Mutable: true})
	v := func(id semantic.ValueID, typ semantic.TypeID, ownership semantic.OwnershipClass) semantic.Value {
		return semantic.Value{ID: id, Type: typ, Ownership: ownership}
	}
	dynamic := func(borrow uint32) *semantic.ReferenceMetadata {
		return &semantic.ReferenceMetadata{Borrow: borrow, Lifetime: borrow, Validity: semantic.ReferenceDynamicEpoch, InvalidationDomain: 1, EpochDependency: 2,
			AddressSpace: semantic.AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "arena"}
	}
	proven := func(borrow, source uint32) *semantic.ReferenceMetadata {
		return &semantic.ReferenceMetadata{Borrow: borrow, Lifetime: borrow, SourceBorrow: source, Validity: semantic.ReferenceProven, ProofProvenance: "lexical-borrow",
			AddressSpace: semantic.AddressSpaceOrdinary, AddressStability: "Stable", Provenance: "local", OriginStorages: []uint32{1}}
	}
	imm, tmp := semantic.OwnershipImmediate, semantic.OwnershipCompilerTemporary
	failure := func(id semantic.BlockID) *semantic.Block {
		return &semantic.Block{ID: id, Operations: []semantic.Operation{{Kind: semantic.OpReferenceGenerationFailure, PanicReasonID: 8, PanicReason: "InvalidReferenceGeneration"}}}
	}
	parameter := &semantic.Function{ID: "main::Read(ref int)", Name: "Read", ReturnType: intType, Entry: 0,
		Parameters: []semantic.Parameter{{Name: "value", Value: v(1, refInt, semantic.OwnershipSharedReference), Reference: dynamic(1)}}}
	parameter.Blocks = []*semantic.Block{
		{ID: 0, Operations: []semantic.Operation{
			{Kind: semantic.OpReferenceIsValid, Operands: []semantic.ValueID{1}, Results: []semantic.Value{v(2, boolType, imm)}, Reference: dynamic(1)},
			{Kind: semantic.OpCondBranch, Operands: []semantic.ValueID{2}, Successors: []semantic.BranchTarget{{Block: 1}, {Block: 2}}},
		}},
		{ID: 1, Operations: []semantic.Operation{
			{Kind: semantic.OpReferenceDerefPlace, Operands: []semantic.ValueID{1}, ReferenceGuard: 2, Results: []semantic.Value{v(3, placeRO, tmp)}, Reference: dynamic(1)},
			{Kind: semantic.OpPlaceRead, Operands: []semantic.ValueID{3}, Results: []semantic.Value{v(4, intType, imm)}},
			{Kind: semantic.OpReturn, Operands: []semantic.ValueID{4}},
		}},
		failure(2),
	}
	local := &semantic.Function{ID: "main::Local()", Name: "Local", ReturnType: intType, Entry: 0,
		Storages: []semantic.Storage{{ID: 1, Name: "slot", Type: intType, Mutable: true, Class: semantic.StorageLocalAutomatic, AddressTaken: true}}}
	local.Blocks = []*semantic.Block{{ID: 0, Operations: []semantic.Operation{
		{Kind: semantic.OpConstInt, Integer: big.NewInt(1), Results: []semantic.Value{v(1, intType, imm)}},
		{Kind: semantic.OpStorageDeclare, Storage: 1},
		{Kind: semantic.OpStorageInit, Storage: 1, Operands: []semantic.ValueID{1}},
		{Kind: semantic.OpPlaceStorageRoot, Storage: 1, Results: []semantic.Value{v(2, placeRW, tmp)}, Place: &semantic.PlaceMetadata{Root: 1, RootKind: semantic.PlaceRootLocal, Storage: 1, AddressSpace: semantic.AddressSpaceOrdinary}},
		{Kind: semantic.OpReferenceBorrowMutable, Operands: []semantic.ValueID{2}, Results: []semantic.Value{v(3, refMutInt, semantic.OwnershipMutableReference)}, Reference: proven(1, 0)},
		{Kind: semantic.OpReferenceMove, Operands: []semantic.ValueID{3}, Results: []semantic.Value{v(4, refMutInt, semantic.OwnershipMutableReference)}, Reference: proven(1, 1)},
		{Kind: semantic.OpReferenceReborrowShared, Operands: []semantic.ValueID{4}, Results: []semantic.Value{v(5, refInt, semantic.OwnershipSharedReference)}, Reference: proven(2, 1)},
		{Kind: semantic.OpReferenceCopyShared, Operands: []semantic.ValueID{5}, Results: []semantic.Value{v(6, refInt, semantic.OwnershipSharedReference)}, Reference: proven(2, 2)},
		{Kind: semantic.OpReferenceCompare, Operator: "ne", Operands: []semantic.ValueID{5, 6}, Results: []semantic.Value{v(7, boolType, imm)}},
		{Kind: semantic.OpReferenceDerefPlace, Operands: []semantic.ValueID{6}, Results: []semantic.Value{v(8, placeRO, tmp)}, Reference: proven(2, 2)},
		{Kind: semantic.OpPlaceRead, Operands: []semantic.ValueID{8}, Results: []semantic.Value{v(9, intType, imm)}},
		{Kind: semantic.OpReferenceEndBorrow, Operands: []semantic.ValueID{6}, Reference: proven(2, 2)},
		{Kind: semantic.OpReferenceEndBorrow, Operands: []semantic.ValueID{5}, Reference: proven(2, 1)},
		{Kind: semantic.OpReferenceEndBorrow, Operands: []semantic.ValueID{4}, Reference: proven(1, 1)},
		{Kind: semantic.OpReturn, Operands: []semantic.ValueID{9}},
	}}}
	module := &semantic.Module{Version: semantic.Version, Identity: "main", Types: types, Functions: []*semantic.Function{parameter, local}}
	if err := semantic.Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, semantic.Format(module))
	}
	for _, width := range []uint16{32, 64} {
		output, err := Emit(module, testPlan(width))
		if err != nil {
			t.Fatal(err)
		}
		text := string(output)
		for _, want := range []string{
			`%v1: !sec.ref<!sec.int> {sec.reference = {borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "dynamic-epoch", invalidation_domain = 1 : i32, epoch_dependency = 2 : i32, address_stability = "Stable", provenance = "arena"}}`,
			`%v2 = "sec.ref.is_valid"(%v1) : (!sec.ref<!sec.int>) -> i1`,
			`cf.cond_br %v2, ^bb1, ^bb2`,
			`"sec.fail.reference_generation"() <{reason = "panic.invalid-reference-generation"}> : () -> ()`,
			`"sec.ref.move"(%v3) <{borrow_id = 1 : i32, lifetime_id = 1 : i32, validity_policy = "proven", validity_proof = "lexical-borrow", address_stability = "Stable", provenance = "local"}> {sec.origin_storages = [1 : i32]}`,
			`source_borrow = 1 : i32}>`,
			`<{predicate = "ne"}>`,
		} {
			if !strings.Contains(text, want) {
				t.Errorf("missing %q in:\n%s", want, text)
			}
		}
		verifyPackage15MLIR(t, "dynamic", output)
	}
}

// Facts schema 11 cannot spell are rejected instead of dropped: a reference
// in a target-defined address space and a static Place root (P15-57).
func TestPackage15EmitRejectsUnrepresentedReferenceFacts(t *testing.T) {
	types := semantic.NewTypeTable()
	intType := types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int", Signed: true, TargetSize: true})
	refInt := types.Intern(semantic.Type{Kind: semantic.TypeReference, Element: intType})
	placeRO := types.Intern(semantic.Type{Kind: semantic.TypePlace, Element: intType})
	reference := &semantic.ReferenceMetadata{Borrow: 1, Lifetime: 1, Validity: semantic.ReferenceProven, ProofProvenance: "caller-owned-borrow",
		AddressSpace: "target:sram2", AddressStability: "Stable", Provenance: "parameter"}
	function := &semantic.Function{ID: "main::Read(ref int)", Name: "Read", ReturnType: intType, Entry: 0,
		Parameters: []semantic.Parameter{{Name: "value", Value: semantic.Value{ID: 1, Type: refInt, Ownership: semantic.OwnershipSharedReference}, Reference: reference}}}
	function.Blocks = []*semantic.Block{{ID: 0, Operations: []semantic.Operation{
		{Kind: semantic.OpReferenceDerefPlace, Operands: []semantic.ValueID{1}, Results: []semantic.Value{{ID: 2, Type: placeRO, Ownership: semantic.OwnershipCompilerTemporary}}, Reference: reference},
		{Kind: semantic.OpPlaceRead, Operands: []semantic.ValueID{2}, Results: []semantic.Value{{ID: 3, Type: intType, Ownership: semantic.OwnershipImmediate}}},
		{Kind: semantic.OpReturn, Operands: []semantic.ValueID{3}},
	}}}
	module := &semantic.Module{Version: semantic.Version, Identity: "main", Types: types, Functions: []*semantic.Function{function}}
	if err := semantic.Verify(module); err != nil {
		t.Fatalf("verify: %v", err)
	}
	output, err := Emit(module, testPlan(64))
	var unsupported *UnsupportedLoweringError
	if output != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "target:sram2") {
		t.Fatalf("output=%q err=%v", output, err)
	}
}
