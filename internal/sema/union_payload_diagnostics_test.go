package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// A payload-carrying union variant used without its payload as a value
// reports a stable ID with help naming the canonical form. As a match pattern
// the bare variant is valid since MD-029 (TestBarePayloadVariantPatterns).
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

`)
	want := map[string]string{
		"union variant Failure.Allocation requires payload": "S1151|`Failure.Allocation(value)`",
		"union variant Failure.Shape requires payload":      "S1151|`Failure.Shape { width: ..., height: ... }`",
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

// MD-029: a bare variant pattern selects a payload-carrying variant without
// binding its payload, covers the whole variant and emits the configurable
// A2007 Information; `Variant(_)` and `Variant {}` suppress it.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — § 8 "Variant patterns"
//   - rules/declarations/unions.md — § 9.2a
//   - rules/tooling/diagnostics.md — § 24(4e)
func TestBarePayloadVariantPatterns(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Failure union {
    Allocation(int)
    Shape { width: int, height: int }
    Other
}

fn Bare(value: Failure) int {
    match value {
        Allocation => return 1
        Failure.Shape => return 2
        Other => return 0
    }
}

fn Explicit(value: Failure) int {
    match value {
        Allocation(_) => return 1
        Shape {} => return 2
        Other => return 0
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("bare payload variant patterns rejected: %v", errors)
	}
	want := map[string]string{
		"Failure.Allocation carries a `int` payload that this pattern does not inspect":           "`Allocation(_)`",
		"Failure.Shape carries fields width: int, height: int that this pattern does not inspect": "`Shape {}`",
	}
	for _, warning := range analyzer.Warnings() {
		help, ok := want[warning.Message]
		if !ok {
			t.Fatalf("unexpected advisory %+v", warning)
		}
		if warning.ID != diagnostics.OmittedPayloadPattern || warning.Severity != diagnostics.SeverityInformation || !strings.Contains(warning.Help, help) {
			t.Fatalf("%q: got ID %q severity %q help %q", warning.Message, warning.ID, warning.Severity, warning.Help)
		}
		delete(want, warning.Message)
	}
	if len(want) != 0 {
		t.Fatalf("missing advisories %v", want)
	}

	incomplete := analyzeSourceRaw(t, `
module main

type Failure union {
    Allocation(int)
    Other
}

fn Partial(value: Failure) int {
    match value {
        Allocation where true => return 1
        Other => return 0
    }
}
`)
	found := false
	for _, err := range incomplete {
		found = found || strings.Contains(err.Message, "non-exhaustive match for Failure: missing Allocation")
	}
	if !found {
		t.Fatalf("a guarded bare variant must not cover the variant: %v", incomplete)
	}
}
