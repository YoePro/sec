package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// A template parameter forwarded to another generic satisfies the
// constraint its own declaration guarantees, and a return-position call
// selects the non-generic overload when the generic candidate's constraint
// fails (sec/stdlib/str/join.sec Concat/Join).
//
// Rules:
//   - rules/declarations/generics.md — §14 "Constraint satisfaction", §29 "Overload resolution"
func TestForwardedConstraintsAndNonGenericOverloadFallback(t *testing.T) {
	source := genericIteratorPrelude + `
fn Join[I: Iterator[string]](values: I, separator: string) int {
    return 1
}

fn Join(values: ref string[], separator: string) int {
    return 2
}

fn Concat[I: Iterator[string]](values: I) int {
    return Join(<-values, "")
}

fn Concat(values: ref string[]) int {
    return Join(values, "")
}

fn Main() int {
    let words: string[] := ["a"]
    let total := Concat(ref words[..])
    discard total
    return Concat(Words { index: 0 })
}
`
	if errors := analyzeSourceRaw(t, source); len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
}

// A compiler-known lowercase type name in value position is reported as a
// type, not as an undefined variable.
func TestCompilerKnownTypeNameUsedAsValue(t *testing.T) {
	errors := analyzeSourceRaw(t, "module main\n\nfn F() void {\n    let reserved := list\n    discard reserved\n}\n")
	if len(errors) == 0 || errors[0].ID != diagnostics.TypeNameUsedAsValue || !strings.Contains(errors[0].Message, "list is a type, not a value") {
		t.Fatalf("errors = %+v", errors)
	}
}
