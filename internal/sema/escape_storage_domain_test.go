package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

const storageDomainDeclarations = `module main

type Holder struct {
    value: ref int,
}

let shared: int := 7
let mut slot: ref int := ref shared
let mut holder: Holder := Holder { value: ref shared }
let mut count: int := 0

`

// Storing a borrowed dependency into Static storage is an escape to the
// static storage domain; lifetime validation accepts it only when the
// referent is itself static (escape_analysis.md "Static and thread-local
// destinations"; lifetime_analysis.md § 19).
func TestStaticStoreOfShortLivedReferenceIsRejected(t *testing.T) {
	for _, test := range []struct {
		body, origin string
	}{
		{"let x := 1\n    slot = ref x", "x"},
		{"let x := 1\n    slot = x", "x"},
		{"let x := 1\n    holder.value = ref x", "x"},
		{"let x := 1\n    holder = Holder { value: ref x }", "x"},
	} {
		source := storageDomainDeclarations + "fn Store() void {\n    " + test.body + "\n}\n"
		errors := analyzeSourceRaw(t, source)
		if len(errors) != 1 || errors[0].ID != diagnostics.StaticStoreOfShortLivedReference {
			t.Fatalf("%q errors = %v, want one %s", test.body, errors, diagnostics.StaticStoreOfShortLivedReference)
		}
		if !strings.Contains(errors[0].Message, "reference to "+test.origin+" into static storage") {
			t.Fatalf("%q message = %q", test.body, errors[0].Message)
		}
		causes := errors[0].EscapeCauses
		if len(causes) != 1 || causes[0].Destination != EscapeDestinationStaticStorage {
			t.Fatalf("%q causes = %#v", test.body, causes)
		}
	}
}

// A borrowed parameter is valid only for the call, so storing it in static
// storage is rejected and the parameter is summarized as Retained.
func TestStaticStoreOfBorrowedParameterIsRejectedAndRetained(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, storageDomainDeclarations+"fn Keep(value: ref int) void {\n    slot = value\n}\n")
	if len(errors) != 1 || errors[0].ID != diagnostics.StaticStoreOfShortLivedReference {
		t.Fatalf("errors = %v", errors)
	}
	if steps := errors[0].EscapeCauses[0].Steps; steps[0].Message != "origin is a borrowed parameter, valid only for this call" {
		t.Fatalf("origin step = %#v", steps[0])
	}
	summary := escapeSummaryNamed(t, analyzer.EscapeAnalysis(), "Keep")
	if len(summary.Parameters) != 1 || !hasEscapeDisposition(summary.Parameters[0].Dispositions, EscapeParameterRetained) {
		t.Fatalf("Keep parameters = %#v", summary.Parameters)
	}
}

// References into static storage may be stored into static storage, and the
// store is still recorded as an escape to the static domain; owned values
// without references record nothing.
func TestStaticStoreOfStaticReferenceIsAcceptedAndRecorded(t *testing.T) {
	source := storageDomainDeclarations + `fn Store(number: int) void {
    slot = ref shared
    holder.value = ref shared
    holder = Holder { value: ref shared }
    count = number
}
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	assertSemaErrors(t, errors, nil)
	staticFacts := 0
	for _, fact := range analyzer.EscapeAnalysis().Facts() {
		if fact.Destination != EscapeDestinationStaticStorage {
			continue
		}
		staticFacts++
		if fact.Mode != EscapeModeBorrowEscape || fact.Unknown || len(fact.Sources) != 1 || fact.Sources[0].Kind != EscapeSourceStatic || fact.Sources[0].Name != "shared" {
			t.Fatalf("static store fact = %#v", fact)
		}
	}
	if staticFacts != 3 {
		t.Fatalf("static store facts = %d, want 3", staticFacts)
	}
}
