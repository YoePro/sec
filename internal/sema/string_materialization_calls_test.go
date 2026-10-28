package sema

import (
	"os"
	"strings"
	"testing"
)

// Public materializers borrow named arrays and slices, preserve caller use,
// validate element/arity contracts and reject consuming argument markers.
// Rules: rules/corrections/md109-core-string-materialization-correction.md
// — §§1.2–1.4,2.1–2.6,10.2; rules/memory/borrowing.md — §15.2.
func TestStringMaterializationStaticCalls(t *testing.T) {
	valid, err := os.ReadFile("../../testdata/sema/string_materialization_static_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(valid)); len(errors) != 0 {
		t.Fatal(errors)
	}
	invalid, err := os.ReadFile("../../testdata/sema/string_materialization_static_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(invalid))
	want := []string{"requires a byte array or slice", "requires a rune array or slice", "expects 1 argument", "Cannot move 'data' into borrowed parameter 'data'.", "Cannot move 'data' into borrowed parameter 'data'."}
	if len(errors) != len(want) {
		t.Fatalf("got %v, want %d diagnostics", errors, len(want))
	}
	for index, diagnostic := range errors {
		if !strings.Contains(diagnostic.Message, want[index]) {
			t.Fatalf("diagnostic %d: %v, want %s", index, diagnostic, want[index])
		}
	}
}
