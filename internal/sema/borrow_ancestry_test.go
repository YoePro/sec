package sema

import (
	"fmt"
	"strings"
	"testing"
)

// Nested granting authority must not turn siblings or suspended parents into
// legal access paths, even when they share storage and range coordinates.
// Rules: rules/memory/borrowing.md — §§3.2(4),4(3),10(6–8).
func TestSliceReborrowAncestryKeepsConflicts(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			if _, _, errs := readSliceMatrixInput(t, "slice_reborrow_ancestry_valid.sec", width); len(errs) != 0 {
				t.Fatalf("legal descendant access: %v", errs)
			}
			_, _, errs := readSliceMatrixInput(t, "slice_reborrow_ancestry_invalid.sec", width)
			creation, mutation := false, false
			for _, err := range errs {
				creation = creation || strings.Contains(err.Message, "cannot create shared reference")
				mutation = mutation || strings.Contains(err.Message, "cannot assign to")
			}
			if !creation || !mutation {
				t.Fatalf("sibling/parent conflicts lost: %v", errs)
			}
		})
	}
}

// Joined, cyclic or loop-carried links cannot prove ancestry existentially;
// each represented origin must establish the same granting relationship.
// Rules: rules/memory/borrowing.md — §§12,14,32;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§27–30,43.
func TestGrantingBorrowAncestryRequiresAllEdges(t *testing.T) {
	place := Place{Root: "values", RootID: 1, ReferenceHolder: "child"}
	record := borrowRecord{Holder: "parent"}
	for _, bad := range []string{"other", "child", ""} {
		a := NewAnalyzer()
		a.borrows = map[string][]borrowRecord{"values": {
			{Holder: "child", Kind: mutableBorrow, Place: Place{RootID: 1, ReferenceHolder: "parent"}},
			{Holder: "child", Kind: mutableBorrow, Place: Place{RootID: 1, ReferenceHolder: bad}},
		}}
		if a.grantingBorrowAncestor(place, record) {
			t.Fatal("accepted incomplete ancestry", bad)
		}
	}
	for _, limitation := range []string{"unknown", "ambiguous", "loop-carried"} {
		a := NewAnalyzer()
		edge := borrowRecord{Holder: "child", Kind: mutableBorrow, Place: Place{RootID: 1, ReferenceHolder: "parent"}}
		switch limitation {
		case "unknown":
			edge.Place.RootID = 0
		case "ambiguous":
			edge.Place.AmbiguousProvenance = true
		case "loop-carried":
			edge.LoopCarried = true
		}
		a.borrows = map[string][]borrowRecord{"values": {edge}}
		if a.grantingBorrowAncestor(place, record) {
			t.Fatal("accepted unproven ancestry", limitation)
		}
	}
}
