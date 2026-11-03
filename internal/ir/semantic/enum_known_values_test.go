package semantic

import (
	"os"
	"testing"
)

// Known enum numeric evidence remains available after Sema restores lexical
// scopes; IR consumes constant proofs for constructors and immutable aliases.
// Rules: rules/declarations/enums.md — §12.1;
// rules/compiler/semantic_ir.md — §124 (resolved exact-conversion proofs).
func TestKnownEnumValueProofsReachIR(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/enum_known_values.sec")
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
	conversions := 0
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind != OpEnumToInteger {
					continue
				}
				conversions++
				if op.ConversionProof == nil || op.ConversionProof.Basis != "constant" {
					t.Fatalf("%s lost constant proof", fn.Name)
				}
			}
		}
	}
	if conversions != 4 {
		t.Fatalf("conversions: %d", conversions)
	}
}
