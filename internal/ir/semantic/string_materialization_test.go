package semantic

import (
	"os"
	"strings"
	"testing"

	"sec/internal/sema"
)

// Runtime concat keeps one maximal operation, explicit allocation capability,
// source-selected failure policy and checked Result routing, without claiming
// physical string allocation or transactional append lowering.
// Rules: rules/compiler/semantic_ir.md — §17(6); MD-039 §§2–5.
func TestMD039RuntimeStringMaterialization(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/md039/runtime_concat.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 19)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("%v\n%s", err, Format(module))
	}
	found := map[string]int{}
	for _, fn := range module.Functions {
		calls := 0
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpDirectCall {
					calls++
				}
				if op.Kind != OpStringMaterialize {
					continue
				}
				found[fn.Name]++
				fact := op.StringMaterialization
				expected := sema.StringMaterializationPanic
				if fn.Name == "Recoverable" || fn.Name == "Handled" {
					expected = sema.StringMaterializationTry
				}
				if fact.Failure != expected || op.AllocationContext == nil || !fn.RequiresAllocationContext {
					t.Fatalf("lost source mode or context: %+v", op)
				}
				if fn.Name == "Ordered" && calls != 2 {
					t.Fatal("operands not evaluated once before materialization", calls)
				}
				// Mutating the explicit failure policy must not silently turn
				// a Result operation into a panicking operation or vice versa.
				old := fact.Failure
				fact.Failure = "invalid"
				if Verify(module) == nil {
					t.Fatal("invalid mode accepted")
				}
				fact.Failure = old
				context := op.AllocationContext.Value
				op.AllocationContext.Value = 999999
				if Verify(module) == nil {
					t.Fatal("missing capability accepted")
				}
				op.AllocationContext.Value = context
				oldReason := fact.PanicReasonID
				fact.PanicReasonID = 65535
				if Verify(module) == nil {
					t.Fatal("invented panic reason accepted")
				}
				fact.PanicReasonID = oldReason
				if expected == sema.StringMaterializationPanic {
					fn.NoPanic, fn.NoPanicSource = true, "written"
					if Verify(module) == nil {
						t.Fatal("ordinary materialization accepted in noPanic IR")
					}
					fn.NoPanic, fn.NoPanicSource = false, ""
				}
			}
		}
	}
	for _, name := range []string{"Ordinary", "Recoverable", "Handled", "Ordered", "EmptyPrefix"} {
		if found[name] != 1 {
			t.Fatalf("%s materializations = %d", name, found[name])
		}
	}
	if !strings.Contains(Format(module), "mode=try") || !strings.Contains(Format(module), "mode=panic") {
		t.Fatal("failure mode lost from text")
	}
}
