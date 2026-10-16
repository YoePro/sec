package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

const md069Prelude = `
module main

type Counter struct {
    Value: int,
}

type Container struct {
    Count: Counter,
}

fn GetCounter(outer: ref mut Container) ref mut Counter {
    return ref mut outer.Count
}
`

// MD-069 (2026-10-16): a child borrow ends at its last use, after which the
// origin root may be used again; a later child use, a call-returned holder
// still in use, or a deferred use keeps the child live. Cases follow the
// correction's § 7 examples.
//
// Rules:
//   - rules/memory/borrowing.md — § 8.1(7)–(7b)
//   - rules/memory/lifetime_analysis.md — § 14.7
func TestMD069OriginRootAndLastUse(t *testing.T) {
	for name, test := range map[string]struct {
		body string
		want string
	}{
		"last use allows parent": {`
fn Use(outer: ref mut Container) void {
    let count := ref mut outer.Count
    count.Value = 10
    outer.Count.Value = 20
}`, ""},
		"child still live": {`
fn Use(outer: ref mut Container) void {
    let count := ref mut outer.Count
    count.Value = 10
    outer.Count.Value = 20
    count.Value = 30
}`, diagnostics.BorrowMutationConflict},
		"returned origin last use": {`
fn Use(outer: ref mut Container) void {
    let count := GetCounter(outer)
    count.Value = 10
    outer.Count.Value = 20
}`, ""},
		"returned origin still live": {`
fn Use(outer: ref mut Container) void {
    let count := GetCounter(outer)
    count.Value = 10
    outer.Count.Value = 20
    count.Value = 30
}`, diagnostics.BorrowMutationConflict},
		"copied shared holder keeps child live": {`
fn Use(outer: ref Container) int {
    let count := ref outer.Count
    let again := count
    return again.Value
}`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, md069Prelude+test.body+"\n")
			if test.want == "" {
				if len(errors) != 0 {
					t.Fatalf("unexpected errors: %v", errors)
				}
				return
			}
			if len(errors) != 1 || errors[0].ID != test.want {
				t.Fatalf("want one %s, got %v", test.want, errors)
			}
		})
	}
}

// MD-064 (2026-10-16): `ref` and `ref mut` of a computed property and of a
// temporary report the mandatory S1168 whose help distinguishes the local
// copy from the property's storage.
//
// Rules:
//   - rules/tooling/diagnostics.md — § 24(4b)
//   - rules/memory/borrowing.md — § 5.2(5)–(7)
func TestMD064NonAddressableBorrowGuidance(t *testing.T) {
	for name, body := range map[string]string{
		"shared property":  "let total := ref counter.Total",
		"mutable property": "let total := ref mut counter.Total",
		"temporary":        "let total := ref Start()",
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, `
module main

type Counter struct {
    value: int,
}

impl Counter {
    property Total: int {
        get { return self.value }
    }
}

fn Start() Counter { return Counter { value: 1 } }

fn Use() int {
    let mut counter := Counter { value: 2 }
    `+body+`
    return 0
}
`)
			if len(errors) != 1 || errors[0].ID != diagnostics.BorrowNonAddressable || errors[0].Severity != diagnostics.SeverityError {
				t.Fatalf("want one mandatory S1168, got %v", errors)
			}
			if help := errors[0].Help; !strings.Contains(help, "local copy, not to the original property") || !strings.Contains(help, "explicitly returns a reference") {
				t.Fatalf("S1168 help does not distinguish the local copy: %q", help)
			}
		})
	}
	if definition, ok := diagnostics.Lookup(diagnostics.BorrowNonAddressable); !ok || definition.Name != "borrowing.non-addressable-place" || !definition.Mandatory {
		t.Fatalf("S1168 registry identity: %+v", definition)
	}
}
