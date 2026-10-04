package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// `Self` in an interface requirement names the conforming type.
//
// Rules:
//   - rules/declarations/interfaces.md — 3.4 "Static member", 6 "Conformance requirements"
func TestInterfaceSelfResolvesToTheConformingType(t *testing.T) {
	errors := analyzeSource(t, `module main

type ParseError enum error {
    Invalid,
}

interface Parsable {
    static fn Parse(value: string) Result[Self, ParseError]
    fn Combine(other: Self) Self
}

type Count struct {
    value: int,
}

impl Count implements Parsable {
    static fn Parse(value: string) Result[Count, ParseError] {
        return Ok(Count{ value: 1 })
    }

    fn Combine(other: Count) Count {
        return Count{ value: self.value + other.value }
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("Self conformance errors: %+v", errors)
	}
}

// Conformance failures are reported at the implementation with the interface
// requirement as related location, and name the first difference.
//
// Rules:
//   - rules/declarations/interfaces.md — 12 "Diagnostics"
func TestInterfaceConformanceDiagnosticsExplainTheMismatchAtTheImplementation(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

interface Figure {
    fn Area() int
    static fn Make(size: int) Self
    fn Scale(factor: ref int) void
    fn Name() string
}

type Square struct {
    side: int,
}

impl Square implements Figure {
    static fn Area() int {
        return 1
    }

    static fn Make(size: string) Square {
        return Square{ side: 1 }
    }

    fn Scale(factor: int) void {
    }
}
`)
	wants := []struct {
		id      string
		line    int
		message string
	}{
		{diagnostics.InterfaceMemberIncompatible, 15, "method Area does not match interface Figure: the interface requires an instance method, but it is declared as a static method"},
		{diagnostics.InterfaceMemberIncompatible, 19, "method Make does not match interface Figure: parameter 1 size has type string, but the interface requires int"},
		{diagnostics.InterfaceMemberIncompatible, 23, "method Scale does not match interface Figure: parameter 1 factor is by value, but the interface requires a ref borrow"},
		{diagnostics.InterfaceMemberMissing, 14, "type Square implements Figure but is missing method Name"},
	}
	for _, want := range wants {
		found := false
		for _, err := range errors {
			if err.ID == want.id && err.Line == want.line && strings.Contains(err.Message, want.message) && err.RelatedLabel == "interface requirement" && err.PreviousLine > 0 {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s at line %d %q in %+v", want.id, want.line, want.message, errors)
		}
	}
}

// An unresolved type in a requirement is reported where it is written and does
// not cascade into a parameter-count conformance error.
func TestInterfaceConformanceSkipsRequirementsWithUnresolvedTypes(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

interface Seeker {
    fn Seek(offset: int, origin: MissingOrigin) int
}

type File struct {
    position: int,
}

impl File implements Seeker {
    fn Seek(offset: int, origin: int) int {
        return offset
    }
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "unknown type MissingOrigin") {
		t.Fatalf("errors = %+v", errors)
	}
}
