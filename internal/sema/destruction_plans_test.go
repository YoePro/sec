package sema

import (
	"math/big"
	"reflect"
	"strings"
	"testing"
)

const destructionPlanSource = `module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

type Pair struct {
    first: Handle,
    count: int,
    second: Handle,
}

type Wrapper struct {
    pair: Pair,
}

impl Wrapper {
    free {
    }
}

type Choice union {
    Empty,
    One(Handle),
}

type Plain struct {
    a: int,
    b: bool,
}

type Owner struct {
    items: Handle[],
}
`

func destructionTestTypes(t *testing.T) *Analyzer {
	t.Helper()
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, destructionPlanSource)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %v", errors)
	}
	return analyzer
}

// P17-10/11/16: struct plans run custom free first and destroy remaining
// non-trivial fields in reverse declaration order; nested plans are shared by
// deterministic ID; trivial types have trivial plans.
//
// Rules:
//   - rules/memory/destruction.md — §§ 7.2, 15.4, 15.5
func TestDestructionPlansStructOrderAndCustomFree(t *testing.T) {
	analyzer := destructionTestTypes(t)
	pair := analyzer.ResolvedDestructionPlanOf(analyzer.types["Pair"])
	root, ok := pair.Plan(pair.Root)
	if !ok || root.Kind != DestructionStruct || root.CustomFree {
		t.Fatalf("Pair plan: %+v", root)
	}
	if len(root.Fields) != 2 || root.Fields[0].Field != "second" || root.Fields[1].Field != "first" || root.Fields[0].Index != 2 {
		t.Fatalf("Pair fields must be second, first (reverse declaration, trivial count skipped): %+v", root.Fields)
	}
	if root.Fields[0].Plan != root.Fields[1].Plan {
		t.Fatal("equal field types must share one plan ID")
	}
	handle, _ := pair.Plan(root.Fields[0].Plan)
	if handle.Kind != DestructionStruct || !handle.CustomFree || len(handle.Fields) != 0 {
		t.Fatalf("Handle plan: %+v", handle)
	}
	wrapper := analyzer.ResolvedDestructionPlanOf(analyzer.types["Wrapper"])
	if plan, _ := wrapper.Plan(wrapper.Root); !plan.CustomFree || len(plan.Fields) != 1 || plan.Fields[0].Field != "pair" {
		t.Fatalf("Wrapper plan: %+v", plan)
	}
	plain := analyzer.ResolvedDestructionPlanOf(analyzer.types["Plain"])
	if plan, _ := plain.Plan(plain.Root); plan.Kind != DestructionTrivial || len(plain.Plans) != 1 {
		t.Fatalf("Plain plan: %+v", plain)
	}
	if !strings.HasPrefix(string(pair.Root), "destroy:") {
		t.Fatalf("plan ID %q is not derived from the canonical identity", pair.Root)
	}
}

