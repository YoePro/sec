package sema

import (
	"strings"
	"testing"
)

// Unit symbols occupy a separate unit-symbol namespace: parameters, generic
// parameters, locals, module functions, and module variables may share a
// unit's spelling, while unit-vs-unit duplicates and ordinary shadowing of
// real types are still diagnosed.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §2 "One declaration namespace per scope"
//   - rules/types/units.md — "Unit names and compiler-known names"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — § 8
func TestUnitSymbolsOccupySeparateNamespace(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

unit day physical
unit ms physical

type Span int

fn ms() int {
    return 1
}

fn FromDays(day: int) int {
    let ms := day * 86400000
    return ms
}

fn Generic[day](value: day) day {
    return value
}

fn Shadow(Span: int) int {
    return Span
}

unit day physical
`)
	got := []string{}
	for _, err := range errors {
		got = append(got, err.Message)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"parameter Span shadows visible type Span",
		"unit day already declared",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	for _, unwanted := range []string{"shadows visible type day", "shadows visible type ms", "conflicts with unit", "conflicts with function"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("unit symbol treated as ordinary declaration (%q):\n%s", unwanted, joined)
		}
	}
}
