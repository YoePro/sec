package sema

import (
	"os"
	"sec/internal/diagnostics"
	"testing"
)

// TestPropertyReceiverInference closes getter/method requirements together and
// respects shadowing, ref mut, owned storage and fresh temporary authority.
// Rules: rules/declarations/properties.md — §8; rules/memory/borrowing.md — §§5.4,7.
func TestPropertyReceiverInference(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/property_receivers/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	typ := a.types["Counter"]
	for name, want := range map[string]bool{"Direct": true, "Indirect": true, "Bare": true, "ThroughMethod": true, "Borrow": true, "Pure": false, "Shadow": false, "ShadowBinding": false} {
		property, ok := lookupProperty(typ, name)
		if !ok || property.GetterCallable == nil || property.GetterCallable.ReceiverMutable != want {
			t.Fatalf("%s: %+v", name, property.GetterCallable)
		}
		facts, known := a.ReceiverReferenceFactsOf(*property.GetterCallable)
		if !known || facts.MutableAuthority != want {
			t.Fatalf("%s facts = %+v, known %v", name, facts, known)
		}
	}
	for name, want := range map[string]bool{"Counter.ThroughProperty": true, "Counter.BareMethod": true, "Counter.ShadowParameter": false} {
		if a.functions[name][0].ReceiverMutable != want {
			t.Fatal(name, a.functions[name][0])
		}
	}
}

// TestPropertyReceiverAuthority rejects both mutating reads and setter writes
// through immutable root storage or a shared reference.
// Rules: rules/declarations/properties.md — §8; rules/memory/borrowing.md — §7.
func TestPropertyReceiverAuthority(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/property_receivers/authority_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 5 {
		t.Fatal(errors)
	}
	count := 0
	for _, e := range errors {
		if e.ID == diagnostics.PropertyReceiverMutable {
			count++
			if e.PreviousLine == 0 || e.Help == "" {
				t.Fatal(e)
			}
		}
	}
	if count != 3 {
		t.Fatal(errors)
	}
}

// TestPropertyReceiverBorrowConflicts retains the conservative receiver-wide
// footprint and reports conflicts with still-live field borrows.
// Rules: rules/declarations/properties.md — §8; rules/memory/borrowing.md — §§6–8.
func TestPropertyReceiverBorrowConflicts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/property_receivers/borrow_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 2 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		if e.ID != diagnostics.BorrowMutationConflict && e.ID != diagnostics.BorrowOwnerReadConflict {
			t.Fatal(e)
		}
	}
}
