package sema

import (
	"strings"
	"testing"
)

// Concrete generic type arguments must satisfy every declared interface
// constraint for structs, named types, unions, and interfaces. A structurally
// unrelated type with only partial explicit conformance remains invalid.
//
// Rules:
//   - rules/declarations/generics.md — §12 "Multiple constraints"
//   - rules/declarations/generics.md — §14 "Constraint satisfaction"
//   - rules/declarations/generics.md — §25 "Concrete specialization"
func TestGenericTypeConstraintsRequireEveryConjunct(t *testing.T) {
	input := `
module main

interface First {}
interface Second {}

type Both struct {}
impl Both implements First, Second {}

type OnlyFirst struct {}
impl OnlyFirst implements First {}

type Box[T: First & Second] struct {
    value: T
}

type Wrapped[T: First & Second] T

type Maybe[T: First & Second] union {
    Some(T)
    None
}

interface View[T: First & Second] {}

enum Choice[T: First & Second] {
    One
}

fn Valid(box: Box[Both], wrapped: Wrapped[Both], maybe: Maybe[Both], view: View[Both], choice: Choice[Both]) void {}

fn Invalid(box: Box[OnlyFirst], wrapped: Wrapped[OnlyFirst], maybe: Maybe[OnlyFirst], view: View[OnlyFirst], choice: Choice[OnlyFirst]) void {}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	if len(errors) != 5 {
		t.Fatalf("constraint errors = %v, want five failures", errors)
	}
	for _, diagnostic := range errors {
		if !strings.Contains(diagnostic.Message, "type OnlyFirst does not satisfy constraint Second for T") {
			t.Fatalf("unexpected constraint diagnostic: %v", diagnostic)
		}
	}

	for _, name := range []string{"Box", "Wrapped", "Maybe", "View", "Choice"} {
		template := analyzer.types[name]
		if len(template.GenericConstraints) != 2 ||
			template.GenericConstraints[0].Interface.Name != "First" ||
			template.GenericConstraints[1].Interface.Name != "Second" {
			t.Fatalf("%s constraints = %+v, want ordered First & Second", name, template.GenericConstraints)
		}
	}
}
