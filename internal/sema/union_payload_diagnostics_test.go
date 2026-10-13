package sema

import (
	"strings"
	"testing"
)

// A payload-carrying union variant used without its payload, as a value or
// as a match pattern, reports a stable ID with help naming the canonical form.
//
// Rules:
//   - rules/declarations/unions.md — §§5.2,5.3 construction
//   - rules/declarations/unions.md — §§8.1,9.2–9.4 is tests and match patterns
func TestUnionVariantMissingPayloadHasStableDiagnostics(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Failure union {
    Allocation(int)
    Shape { width: int, height: int }
    Other
}

fn Bare() Failure {
    return Failure.Allocation
}

fn Fields() Failure {
    return Failure.Shape
}

fn Pattern(value: Failure) int {
    match value {
        Allocation => return 1
        Shape(shape) => return shape.width
        Other => return 0
    }
}
`)
	want := map[string]string{
		"union variant Failure.Allocation requires payload":         "S1151|`Failure.Allocation(value)`",
		"union variant Failure.Shape requires payload":              "S1151|`Failure.Shape { width: ..., height: ... }`",
		"union variant Failure.Allocation requires payload binding": "S1152|`value is Failure.Allocation`",
	}
	for _, err := range errors {
		expected, ok := want[err.Message]
		if !ok {
			continue
		}
		id, help, _ := strings.Cut(expected, "|")
		if err.ID != id || !strings.Contains(err.Help, help) {
			t.Fatalf("%q: got ID %q help %q, want %s with %s", err.Message, err.ID, err.Help, id, help)
		}
		delete(want, err.Message)
	}
	if len(want) != 0 {
		t.Fatalf("missing diagnostics %v in %+v", want, errors)
	}
}
