package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Literal boolean conditions prove the excluded branch unreachable. Its first
// statement must use the same mandatory S3001 diagnostic as code following a
// terminating statement; an empty excluded branch has no statement to report.
//
// Rules:
//   - rules/tooling/diagnostics.md — §21 "Proven unreachable and dead code"
//   - rules/control-flow/flowcontrol_if.md — §20 "Constant conditions and unreachable code"
func TestConstantIfBranchesReportCanonicalUnreachableStatement(t *testing.T) {
	errors := analyzeSource(t, `
fn Check() void {
	if false {
		discard 1
	}
	if true {
		discard 2
	} else {
		discard 3
	}
	if false {}
}
`)

	if len(errors) != 2 {
		t.Fatalf("errors = %v, want two unreachable-branch diagnostics", errors)
	}
	for _, diagnostic := range errors {
		if diagnostic.ID != diagnostics.UnreachableStatement ||
			diagnostic.Severity != diagnostics.SeverityError ||
			!strings.Contains(diagnostic.Message, "unreachable statement") ||
			!strings.Contains(diagnostic.Help, "constant condition") {
			t.Fatalf("incomplete S3001 diagnostic: %+v", diagnostic)
		}
	}
}

// Folded integer comparisons and boolean operators are compile-time boolean
// conditions too. They must drive the same S3001 path proof as literal
// conditions, while a runtime-dependent comparison remains conditional.
//
// Rules:
//   - rules/tooling/diagnostics.md — §21 "Proven unreachable and dead code"
//   - rules/control-flow/flowcontrol_if.md — §20 "Constant conditions and unreachable code"
//   - rules/control-flow/flowcontrol_while.md — §19 "Constant conditions"
func TestFoldedBooleanConditionsReportCanonicalUnreachableStatement(t *testing.T) {
	errors := analyzeSource(t, `
fn Check(flag: bool, value: int) void {
	if 1 + 1 == 2 {
	} else {
		discard 1
	}
	if !(3 < 4) {
		discard 2
	}
	if false && flag {
		discard 3
	}
	if true || flag {
	} else {
		discard 6
	}
	if true == false {
		discard 7
	}
	while 10 / 2 > 5 {
		discard 4
	}
	let limit := 4
	if limit <= 4 {
	} else {
		discard 5
	}
	if value > 0 {
		discard value
	}
}
`)

	if len(errors) != 7 {
		t.Fatalf("errors = %v, want seven folded-condition unreachable diagnostics", errors)
	}
	for _, diagnostic := range errors {
		if diagnostic.ID != diagnostics.UnreachableStatement ||
			diagnostic.Severity != diagnostics.SeverityError ||
			!strings.Contains(diagnostic.Help, "constant condition") {
			t.Fatalf("incomplete folded-condition S3001 diagnostic: %+v", diagnostic)
		}
	}
}

// A literal-false while body is a proven-unreachable block. Its first
// statement uses S3001, while the explicitly permitted empty form remains
// diagnostic-free.
//
// Rules:
//   - rules/tooling/diagnostics.md — §21 "Proven unreachable and dead code"
//   - rules/control-flow/flowcontrol_while.md — §19 "Constant conditions"
func TestConstantFalseWhileReportsCanonicalUnreachableStatement(t *testing.T) {
	errors := analyzeSource(t, `
fn Check() void {
	while false {
		discard 1
	}
	while false {}
}
`)

	if len(errors) != 1 {
		t.Fatalf("errors = %v, want one unreachable-loop-body diagnostic", errors)
	}
	diagnostic := errors[0]
	if diagnostic.ID != diagnostics.UnreachableStatement ||
		diagnostic.Severity != diagnostics.SeverityError ||
		!strings.Contains(diagnostic.Message, "unreachable statement") ||
		!strings.Contains(diagnostic.Help, "loop body") {
		t.Fatalf("incomplete S3001 diagnostic: %+v", diagnostic)
	}
}
