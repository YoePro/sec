package semantic

import (
	"os"
	"testing"
)

// Unnamed open values retain the full open-enum proof through IR construction;
// bit[8] to int16 is unsigned widening even though the target is signed.
// Rules: rules/declarations/enums.md — §§10,12.1;
// rules/compiler/semantic_ir.md — §124.
func TestOpenEnumDomainProofReachesIR(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/enum_open_domain.sec")
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
			for _, op := range block.Operations {
				if op.Kind != OpEnumToInteger {
					continue
				}
				width := uint16(8)
				if fn.Name == "Word" {
					width = 16
				}
				p := op.ConversionProof
				source, sourceOK := module.Types.Lookup(fn.Parameters[0].Value.Type)
				carrier, carrierOK := module.Types.Lookup(source.Underlying)
				if p == nil || p.Basis != "open-enum-domain" || p.PlanPointerWidthBits != 0 || !sourceOK || !carrierOK || carrier.Signed || carrier.BitWidth != width {
					t.Fatalf("%s lost open unsigned domain: proof=%+v source=%+v carrier=%+v", fn.Name, p, source, carrier)
				}
				seen[fn.Name] = true
			}
		}
	}
	for _, name := range []string{"Byte", "SignedWide", "Word"} {
		if !seen[name] {
			t.Fatalf("missing conversion: %s", name)
		}
	}
}
