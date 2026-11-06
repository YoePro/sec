package semantic

import (
	"os"
	"strings"
	"testing"
)

// A raw-to-safe property retains the explicit capability boundary; it must not
// become an ordinary property load or manufacture a safe reference operation.
// Rules: rules/memory/raw_pointers.md §32(7);
// rules/corrections/applied/md070-as-property-naming-correction-20261106.md §3.
func TestRawPointerReferencePropertyBoundary(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/raw_pointer_reference_property.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(data), 27)
	if module != nil || err == nil || !strings.Contains(err.Error(), "raw-to-safe reference conversion RawPtr.AsRef") {
		t.Fatalf("boundary: module=%v err=%v", module, err)
	}
}