// Fixed arrays are one compact element plan plus an exact count, never one
// record per element; variants destroy only their payload; owning dynamic
// arrays are a named P18 boundary, not trivial.
//
// Rules:
//   - rules/memory/destruction.md — §§ 7.3, 7.4, 10
func TestDestructionPlansArraysVariantsAndBoundaries(t *testing.T) {
	analyzer := destructionTestTypes(t)
	array := NewFixedArrayType(analyzer.types["Handle"], big.NewInt(1000000))
	set := analyzer.ResolvedDestructionPlanOf(array)
	plan, _ := set.Plan(set.Root)
	if plan.Kind != DestructionFixedArray || plan.ElementCount != "1000000" || len(set.Plans) != 2 {
		t.Fatalf("fixed array plan: %+v (%d plans)", plan, len(set.Plans))
	}
	choice := analyzer.ResolvedDestructionPlanOf(analyzer.types["Choice"])
	plan, _ = choice.Plan(choice.Root)
	if plan.Kind != DestructionVariant || plan.VariantFamily != "union" || len(plan.Variants) != 1 || plan.Variants[0].Variant != "One" || plan.Variants[0].Index != 1 {
		t.Fatalf("Choice plan: %+v", plan)
	}
	result := Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{analyzer.types["Handle"], {Name: "int", Kind: IntType}}}
	set = analyzer.ResolvedDestructionPlanOf(result)
	plan, _ = set.Plan(set.Root)
	if plan.Kind != DestructionVariant || plan.VariantFamily != "result" || len(plan.Variants) != 1 || plan.Variants[0].Variant != "Ok" {
		t.Fatalf("Result plan: %+v", plan)
	}
	owner := analyzer.ResolvedDestructionPlanOf(analyzer.types["Owner"])
	plan, _ = owner.Plan(owner.Root)
	if plan.Kind != DestructionStruct || len(plan.Fields) != 1 {
		t.Fatalf("Owner plan: %+v", plan)
	}
	items, _ := owner.Plan(plan.Fields[0].Plan)
	if items.Kind != DestructionUnsupported || !strings.Contains(items.Unsupported, "P18") {
		t.Fatalf("dynamic array plan must be the P18 boundary: %+v", items)
	}
	arena := ResolveDestructionPlans(Type{Name: "Arena", Kind: StructType, Intrinsic: true})
	if plan, _ := arena.Plan(arena.Root); plan.Kind != DestructionResource || plan.Resource != "Arena" {
		t.Fatalf("Arena plan: %+v", plan)
	}
}

// P17-14: repeated queries are equal and detached; mutating a returned plan
// never changes another query's result.
func TestDestructionPlanQueriesAreDetached(t *testing.T) {
	analyzer := destructionTestTypes(t)
	first := analyzer.ResolvedDestructionPlanOf(analyzer.types["Pair"])
	second := analyzer.ResolvedDestructionPlanOf(analyzer.types["Pair"])
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first.IDs(), second.IDs()) {
		t.Fatal("repeated destruction plan queries differ")
	}
	plan, _ := first.Plan(first.Root)
	plan.Fields[0].Field = "mutated"
	again, _ := first.Plan(first.Root)
	if again.Fields[0].Field != "second" {
		t.Fatal("a returned plan aliases the plan set")
	}
}

// P17-08: copy classification maps to transfer actions; MoveOnly (custom
// free, ref mut) and @noCopy need an explicit move, which is always legal for
// a concrete type; a conditional classification is never a trivial copy.
//
// Rules:
//   - rules/memory/copy_move.md — §§ 3.2–3.5
func TestTransferActionsFollowCopyClassification(t *testing.T) {
	analyzer := destructionTestTypes(t)
	integer := Type{Name: "int", Kind: IntType}
	mutableRef := Type{Kind: ReferenceType, ReferenceMutable: true, Element: &integer}
	sharedRef := Type{Kind: ReferenceType, Element: &integer}
	for name, test := range map[string]struct {
		typ      Type
		explicit bool
		want     TransferAction
	}{
		"plain copy":           {analyzer.types["Plain"], false, TransferCopyTrivial},
		"custom free copy":     {analyzer.types["Handle"], false, TransferRejected},
		"custom free move":     {analyzer.types["Handle"], true, TransferMove},
		"containing aggregate": {analyzer.types["Pair"], false, TransferRejected},
		"shared ref copy":      {sharedRef, false, TransferCopyTrivial},
		"mutable ref copy":     {mutableRef, false, TransferRejected},
		"mutable ref move":     {mutableRef, true, TransferMove},
		"generic copy":         {Type{Name: "T", Kind: GenericType}, false, TransferRejected},
		"generic move":         {Type{Name: "T", Kind: GenericType}, true, TransferRejected},
		"trivial move":         {integer, true, TransferMove},
	} {
		action, reason := ResolveTransferAction(test.typ, test.explicit)
		if action != test.want {
			t.Errorf("%s: %s (%s), want %s", name, action, reason, test.want)
		}
		if action == TransferRejected && reason == "" {
			t.Errorf("%s: rejection without reason", name)
		}
	}
}
