package sema

import (
	"fmt"
	"strings"
	"testing"
)

// A borrow ends at its holder's last required use on every path, not at the
// end of the block (MD-061 § 4): mutation after the last use is accepted,
// while a later use, a defer body, a loop backedge or a use on another path
// keeps the borrow live and the conflict is still rejected.
//
// Rules:
//   - rules/memory/borrowing.md — § 8.1 "Compiler-derived liveness"
func TestBorrowEndsAtLastRequiredUse(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Read(v: ref int) int {
    return 0
}

fn AfterLastUse() int {
    let mut value := 1
    let a := ref value
    let n := Read(a)
    value = 2
    let b := ref mut value
    return n
}

fn BranchLastUse(flag: bool) void {
    let mut value := 1
    let a := ref value
    if flag {
        discard Read(a)
    } else {
        value = 3
    }
}

fn LaterUse() int {
    let mut value := 1
    let a := ref value
    value = 2
    return Read(a)
}

fn Deferred() void {
    let mut value := 1
    let a := ref value
    defer {
        discard Read(a)
    }
    value = 2
}

fn LoopBackedge() void {
    let mut value := 1
    let a := ref value
    let mut i := 0
    while i < 3 {
        discard Read(a)
        value = 2
        i += 1
    }
}
`)
	var conflicts []string
	for _, err := range errors {
		if strings.Contains(err.Message, "cannot assign to value while it is shared borrowed") {
			conflicts = append(conflicts, fmt.Sprintf("%d:%d", err.Line, err.Column))
		}
	}
	if len(errors) != 3 || strings.Join(conflicts, " ") != "30:5 40:5 49:9" {
		t.Fatalf("want conflicts only in LaterUse, Deferred and LoopBackedge (30:5 40:5 49:9), got %v: %+v", conflicts, errors)
	}
}

// `ref r` / `ref mut r` over a reference holder reborrow the referent
// (MD-061 § 3): ref T from ref T and ref mut T, ref mut T from ref mut T, and
// never ref ref T; elevating a shared holder is an error, and every legal
// explicit reborrow carries the Information diagnostic A2005.
func TestExplicitReborrowTypesAndInformation(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Inspect(v: ref int) int {
    return 0
}

fn Modify(v: ref mut int) void {
}

fn Cases() void {
    let mut value := 42
    let r := ref mut value
    let view := ref r
    discard Inspect(view)
    Modify(r)

    let mut other := 1
    let shared := ref other
    let again := ref shared
    let same := shared == again

    let mut third := 2
    let exclusive := ref mut third
    let child := ref mut exclusive
    Modify(child)
}

fn Elevate() void {
    let mut value := 1
    let shared := ref value
    let elevated := ref mut shared
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot reborrow shared reference shared as ref mut") {
		t.Fatalf("want only the elevation error, got %+v", errors)
	}
	information := 0
	for _, warning := range analyzer.Warnings() {
		if warning.ID == "A2005" && warning.Severity == "information" {
			information++
			if strings.Contains(warning.Help, "instead") || strings.Contains(warning.Help, "replace") {
				t.Errorf("reborrow information must not prescribe replacing the holder: %q", warning.Help)
			}
		}
	}
	if information != 3 {
		t.Fatalf("want three A2005 information diagnostics, got %+v", analyzer.Warnings())
	}
}
