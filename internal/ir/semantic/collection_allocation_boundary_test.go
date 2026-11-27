package semantic

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Every allocation-capable collection operation lowered in Sec 0.1 (Append,
// range Append and the rune[] materialization) keeps the lower-level
// AllocationError inside the allocation layer: the growth reports it on its
// failure edge, and the public operation maps it to
// CollectionError.AllocationFailed before the result leaves the collection
// API. The public failure is never AllocationError and is not implicitly
// convertible to it.
// Rules: rules/collections/collections.md — §§ 6.4, 16;
// rules/library/core-library.md — AllocationError and CollectionError.
func TestCollectionOperationsMapAllocationErrorAtBoundary(t *testing.T) {
	source := `module main

fn One() Result[uint, CollectionError] {
    let mut values: int[]
    try values.Append(4)
    return Ok(values.Len)
}

fn Range() Result[uint, CollectionError] {
    let mut values: int[]
    try values.Append(1..9)
    return Ok(values.Len)
}

fn Runes() Result[uint, CollectionError] {
    let text := "héj"
    let runes := try text.ToRuneArray()
    return Ok(runes.Len)
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
	for _, name := range []string{"One", "Range", "Runes"} {
		fn := loopFunction(t, module, name)
		blocks := map[BlockID]*Block{}
		for _, block := range fn.Blocks {
			blocks[block.ID] = block
		}
		growths := 0
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind != OpDynamicArrayGrowBacking {
					continue
				}
				growths++
				if len(op.Results) != 2 || !enumIdentity(module, op.Results[1].Type, "core::AllocationError") {
					t.Fatalf("%s: growth does not report AllocationError: %+v", name, op)
				}
				branch := block.Operations[index+1]
				if branch.Kind != OpCondBranch || len(branch.Successors[0].Arguments) != 1 || branch.Successors[0].Arguments[0] != op.Results[1].ID {
					t.Fatalf("%s: AllocationError does not reach the failure edge: %+v", name, branch)
				}
				failure := blocks[branch.Successors[0].Block]
				if len(failure.Parameters) != 1 || failure.Parameters[0].Type != op.Results[1].Type {
					t.Fatalf("%s: failure block does not receive the AllocationError: %+v", name, failure.Parameters)
				}
				if !mapsToAllocationFailed(module, failure) {
					t.Fatalf("%s: failure edge does not map to Err(CollectionError.AllocationFailed): %+v", name, failure.Operations)
				}
			}
		}
		if growths == 0 {
			t.Fatalf("%s lowered no growth", name)
		}
	}

	for _, item := range []string{
		"fn F() Result[uint, AllocationError] {\n    let mut values: int[]\n    try values.Append(4)\n    return Ok(values.Len)\n}\n",
		"fn F() Result[uint, AllocationError] {\n    let text := \"a\"\n    let runes := try text.ToRuneArray()\n    return Ok(runes.Len)\n}\n",
	} {
		parsed := parser.New(lexer.NewWithFile("module main\n\n"+item+"\nfn main() int {\n    return 0\n}\n", "sample.sec")).Parse()
		a := sema.NewAnalyzer()
		a.SetAllocationProfile("hosted")
		errs := a.Analyze(parsed.Program)
		if parsed.HasErrors || len(errs) != 1 || !strings.Contains(errs[0].Message, "propagates CollectionError") || !strings.Contains(errs[0].Message, "map CollectionError to AllocationError") {
			t.Errorf("CollectionError was accepted as AllocationError: %v\n%s", errs, item)
		}
	}
}

// enumIdentity reports whether typeID is the enum with the given identity.
func enumIdentity(module *Module, typeID TypeID, identity string) bool {
	typ, ok := module.Types.Lookup(typeID)
	return ok && typ.Kind == TypeEnum && typ.Identity == identity
}

// mapsToAllocationFailed reports whether the failure block builds
// CollectionError.AllocationFailed and wraps it in Err.
func mapsToAllocationFailed(module *Module, block *Block) bool {
	var reason ValueID
	found := false
	for _, op := range block.Operations {
		if op.Kind == OpEnumConstant && len(op.Results) == 1 && enumIdentity(module, op.Results[0].Type, "core::CollectionError") {
			definition, ok := enumDefinition(module, op.Results[0].Type)
			if !ok || int(op.EnumCase) >= len(definition.Cases) || definition.Cases[op.EnumCase].Name != "AllocationFailed" {
				return false
			}
			reason, found = op.Results[0].ID, true
		}
		if op.Kind == OpResultErr && found && len(op.Operands) == 1 && op.Operands[0] == reason {
			return true
		}
	}
	return false
}
