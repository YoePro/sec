package semantic

import (
	"os"
	"strings"
	"testing"
)

// The optional terminal marker adds no Semantic IR operation, extra move,
// evaluation or failure path. Compare location-normalized modules for the
// native track and the represented aggregate, Result and reference packages.
// Rules: rules/declarations/functions.md — §14;
// rules/compiler/semantic_ir.md — §§36,46.
func TestExplicitReturnHasIdenticalSemanticIR(t *testing.T) {
	for _, fixture := range []struct {
		path string
		pkg  uint8
	}{
		{"../../../testdata/build/native/explicit_return.sec", 14},
		{"../../../testdata/ir/explicit_return.sec", 15},
	} {
		source, err := os.ReadFile(fixture.path)
		if err != nil {
			t.Fatal(err)
		}
		marked, err := analyzedModule(t, string(source), fixture.pkg)
		if err != nil {
			t.Fatal(err)
		}
		// Replace the marker with spaces to preserve every source location.
		plain, err := analyzedModule(t, strings.ReplaceAll(string(source), "return <-", "return   "), fixture.pkg)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(marked); err != nil {
			t.Fatal(err)
		}
		if got, want := Format(marked), Format(plain); got != want {
			t.Fatalf("%s: explicit marker changed semantic operations:\n%s\nplain:\n%s", fixture.path, got, want)
		}
	}
}
