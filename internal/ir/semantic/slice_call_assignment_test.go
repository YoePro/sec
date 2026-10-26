package semantic

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// Returned mutable slice elements are writable using their canonical referent
// authority; the source/index/RHS expressions evaluate once in that order.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§55,58–62,64–67;
// rules/memory/borrowing.md — §§12,14.4–14.5,15.
func TestPackage16CallSliceAssignment(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_call_index_assignment.sec")
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
		repeated, err := buildPackage16Source(t, string(source), width)
		if err != nil || Format(module) != Format(repeated) {
			t.Fatalf("unstable assignment facts: %v", err)
		}
		for _, name := range []string{"Write", "Nested", "Local", "Disjoint"} {
			fn := functionNamed(t, module, name)
			definitions := map[ValueID]Operation{}
			var order []string
			writes := 0
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					for _, result := range op.Results {
						definitions[result.ID] = op
					}
					if op.Kind == OpDirectCall {
						for _, callee := range []string{"Writable", "Index", "Value"} {
							if strings.Contains(string(op.Callee), "::"+callee+"(") {
								order = append(order, callee)
							}
						}
					}
					if op.Kind == OpSliceElementPlace {
						if source := definitions[op.Operands[0]]; source.Kind == OpDirectCall {
							if source.Reference == nil || op.Reference == nil || source.Reference.Borrow != op.Reference.Borrow {
								t.Fatalf("%s lost granting call borrow", name)
							}
							order = append(order, "element")
						}
					}
					if op.Kind == OpPlaceWrite {
						writes++
						place := definitions[op.Operands[0]]
						if place.Kind != OpSliceElementPlace || definitions[place.Operands[0]].Kind != OpDirectCall {
							t.Fatalf("%s does not write the returned slice element", name)
						}
						order = append(order, "write")
					}
					if op.Kind == OpArrayExtract {
						t.Fatalf("%s copied an array element", name)
					}
				}
			}
			want := []string{"Writable", "Index", "element", "Value", "write"}
			if name == "Nested" || name == "Local" {
				want = append([]string{"Writable"}, want...)
			}
			if writes != 1 || !reflect.DeepEqual(order, want) {
				t.Fatalf("%s evaluation order: %v, want %v", name, order, want)
			}
		}
	}
}
