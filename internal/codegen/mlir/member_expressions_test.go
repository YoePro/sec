package mlir

import (
	"os"
	"strings"
	"testing"
)

// Canonical array Len must lower on every supported native integer width;
// the migration spelling remains the same operation, not a distinct member.
// Rules: rules/compiler/compiler_known_members.md — "Legacy lowercase member spellings";
// rules/collections/collections.md — §5.7 "Fixed-array public surface".
func TestFixedArrayCanonicalAndMigrationLengthLowerIdentically(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/codegen/mlir_target_integers/aggregates.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := "module main\n" + string(data) + "\nfn main() int { return 0 }\n"
	if !strings.Contains(source, "value.Len") {
		t.Fatal("fixture no longer covers canonical array Len")
	}
	for _, triple := range []string{"armv7-unknown-linux-gnueabihf", "x86_64-pc-linux-gnu"} {
		canonical, err := GenerateWithTriple(parseTestProgram(t, source), triple)
		if err != nil {
			t.Fatal(triple, err)
		}
		legacy, err := GenerateWithTriple(parseTestProgram(t, strings.ReplaceAll(source, "value.Len", "value.len")), triple)
		if err != nil || canonical != legacy {
			t.Fatalf("%s: canonical/migration lowering differs: %v", triple, err)
		}
	}
}
