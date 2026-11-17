package main

import (
	"os"
	"strings"
	"testing"
)

// Editor diagnostics use the same generic volatile eligibility authority as
// compiler check, including in unsaved generic method bodies.
// Rules: rules/platform/volatile.md — §§33,38;
// rules/tooling/lsp.md — Diagnostics.
func TestVolatileGenericEligibilityLSP(t *testing.T) {
	testVolatileEligibilityLSP(t, "volatile_generic", []int{5, 9, 13, 17, 24, 27}, "requires proven volatile eligibility for generic pointee T")
}

// Unsaved editor sources preserve the same aggregate contract check and focused
// call locations as Sema, independently of unsafe authorization.
// Rules: rules/platform/volatile.md — §37; rules/tooling/lsp.md — Diagnostics.
func TestVolatileAggregateEligibilityLSP(t *testing.T) {
	testVolatileEligibilityLSP(t, "volatile_aggregate", []int{12, 13, 18, 19, 24, 25, 30, 31, 36, 37, 42, 43, 48, 49, 54, 55, 60, 61, 66, 67}, "requires an explicit physical layout and access contract for aggregate")
}

// testVolatileEligibilityLSP compares valid and invalid editor sources against
// the canonical type-eligibility errors and operation locations.
// Rules: rules/platform/volatile.md — §§33,37–38;
// rules/tooling/lsp.md — Diagnostics.
func testVolatileEligibilityLSP(t *testing.T, fixture string, lines []int, message string) {
	t.Helper()
	for _, valid := range []bool{true, false} {
		name := fixture + "_invalid.sec"
		if valid {
			name = fixture + "_valid.sec"
		}
		source, err := os.ReadFile("../../testdata/sema/" + name)
		if err != nil {
			t.Fatal(err)
		}
		diagnostics := analyze("file:///tmp/sec-"+fixture+"/main.sec", string(source))
		count := 0
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity != 1 {
				continue
			}
			count++
			if valid || !strings.Contains(diagnostic.Message, message) || count > len(lines) || int(diagnostic.Range.Start.Line) != lines[count-1] {
				t.Fatalf("unexpected diagnostic: %+v", diagnostic)
			}
		}
		want := len(lines)
		if valid {
			want = 0
		}
		if count != want {
			t.Fatalf("got %d errors, want %d: %+v", count, want, diagnostics)
		}
	}
}
