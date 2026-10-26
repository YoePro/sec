package semantic

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// Trivial integer compound slice writes use one guarded Place and checked
// arithmetic before committing a replacement.
// Rules: rules/foundations/operators.md — "Compound assignment", "Compound arithmetic failure";
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58.
func TestPackage16SliceCompoundAssignment(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_compound_assignment.sec")
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
			t.Fatalf("unstable compound facts: %v", err)
		}
		for name, want := range map[string]int{"Add": 1, "Operations": 11, "Wide": 2} {
			fn := functionNamed(t, module, name)
			definitions := map[ValueID]Operation{}
			reads, writes, places, checked, failures := 0, 0, 0, 0, 0
			var order []string
			for _, block := range fn.Blocks {
				for index, op := range block.Operations {
					for _, result := range op.Results {
						definitions[result.ID] = op
					}
					switch op.Kind {
					case OpDirectCall:
						for _, callee := range []string{"Writable", "Index", "Value"} {
							if strings.Contains(string(op.Callee), "::"+callee+"(") {
								order = append(order, callee)
							}
						}
					case OpSliceElementPlace:
						places++
						order = append(order, "place")
					case OpPlaceRead:
						reads++
						order = append(order, "read")
					case OpIntBinaryChecked, OpIntShiftChecked:
						checked++
						order = append(order, "operator")
						// A failed operator branches before any destination commit.
						if index+1 >= len(block.Operations) || block.Operations[index+1].Kind != OpCondBranch {
							t.Fatal("checked operator has no failure edge")
						}
						for _, later := range block.Operations[index+1:] {
							if later.Kind == OpPlaceWrite {
								t.Fatal("write before arithmetic success")
							}
						}
					case OpArithmeticFailure:
						failures++
						for _, other := range block.Operations {
							if other.Kind == OpPlaceWrite {
								t.Fatal("failure block writes the destination")
							}
						}
					case OpPlaceWrite:
						writes++
						order = append(order, "write")
						place := definitions[op.Operands[0]]
						if place.Kind != OpSliceElementPlace {
							t.Fatal("compound write recomputed its destination")
						}
						value := definitions[op.Operands[1]]
						if len(value.Operands) < 1 || definitions[value.Operands[0]].Kind != OpPlaceRead || definitions[value.Operands[0]].Operands[0] != op.Operands[0] {
							t.Fatal("compound write did not read the same Place")
						}
					case OpArrayExtract:
						t.Fatal("compound write copied backing storage")
					}
				}
			}
			wantChecked := map[string]int{"Add": 1, "Operations": 8, "Wide": 2}[name]
			if reads != want || writes != want || places != want || checked != failures || checked != wantChecked {
				t.Fatalf("%s places=%d reads=%d writes=%d checked=%d failures=%d", name, places, reads, writes, checked, failures)
			}
			if name == "Add" && !reflect.DeepEqual(order, []string{"Writable", "Index", "place", "read", "Value", "operator", "write"}) {
				t.Fatalf("compound order: %v", order)
			}
		}
	}
}

// Compound writes keep authority, operand-type and live-child restrictions at
// Sema; invalid programs produce no partial Semantic IR.
// Rules: rules/foundations/operators.md — "Compound assignment target";
// rules/memory/borrowing.md — §§8.1,14.4–14.5,15.
func TestPackage16SliceCompoundRestrictions(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/sema/slice_compound_assignment_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		module, err := buildPackage16Source(t, string(source), width)
		if module != nil || err == nil {
			t.Fatal("invalid compound writes produced IR")
		}
		for _, want := range []string{"immutable index target", "bool", "shared borrowed"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("missing %q: %v", want, err)
			}
		}
	}
}
