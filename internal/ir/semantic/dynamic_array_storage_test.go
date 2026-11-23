package semantic

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// hostedModule builds Semantic IR under the hosted allocation profile, the
// one that gives Append a resolved allocation context.
func hostedModule(t *testing.T, source string, maxPackage uint8) (*Module, error) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "sample.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	a := sema.NewAnalyzer()
	a.SetAllocationProfile("hosted")
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatalf("sema errors: %v", errs)
	}
	return Build(parsed.Program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{"sample.sec"}, MaxPackage: maxPackage})
}

// `values.Append(x)` evaluates the receiver Place, then the element, and
// splits on the owner's backing state: an owner without backing grows with
// establish-domain and establishes its domain, one without spare capacity
// grows with advance-epoch and advances it; both continue to
// append-prepared. A failed growth leaves the owner unchanged and yields
// CollectionError.AllocationFailed without a panic endpoint. The
// destruction plan ends the backing domain and reclaims per plan.
// Rules: rules/collections/collections.md — §§ 6.4, 6.7, 16;
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 25–37, 54, 58–67.
func TestDynamicArrayAppendSemanticIR(t *testing.T) {
	source := `module main

fn Fill() Result[uint, CollectionError] {
    let mut values: int[]
    try values.Append(4)
    try values.Append(5)
    return Ok(values.Len)
}

fn main() int {
    return 0
}
`
	module, err := hostedModule(t, source, 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	facts := "allocation=#1 context=#1 origin=Arena relation=Owned authority=Arena stability=Movable space=ordinary}"
	for _, want := range []string{
		"dynamic-array.has-backing %2", "dynamic-array.grow-backing %2 count=1 {domain=#1 transition=establish-domain " + facts,
		"storage.establish-domain %2 {domain=#1 transition=establish-domain", "dynamic-array.has-capacity %2 count=1",
		"dynamic-array.grow-backing %2 count=1 {domain=#1 transition=advance-epoch", "storage.advance-epoch %2 {domain=#1 transition=advance-epoch",
		"dynamic-array.append-prepared %2, %3", "result.ok", "enum.constant case=#0", "cleanup.run-function exit=error-propagation",
		`backing=[end-domain,reclaim-per-allocation-plan]`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Count(text, "dynamic-array.append-prepared") != 2 || strings.Count(text, "dynamic-array.grow-backing") != 4 {
		t.Errorf("want one append and two growth forms per Append:\n%s", text)
	}
	for _, unwanted := range []string{"fail.bounds", "panic", "storage.reclaim"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("Append must not lower %q (Arena authority reclaims nothing per array):\n%s", unwanted, text)
		}
	}

	for _, item := range []struct {
		source  string
		pkg     uint8
		feature string
	}{
		{"fn F() Result[void, CollectionError] {\n    let mut values: int[][]\n    try values.Append([])\n    return Ok()\n}\n", 18, "Append of a non-trivial element"},
		{"fn F() Result[void, CollectionError] {\n    let mut values: int[]\n    try values.Append(4)\n    return Ok()\n}\n", 17, "allocation lowering is unsupported"},
	} {
		_, err := hostedModule(t, "module main\n\n"+item.source+"\nfn main() int {\n    return 0\n}\n", item.pkg)
		if err == nil || !strings.Contains(err.Error(), item.feature) {
			t.Errorf("package %d %q: want %q, got %v", item.pkg, item.source, item.feature, err)
		}
	}
}

// The storage-transition verifier keeps growth transactional: every growth
// is tested on failed, a transition opens only the matching success edge,
// append-prepared is reachable only after capacity is ensured, reclamation
// needs its owning-value plan, and an allocation candidate never reaches its
// failure edge.
func TestDynamicArrayStorageTransitionVerifier(t *testing.T) {
	source := `module main

fn Fill() Result[uint, CollectionError] {
    let mut values: int[]
    try values.Append(4)
    return Ok(values.Len)
}

fn main() int {
    return 0
}
`
	fresh := func() (*Module, *Function) {
		module, err := hostedModule(t, source, 18)
		if err != nil {
			t.Fatal(err)
		}
		return module, loopFunction(t, module, "Fill")
	}
	find := func(fn *Function, kind OpKind) (*Block, int) {
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind == kind {
					return block, index
				}
			}
		}
		t.Fatalf("no %s", kind)
		return nil, 0
	}
	for name, mutate := range map[string]func(*Module, *Function){
		"unknown authority": func(_ *Module, fn *Function) {
			block, index := find(fn, OpDynamicArrayGrowBacking)
			backing := *block.Operations[index].Backing
			backing.Authority = string(sema.ReclamationUnknown)
			block.Operations[index].Backing = &backing
		},
		"bad transition": func(_ *Module, fn *Function) {
			block, index := find(fn, OpDynamicArrayGrowBacking)
			backing := *block.Operations[index].Backing
			backing.Transition = "none"
			block.Operations[index].Backing = &backing
		},
		"advance after establish": func(_ *Module, fn *Function) {
			block, index := find(fn, OpStorageEstablishDomain)
			block.Operations[index].Kind = OpStorageAdvanceEpoch
		},
		"append without capacity": func(_ *Module, fn *Function) {
			block, index := find(fn, OpDynamicArrayHasCapacity)
			branch := &block.Operations[index+1]
			branch.Successors[0], branch.Successors[1] = branch.Successors[1], branch.Successors[0]
		},
		"reclaim without plan": func(_ *Module, fn *Function) {
			block, index := find(fn, OpStorageAdvanceEpoch)
			op := block.Operations[index]
			reclaim := Operation{Kind: OpStorageReclaim, Operands: op.Operands, Backing: op.Backing, Location: op.Location}
			block.Operations = append(block.Operations[:index+1], append([]Operation{reclaim}, block.Operations[index+1:]...)...)
		},
		"plan without end-domain": func(module *Module, _ *Function) {
			for index := range module.DestructionPlans {
				if module.DestructionPlans[index].Kind == "dynamic-array" {
					module.DestructionPlans[index].BackingTransitions = []string{"reclaim-per-allocation-plan"}
				}
			}
		},
	} {
		module, fn := fresh()
		mutate(module, fn)
		if err := Verify(module); err == nil {
			t.Errorf("%s: malformed growth accepted", name)
		}
	}

	table := NewTypeTable()
	intType := table.Intern(Type{Kind: TypeInt, Name: "int", TargetSize: true})
	array, _ := table.InternDynamicArray(intType)
	boolean := table.Intern(Type{Kind: TypeBool, Name: "bool"})
	allocationError := table.Intern(Type{Kind: TypeEnum, Name: "AllocationError", Identity: "core::AllocationError"})
	allocate := Operation{
		Kind: OpDynamicArrayAllocate, Integer: big.NewInt(4),
		Results: []Value{{ID: 1, Type: array, Ownership: OwnershipOwned}, {ID: 2, Type: boolean}, {ID: 3, Type: allocationError}},
		Backing: &BackingMetadata{Domain: 1, Transition: "establish-domain", Allocation: 1, Context: 1, Origin: "Arena", Relation: "Owned", Authority: "Arena", Stability: "Movable", Space: "ordinary"},
	}
	if err := verifyDynamicArrayStorageOperation(table, allocate, nil); err != nil {
		t.Fatalf("canonical allocate rejected: %v", err)
	}
	borrowed := allocate
	borrowed.Results = append([]Value{{ID: 1, Type: array, Ownership: OwnershipImmediate}}, allocate.Results[1:]...)
	if err := verifyDynamicArrayStorageOperation(table, borrowed, nil); err == nil {
		t.Error("an allocation candidate must be owned")
	}
	owning := allocate
	owning.Backing = &BackingMetadata{Domain: 1, Transition: "establish-domain", Allocation: 1, Context: 1, Origin: "Heap", Relation: "Owned", Authority: "OwningValue", Stability: "Movable", Space: "ordinary"}
	if err := verifyDynamicArrayStorageOperation(table, owning, nil); err == nil {
		t.Error("owning-value authority requires its paired reclamation plan")
	}
	escaped := &Function{Blocks: []*Block{
		{ID: 0, Operations: []Operation{allocate, {Kind: OpCondBranch, Operands: []ValueID{2}, Successors: []BranchTarget{{Block: 1, Arguments: []ValueID{1}}, {Block: 2}}}}},
		{ID: 1}, {ID: 2},
	}}
	if err := verifyDynamicArrayTransitions(escaped); err == nil {
		t.Error("an allocation candidate escaped on its failure edge")
	}
}
