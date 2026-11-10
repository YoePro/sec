package secmlir

import (
	"os"
	"strings"
	"testing"
)

// TestNamedIntegerStorageProvenance preserves nominal storage types in emitted
// semantic MLIR while attaching carrier metadata for physical normalization.
// Rules: rules/compiler/semantic_ir.md — §7(3–5); rules/types/types.md — §22.1.
func TestNamedIntegerStorageProvenance(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/build/native/named_integer_storage.sec")
	if err != nil {
		t.Fatal(err)
	}
	module := buildPackage15EmitModule(t, "main", string(data))
	for _, width := range []uint16{32, 64} {
		output, err := Emit(module, testPlan(width))
		if err != nil {
			t.Fatal(err)
		}
		text := string(output)
		for _, kind := range []string{"int", "uint8", "uint256"} {
			if !strings.Contains(text, `sec.scalar_kind = "`+kind+`"`) {
				t.Fatal("missing carrier metadata", kind, text)
			}
		}
		if !strings.Contains(text, `!sec.storage<!sec.named<"main::Distance", !sec.named<"main::Meters", !sec.int>>>`) {
			t.Fatal("nominal storage identity was lost before physical lowering", text)
		}
	}
}
