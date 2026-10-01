package sema

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
)

// Local variables and loop bindings that hide a visible type, function, or
// generic parameter report S1057 with the hidden declaration's location.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §2 "One declaration namespace per scope", §8 "Shadowing", §20 "Diagnostics"
func TestLocalShadowingOfVisibleDeclarations(t *testing.T) {
	input, err := os.ReadFile("../../testdata/names/local_shadowing_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(input))
	wants := []struct {
		line, previousLine int
		message            string
	}{
		{14, 5, "local declaration Packet shadows visible type Packet"},
		{18, 9, "local declaration Helper shadows visible function Helper"},
		{23, 22, "local declaration T shadows visible generic parameter T"},
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %+v, want %d", errors, len(wants))
	}
	for i, want := range wants {
		got := errors[i]
		if got.ID != diagnostics.LocalShadowsDeclaration || got.Line != want.line || got.PreviousLine != want.previousLine || got.Message != want.message {
			t.Fatalf("error %d = %+v, want S1057 %q at line %d (previous %d)", i, got, want.message, want.line, want.previousLine)
		}
	}
}

// Disjoint sibling scopes may reuse a name, and unit symbols are exempt until
// the unit namespace decision (MD-007) is recorded.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §8 "Shadowing"
func TestLocalShadowingAllowsSiblingScopesAndUnitNames(t *testing.T) {
	input := `
module main

unit meter uint physical

fn Pick(flag: bool) int {
    if flag {
        let result := 1
        return result
    } else {
        let result := 2
        return result
    }
}

fn Units() void {
    let meter := 3
}
`
	if errors := analyzeSourceRaw(t, input); len(errors) != 0 {
		t.Fatalf("errors = %+v, want none", errors)
	}
}
