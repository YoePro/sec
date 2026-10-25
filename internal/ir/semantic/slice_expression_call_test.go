package semantic

import (
	"os"
	"strings"
	"testing"
)

// Expression arguments grant a distinct shared pass without evaluating a
// slice source, endpoint or nested call twice. Returned dependencies survive.
// Rules: rules/memory/borrowing.md — §15.3(2), §15.4(1)–(5);
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,64–67.
func TestPackage16SliceExpressionCallReborrow(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_expression_call_reborrow.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		module, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		again, err := buildPackage16Source(t, string(source), width)
		if err != nil || Format(module) != Format(again) {
			t.Fatalf("repeat build changed slice call facts: %v", err)
		}
		for _, name := range []string{"Fresh", "Nested", "Returned"} {
			fn := functionNamed(t, module, name)
			shared := 0
			calls := map[FunctionID]int{}
			var order []string
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					if op.Kind == OpDirectCall {
						calls[op.Callee]++
						order = append(order, string(op.Callee))
					}
					if op.Kind == OpSliceCopyShared || op.Kind == OpSliceMoveMutable {
						t.Fatalf("%s copied or moved its mutable argument", name)
					}
					if op.Kind == OpSliceReborrowShared {
						shared++
						if op.Reference.Borrow == op.Reference.SourceBorrow || op.Reference.SourceBorrow == 0 {
							t.Fatalf("%s reused its parent borrow: %+v", name, op.Reference)
						}
					}
				}
			}
			if shared != 1 {
				t.Fatalf("%s/%d has %d shared reborrows, want 1", name, width, shared)
			}
			for callee, count := range calls {
				if count != 1 {
					t.Fatalf("%s evaluated %s %d times", name, callee, count)
				}
			}
			wantCalls := 2
			if name == "Fresh" {
				wantCalls = 3
			}
			if len(calls) != wantCalls {
				t.Fatalf("%s has %d callees, want %d", name, len(calls), wantCalls)
			}

			if name == "Fresh" {
				if !strings.Contains(order[0], "::Start(") || !strings.Contains(order[1], "::End(") || !strings.Contains(order[2], "::Read(") {
					t.Fatalf("fresh argument evaluated out of order: %v", order)
				}
			} else if !strings.Contains(order[0], "::IdentityMutable(") {
				t.Fatalf("nested argument evaluated out of order: %v", order)
			}
		}
		selected := functionNamed(t, module, "SelectFresh")
		calls := 0
		for _, block := range selected.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpSliceReborrowShared {
					t.Fatal("shared reborrow displaced exact mutable overload")
				}
				if op.Kind == OpDirectCall {
					calls++
					if !strings.Contains(string(op.Callee), "::Select(ref mut int[])") {
						t.Fatalf("wrong selected overload: %s", op.Callee)
					}
				}
			}
		}
		if calls != 1 {
			t.Fatalf("SelectFresh has %d calls, want 1", calls)
		}
	}
}
