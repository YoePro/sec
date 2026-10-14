package secmlir

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sec/internal/testsupport"
	"strings"
	"testing"
)

// Canonical error values lower as the same enum on both native uint widths.
// Rules: rules/collections/collections.md — §16;
// rules/compiler/semantic_ir.md — enum identity;
// rules/types/types.md — "int and uint".
func TestEmitCanonicalCollectionErrorTargetWidths(t *testing.T) {
	module := booleanFixtureModule(t, "collection_errors")
	for _, width := range []uint16{32, 64} {
		output, err := Emit(module, testPlan(width))
		if err != nil {
			t.Fatal(width, err)
		}
		if !strings.Contains(string(output), "CollectionError") {
			t.Fatalf("%d: enum identity lost", width)
		}
	}
}

// Real dialect verification preserves the enum schema through current scalar
// passes. Enum-to-native representation lowering is a separate obligation.
// Rules: rules/collections/collections.md — §16;
// rules/compiler/semantic_ir.md — enum identity and case values.
func TestCanonicalCollectionErrorVerifiesWithRealDialect(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	module := booleanFixtureModule(t, "collection_errors")
	for _, width := range []uint16{32, 64} {
		output, err := Emit(module, testPlan(width))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "errors.mlir")
		if err := os.WriteFile(path, output, 0600); err != nil {
			t.Fatal(err)
		}
		data, err := exec.Command(tool, path, "--sec-lower-scalar-core", "--sec-lower-trivial-core", "--verify-each").CombinedOutput()
		if err != nil {
			t.Fatalf("%d: %v\n%s", width, err, data)
		}
		text := string(data)
		for _, name := range []string{"core::CollectionError", "AllocationFailed", "CapacityExceeded", "SizeOverflow", fmt.Sprintf("underlying = ui%d", width)} {
			if !strings.Contains(text, name) {
				t.Fatalf("%d: lost %s\n%s", width, name, text)
			}
		}
	}
}
