package semantic

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Returned slice expressions preserve the callee's instantiated origin,
// evaluate once before endpoints and retain it for range/length/index uses.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,43–49,64–67;
// rules/compiler/semantic_ir.md — §126.8,14–19,22.
func TestPackage16ReturnedSliceSources(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_returned_expression_sources.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			module, err := buildPackage16Source(t, string(source), width)
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(module); err != nil {
				t.Fatal(err)
			}

			repeated, err := buildPackage16Source(t, string(source), width)
			if err != nil || Format(module) != Format(repeated) {
				t.Fatalf("unstable call-source facts: %v", err)
			}
			for _, name := range []string{"Shared", "Mutable", "Reduced", "Observed", "Read"} {
				fn := functionNamed(t, module, name)
				definitions := map[ValueID]Operation{}
				used := map[ValueID]int{}
				var order []string
				sourceCalls := 0
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						for _, result := range op.Results {
							definitions[result.ID] = op
						}
						if op.Kind == OpDirectCall {
							order = append(order, string(op.Callee))
							if len(op.Results) > 0 && op.Reference != nil {
								sourceCalls++
							}
						}
						if op.Kind == OpArrayExtract || op.Kind == OpStorageLoad {
							t.Fatalf("%s copied backing storage", name)
						}
						if len(op.Operands) == 0 {
							continue
						}
						parent, ok := definitions[op.Operands[0]]
						if !ok || parent.Kind != OpDirectCall || parent.Reference == nil {
							continue
						}
						switch op.Kind {
						case OpSliceReborrowShared, OpSliceReborrowMutable:
							if op.Reference.SourceBorrow != parent.Reference.Borrow || op.Reference.Borrow == parent.Reference.Borrow {
								t.Fatalf("%s lost returned borrow identity", name)
							}
							if name == "Mutable" && op.Kind != OpSliceReborrowMutable || name != "Mutable" && op.Kind != OpSliceReborrowShared {
								t.Fatalf("%s lost source authority", name)
							}
							used[op.Operands[0]]++
							order = append(order, "reborrow")
						case OpSliceLength:
							if name == "Observed" {
								used[op.Operands[0]]++
							}
							order = append(order, "length")
						case OpSliceElementPlace:
							if op.Reference.Borrow != parent.Reference.Borrow {
								t.Fatalf("%s changed returned element provenance", name)
							}
							used[op.Operands[0]]++
							order = append(order, "element")
						}
					}
				}
				want := 1
				if name == "Observed" {
					want = 2
				}
				if sourceCalls != want || len(used) != want {
					t.Fatalf("%s source calls=%d observed sources=%v, want %d", name, sourceCalls, used, want)
				}
				for value, count := range used {
					if count != 1 {
						t.Fatalf("%s reused observation %%%d %d times", name, value, count)
					}
				}
				if name == "Shared" && (len(order) != 5 || !strings.Contains(order[0], "::Same(") || !strings.Contains(order[1], "::Start(") || !strings.Contains(order[2], "::End(") || order[3] != "length" || order[4] != "reborrow") {
					t.Fatalf("source/endpoint/observation order: %v", order)
				}
				if name == "Read" && (len(order) != 3 || !strings.Contains(order[0], "::Same(") || !strings.Contains(order[1], "::Index(") || order[2] != "element") {
					t.Fatalf("source/index/element order: %v", order)
				}
			}
		})
	}
}

// Source authority, reservation and escape errors remain Sema errors; a
// narrowed returned origin retains its named MD-115 gate and no partial IR.
// Rules: rules/memory/borrowing.md — §§4,15,22;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§14–19,43,66–67;
// rules/corrections/applied/md084-schema11-reference-metadata-correction-20261021.md — §4(6).
func TestPackage16ReturnedSourceRestrictions(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/sema/slice_returned_sources_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		module, err := buildPackage16Source(t, string(source), width)
		if module != nil || err == nil {
			t.Fatal("invalid source produced IR")
		}
		for _, want := range []string{"shared reference never grants mutation authority", "shared borrowed", "descending slice range", "cannot return reference to local"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("missing source rejection %q: %v", want, err)
			}
		}
		narrow, err := os.ReadFile("../../../testdata/semantic_ir/package16_unsupported/returned_call_subslice.sec")
		if err != nil {
			t.Fatal(err)
		}
		module, err = buildPackage16Source(t, string(narrow), width)
		var unsupported *UnsupportedFeatureError
		if module != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "MD-115") {
			t.Fatalf("lost sub-slice capability gate: %v", err)
		}
	}
}
