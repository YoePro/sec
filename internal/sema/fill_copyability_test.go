package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Repeated-copy forms reject move-only elements even for zero elements. The
// source must receive a registered diagnostic rather than silently constructing
// multiple owners from one value.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill";
// rules/collections/collections.md — §7.9; rules/memory/copy_move.md — §3.
func TestFillCopyabilityRejectsMoveOnlyElements(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/fill_copyability_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors := analyzeSourceWithAnalyzer(t, string(data))
	if len(errors) != 5 {
		t.Fatalf("errors = %v, want five copyability failures", errors)
	}
	for _, err := range errors {
		if err.ID != diagnostics.FillElementNotCopyable || err.Line == 0 || err.Column == 0 || !strings.Contains(err.Message, "requires a copyable element type, got Resource") || !strings.Contains(err.Help, "evaluates its source once") {
			t.Fatalf("unexpected diagnostic: %+v", err)
		}
	}
	definition, ok := diagnostics.Lookup(diagnostics.FillElementNotCopyable)
	if !ok || !definition.Mandatory || definition.Name != "collections.fill-element-not-copyable" {
		t.Fatalf("unregistered copyability diagnostic: %+v", definition)
	}
}

// Ordinary copyable values keep contextual construction and slice replacement,
// with the existing fixed-array/Result result types and empty construction.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill";
// rules/collections/collections.md — §7.9.
func TestFillCopyabilityKeepsCopyableElements(t *testing.T) {
	_, errors := analyzeSourceWithAnalyzer(t, `module main
type Point struct { x: int }
fn F(points: ref mut Point[]) Result[void, CollectionError] {
    let fixed: Point[2] := fill(Point { x: 7 })
    let empty: Point[0] := fill(Point { x: 7 })
    let owned: Point[] := try fill(Point { x: 7 }, 2u)
    let emptyOwner: Point[] := try fill(Point { x: 7 }, 0u)
    points.Fill(Point { x: 7 })
    let text: string := try fill("=", 4u)
    return Ok()
}
`)
	if len(errors) != 0 {
		t.Fatalf("copyable fill rejected: %v", errors)
	}
}
