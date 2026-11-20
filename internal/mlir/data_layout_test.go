package mlir

import (
	"strings"
	"testing"

	"sec/internal/platform/target"
)

// MD-141 § 4(3): the native-track code generator applies exactly the
// maintained data layout for every triple it can compile, and reports a
// triple it has no backend for instead of guessing.
func TestTargetDataLayoutMatchesMaintainedFacts(t *testing.T) {
	toolchain := NewToolchain("")
	if _, err := toolchain.toolPath("llc"); err != nil {
		t.Skip("llc not found; the backend data layout is not checked in this run")
	}
	checked := 0
	for _, definition := range target.Definitions() {
		maintained, ok := definition.DataLayout()
		if !ok {
			continue
		}
		applied, err := toolchain.TargetDataLayout(definition.LLVMTriple)
		if err != nil {
			if !strings.Contains(err.Error(), "llc cannot compile for") {
				t.Errorf("%s: %v", definition.LLVMTriple, err)
			}
			continue
		}
		checked++
		if applied != maintained {
			t.Errorf("%s: llc applies %q, maintained %q", definition.LLVMTriple, applied, maintained)
		}
	}
	if checked == 0 {
		t.Fatal("llc compiled no registered triple")
	}
	t.Logf("llc data layout verified for %d registered triples", checked)
}
