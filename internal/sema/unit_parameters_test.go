package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestUnitPolymorphicParametersReserved rejects type parameters at unit factors
// while allowing an independently declared unit with the same name.
// Rules: rules/types/units.md — Future unit-polymorphic generics; unit namespaces.
func TestUnitPolymorphicParametersReserved(t *testing.T) {
	for _, typ := range []string{"decimal<U>", "<U>", "decimal<m/(U^2)>"} {
		t.Run(typ, func(t *testing.T) {
			line := "fn F[U](value: " + typ + ") void {}"
			errors := analyzeSourceRaw(t, "module main\nunit m physical\n"+line+"\n")
			found := false
			for _, e := range errors {
				if e.ID == diagnostics.UnitPolymorphismReserved {
					found = true
					if e.Line != 3 || e.Column != strings.LastIndex(line, "U")+1 || !strings.Contains(e.Help, "concrete declared unit") {
						t.Fatal(e)
					}
				}
			}
			if !found {
				t.Fatal(errors)
			}
		})
	}
	errors := analyzeSourceRaw(t, "module main\nunit U physical\nfn F[U](value: decimal<U>, other: U) void {}\n")
	if len(errors) != 0 {
		t.Fatal(errors)
	}
}
