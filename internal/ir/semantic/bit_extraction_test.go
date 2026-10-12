package semantic

import (
	"strings"
	"testing"
)

// `bit[N](value)` lowers to one int.bit-extract with the unsigned carrier Sema
// selected; it has no failure edge, and the verifier checks the width domain
// and the carrier (MD-046/MD-056).
//
// Rules:
//   - rules/types/types.md — "Explicit low-bit extraction"
func TestBitExtractionLowersToIntBitExtract(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn Low(word: uint32, negative: int64, b: byte) uint64 {
    let low12 := bit[12](word)
    let low8 := bit[8](negative)
    let fromByte := bit[3](b)
    return bit[64](negative)
}
`, 14)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{"int.bit-extract bits=12 %0 : !5", "int.bit-extract bits=8 %1 : !6", "int.bit-extract bits=64 %1 : !1"} {
		if !strings.Contains(text, want) {
			t.Errorf("module lacks %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"fail.", "conditional-branch"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("bit extraction emitted %q:\n%s", forbidden, text)
		}
	}
	low := functionNamed(t, module, "Low")
	for _, block := range low.Blocks {
		for index := range block.Operations {
			if block.Operations[index].Kind == OpIntBitExtract && block.Operations[index].Integer.Int64() == 12 {
				block.Operations[index].Integer.SetInt64(40)
			}
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "int.bit-extract") {
		t.Fatalf("verify accepted an extraction wider than its operand: %v", err)
	}
}
