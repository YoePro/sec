package sema

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// P13 aggregate copyability follows owned slice fields recursively, while a
// shared slice remains copyable independently of its referents. Contained
// origins survive field extraction, whole-holder copies and returned fields.
// Rules: rules/memory/copy_move.md — §§4.1–4.4;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§39,66–69,114,140.
func TestPackage16AggregateSliceTraitsAndOrigins(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			_, a, errs := readSliceMatrixInput(t, "slice_aggregate_traits.sec", width)
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			for name, want := range map[string]CopyClassification{"Shared": CopyTrivial, "NestedShared": CopyTrivial, "Exclusive": CopyMoveOnly, "NestedExclusive": CopyMoveOnly, "SharedExclusiveElements": CopyTrivial} {
				typ, ok := a.Types()[name]
				if !ok || CopyClassificationOf(typ) != want || !TriviallyDestructible(typ) {
					t.Fatalf("%s traits=%s trivial destruction=%v", name, CopyClassificationOf(typ), TriviallyDestructible(typ))
				}
			}
			for _, test := range []struct{ file, want string }{
				{"slice_aggregate_escape_invalid.sec", "local"},
				{"slice_aggregate_copy_invalid.sec", "move-only"},
			} {
				_, _, failures := readSliceMatrixInput(t, test.file, width)
				found := false
				for _, failure := range failures {
					if strings.Contains(failure.Message, test.want) {
						found = true
					}
				}
				if !found {
					t.Fatalf("%s lost %q diagnostic: %v", test.file, test.want, failures)
				}
			}

			program, a, errs := readSliceMatrixInput(t, "slice_origins_valid.sec", width)
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			plans := sliceBindingPlans(program, a)
			for _, name := range []string{"fieldRange", "copiedFieldRange", "unwrappedRange", "methodRange", "referencedFieldRange"} {
				source, got := plans["Observe/source"], plans["Observe/"+name]
				if got.SourcePlace.String() != "values[3..<5]" || got.SourceLength == nil || got.SourceLength.String() != "2" || !reflect.DeepEqual(got.ReferenceFacts.Facts.StorageIdentities, source.ReferenceFacts.Facts.StorageIdentities) || got.ReferenceFacts.Facts.AddressSpace != source.ReferenceFacts.Facts.AddressSpace || got.ReferenceFacts.Facts.InvalidationDomain != source.ReferenceFacts.Facts.InvalidationDomain || got.ReferenceFacts.Facts.EpochDependency != source.ReferenceFacts.Facts.EpochDependency {
					t.Fatalf("%s lost contained range/storage/validity facts: %+v", name, got)
				}
			}
		})
	}
}
