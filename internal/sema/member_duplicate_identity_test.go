package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Static and instance members are distinguished and duplicates are diagnosed
// across primary and extension fragments for every member category, each
// with a stable registered identity: same-signature callables (static or
// instance, in one or several fragments) and member/field conflicts report
// S1221, associated storage colliding with another member category keeps
// S1197, and a static overload with a different signature stays valid.
//
// Rules:
//   - rules/declarations/static.md — §§ 6, 9, 28
//   - rules/declarations/functions.md — overload sets
//   - rules/tooling/diagnostics.md — stable diagnostic identities
func TestStaticAndInstanceMemberDuplicatesHaveStableIdentity(t *testing.T) {
	tests := []struct {
		name, body, id, message string
	}{
		{"static fn twice across fragments", `
impl Box {
    static fn Make() Box { return Box { value: 1 } }
}
impl extends Box {
    static fn Make() Box { return Box { value: 2 } }
}`, diagnostics.DuplicateDeclaration, `duplicate function "Box.Make" with same signature`},
		{"static and instance same signature", `
impl Box {
    static fn Size() int { return 1 }
    fn Size() int { return self.value }
}`, diagnostics.DuplicateDeclaration, `duplicate function "Box.Size" with same signature`},
		{"instance and static across fragments", `
impl Box {
    fn Size() int { return self.value }
}
impl extends Box {
    static fn Size() int { return 1 }
}`, diagnostics.DuplicateDeclaration, `duplicate function "Box.Size" with same signature`},
		{"static fn versus field", `
impl Box {
    static fn value() int { return 3 }
}`, diagnostics.DuplicateDeclaration, "method value conflicts with field value in Box"},
		{"static let twice across fragments", `
impl Box {
    static let Limit: int := 3
}
impl extends Box {
    static let Limit: int := 4
}`, diagnostics.DuplicateDeclaration, "associated member Box.Limit already declared"},
		{"static let versus static fn", `
impl Box {
    static let Limit: int := 3
    static fn Limit() int { return 1 }
}`, diagnostics.AssociatedStorageConflict, "associated member Box.Limit conflicts with method Limit"},
		{"static let in fragment versus field", `
impl Box {
}
impl extends Box {
    static let value: int := 3
}`, diagnostics.AssociatedStorageConflict, "associated member Box.value conflicts with field value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "\nmodule main\ntype Box struct { value: int, }\n"+test.body+"\n")
			if len(errors) != 1 || errors[0].ID != test.id || !strings.Contains(errors[0].Message, test.message) {
				t.Fatalf("want one %s %q, got %+v", test.id, test.message, errors)
			}
		})
	}
	valid := analyzeSourceRaw(t, `
module main
type Box struct { value: int, }
impl Box {
    static fn Make() Box { return Box { value: 1 } }
    static fn Make(v: int) Box { return Box { value: v } }
}
`)
	assertSemaErrors(t, valid, nil)
}
