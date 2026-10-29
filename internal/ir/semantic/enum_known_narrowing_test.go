package semantic

import (
	"os"
	"testing"
)

// TestKnownEnumNarrowingRetainsConstantProof prevents a known fitting member
// of a wider enum from losing Sema's constant proof at the IR boundary.
// Rules: rules/types/types.md — "Exact integer conversion proof";
// rules/declarations/enums.md — §12.1; rules/compiler/semantic_ir.md — §124.
func TestKnownEnumNarrowingRetainsConstantProof(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/build/native/enum_known_narrowing.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(data), 11)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, operation := range block.Operations {
				if operation.Kind != OpEnumToInteger {
					continue
				}
				proof := operation.ConversionProof
				if proof == nil || proof.Basis != "constant" || proof.PlanPointerWidthBits != 0 {
					t.Fatalf("%s lost selected constant proof: %+v", fn.Name, proof)
				}
				seen[fn.Name] = true
			}
		}
	}
	if !seen["KnownZero"] || !seen["KnownHigh"] {
		t.Fatalf("missing enum conversions: %v", seen)
	}
}
