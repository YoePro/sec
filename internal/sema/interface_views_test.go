package sema

import (
	"os"
	"strings"
	"testing"
)

// Borrowed interface views (MD-057): a concrete type with explicit
// conformance is passed to `ref I` / `ref mut I` without a generic parameter
// or dyn, two calls may pass different concrete types, a `ref mut` view calls
// a mut fn, and `for` iterates a `ref mut Iterator[T]` view with a resolved
// plan whose source is the borrowed view (correction §§ 8.1–8.4).
//
// Rules:
//   - rules/declarations/interfaces.md — § 7.1
//   - rules/control-flow/flowcontrol_for.md — § 37.1
//   - rules/corrections/applied/md057-interface-iterator-parameters-correction-20261017.md — § 8
func TestBorrowedInterfaceViewsAcceptConformingArguments(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/interface_views_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzer(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	views := 0
	for _, plan := range analyzer.resolvedForIterations {
		if plan.Source == ForIteratorBorrowedView {
			views++
			if plan.ElementType.Kind != IntType || !isIteratorInterface(plan.Conformance) {
				t.Fatalf("view plan = %+v", plan)
			}
		}
	}
	if views != 1 {
		t.Fatalf("borrowed iterator view plans = %d", views)
	}
}

// The negative half of the matrix: shared authority never reaches a mut fn
// or a ref mut view, matching member names without `implements` do not
// conform, a shared ref Iterator[T] is not an iteration source, and a by-value
// Iterator[T] parameter is rejected with S1172 instead of becoming an owned
// value or an implicit generic (correction §§ 8.2, 8.6, 8.7, 8.9).
func TestBorrowedInterfaceViewsRejectInvalidUses(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/interface_views_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	for _, want := range []string{
		"method Increment requires mutable receiver",
		"requires ref mut Iterator[int]; a shared ref cannot call mut fn Next",
		"cannot create mutable reference to immutable variable tally",
		"argument 1 to Bump must be ref mut Counter, got ref Tally",
		"argument 1 to Drain must be ref mut Iterator[int], got ref mut Lookalike",
		"parameter \"values\" cannot have interface type Iterator[int] by value",
	} {
		found := false
		for _, err := range errors {
			found = found || strings.Contains(err.Error(), want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, errors)
		}
	}
	for _, err := range errors {
		if strings.Contains(err.Error(), "cannot have interface type") && err.ID != "S1172" {
			t.Errorf("by-value Iterator parameter not S1172: %v", err)
		}
	}
}
