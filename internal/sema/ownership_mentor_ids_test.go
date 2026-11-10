package sema

import (
	"strings"
	"testing"
)

// Every ownership failure named by the mentor-diagnostic requirement carries
// a stable registered ID and actionable help: missing move markers at call,
// construction and binding sites (S1007), Result projection consumption
// (S1006), use after move (S1088), move while borrowed (S1163) and argument
// overlap (S1160).
//
// Rules:
//   - rules/memory/ownership.md — § 33 "Diagnostics" (§ 33.1 missing move markers)
//   - rules/tooling/diagnostics.md — stable IDs and help
func TestOwnershipFailuresCarryStableIDsAndHelp(t *testing.T) {
	const prelude = `
module main

type Res struct {
    data: int[],
}

type Holder struct {
    inner: Res,
}

fn Take(r: Res) void {}

fn Both(a: ref mut int, b: ref mut int) void {}
`
	tests := []struct {
		name, body, id, help string
	}{
		{"missing marker at call", `
fn Use() void {
    let r := Res { data: [] }
    Take(r)
}`, "S1007", "pass `<-r`"},
		{"implicit consuming construction", `
fn Use() Holder {
    let r := Res { data: [] }
    let h := Holder { inner: r }
    return h
}`, "S1007", "Write `<-r`"},
		{"missing marker at binding", `
fn Use() void {
    let r := Res { data: [] }
    let s := r
    discard s
}`, "S1007", ":<-"},
		{"use after move", `
fn Use() void {
    let r := Res { data: [] }
    Take(<-r)
    Take(<-r)
}`, "S1088", "use r before that operation"},
		{"move while borrowed", `
fn Use() void {
    let r := Res { data: [] }
    let view: ref Res := ref r
    Take(<-r)
    discard view
}`, "S1163", "end the conflicting borrow"},
		{"argument overlap", `
fn Use() void {
    let mut x := 1
    Both(ref mut x, ref mut x)
}`, "S1160", "end the conflicting borrow"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prelude+test.body+"\n")
			for _, err := range errors {
				if err.ID == test.id && strings.Contains(err.Help+err.Message, test.help) {
					if err.Help == "" {
						t.Fatalf("%s has no help: %+v", test.id, err)
					}
					return
				}
			}
			t.Fatalf("want %s with %q, got %+v", test.id, test.help, errors)
		})
	}
	errors := analyzeSourceRaw(t, resultStateProofPrelude+`
fn Use() int {
    let result := Load()
    discard result.Ok()
    return 0
}
`)
	if len(errors) != 1 || errors[0].ID != "S1006" || errors[0].Help == "" {
		t.Fatalf("Result projection consumption needs S1006 with help, got %+v", errors)
	}
}
