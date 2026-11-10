package sema

import (
	"testing"

	"sec/internal/diagnostics"
)

// Static members are accessed through their type: a static fn called through
// a value and static storage read through a value report S1210 with a
// type-qualified replacement, while type-qualified access, instance members
// and same-named instance overloads stay valid.
//
// Rules:
//   - rules/declarations/static.md — § 8 "Static member access", § 28 "Diagnostics"
func TestStaticMembersRequireTypeQualifiedAccess(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Box struct {
    value: int,
}

impl Box {
    static fn Make() Box {
        return Box { value: 1 }
    }
    static let Limit: int := 3
    fn Value() int {
        return self.value
    }
}

fn Valid(box: Box) int {
    let made := Box.Make()
    discard made
    return Box.Limit + box.Value() + box.value
}

fn Invalid(box: Box) int {
    let other := box.Make()
    discard other
    return box.Limit
}
`)
	want := map[string]string{
		"static member Make must be accessed through type Box":  "Write Box.Make instead",
		"static member Limit must be accessed through type Box": "Write Box.Limit instead",
	}
	if len(errors) != len(want) {
		t.Fatalf("want %d S1210 errors, got %v", len(want), errors)
	}
	for _, err := range errors {
		help, ok := want[err.Message]
		if !ok || err.ID != diagnostics.StaticTypeQualifiedAccess || len(err.Help) < len(help) || err.Help[:len(help)] != help {
			t.Fatalf("unexpected diagnostic %+v", err)
		}
	}
}
