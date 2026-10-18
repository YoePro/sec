package sema

import (
	"os"
	"strings"
	"testing"
)

// Both spellings have identical ownership, construction and lifetime behavior,
// including rejection of moved sources, escaping locals and indexed moves.
// Rules: rules/declarations/functions.md — §14;
// rules/memory/copy_move.md — §9; rules/memory/ownership.md — §17.
func TestExplicitReturnMatchesImplicitOwnership(t *testing.T) {
	for _, fixture := range []string{"explicit_return_valid.sec", "explicit_return_invalid.sec"} {
		source, err := os.ReadFile("../../testdata/ownership/" + fixture)
		if err != nil {
			t.Fatal(err)
		}
		marked := analyzeSourceRaw(t, string(source))
		plain := analyzeSourceRaw(t, strings.ReplaceAll(string(source), "return <-", "return "))
		if strings.HasSuffix(fixture, "_valid.sec") && len(marked) != 0 {
			t.Fatalf("%s: %v", fixture, marked)
		}
		if strings.HasSuffix(fixture, "_invalid.sec") && len(marked) < 3 {
			t.Fatalf("expected moved, escape and indexed-move diagnostics: %v", marked)
		}
		if len(marked) != len(plain) {
			t.Fatalf("%s: marked %v; plain %v", fixture, marked, plain)
		}
		for i := range marked {
			if marked[i].Message != plain[i].Message {
				t.Fatalf("%s: marked %v; plain %v", fixture, marked, plain)
			}
		}
	}
}
