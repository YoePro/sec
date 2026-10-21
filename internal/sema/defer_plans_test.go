package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// P17-20: a deferred body retains binding/Place identities and the required
// action lifetime, preserving delayed reads rather than taking value snapshots.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§42–44,79;
// rules/control-flow/defer.md — §§9–11,27,29.
func TestResolvedDeferCapturePlans(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "defer_captures.sec")
	plans := map[string]ResolvedDeferPlan{}
	for statement := range a.resolvedDeferPlans {
		plan, ok := a.ResolvedDeferPlanOf(statement)
		if !ok || plan.Identity.Action == 0 || plan.Invocation == "" || plan.Body == "" || plan.Invocation == plan.Body {
			t.Fatalf("missing defer identity/lifetime: %+v", plan)
		}
		name := a.callGraph.nodes[plan.Invocation].Name
		plans[name] = plan
		for _, capture := range plan.Captures {
			if capture.Binding == 0 || capture.Place.RootID == 0 || capture.RequiredUntil != plan.Identity.Action {
				t.Fatalf("capture has no binding, Place or required lifetime: %+v", capture)
			}
			binding, ok := a.ResolvedBindingDeclaredAt(capture.Place.RootToken.File, capture.Place.RootToken.Line, capture.Place.RootToken.Column)
			if !ok || binding.ID != capture.Binding {
				t.Fatal("capture binding was reconstructed from its display name")
			}
		}
	}
	want := map[string][]string{
		"Late": {"value"}, "Fields": {"pair.first"}, "Index": {"values[2]"},
		"Holder": {"view", "pair.first"}, "Empty": nil, "Other": {"value"}, "Parameter": {"value"},
	}
	if len(plans) != len(want) {
		t.Fatalf("plans = %v, want %d", plans, len(want))
	}
	for name, paths := range want {
		plan, ok := plans[name]
		if !ok {
			t.Errorf("missing %s plan", name)
			continue
		}
		var got []string
		for _, capture := range plan.Captures {
			got = append(got, capture.Place.String())
		}
		if !reflect.DeepEqual(got, paths) {
			t.Errorf("%s captures = %v, want %v", name, got, paths)
		}
	}
	if plans["Late"].Captures[0].Binding == plans["Other"].Captures[0].Binding || plans["Late"].Invocation == plans["Other"].Invocation {
		t.Fatal("same-spelled bindings in different invocations were merged")
	}
	holder := plans["Holder"].Captures
	if len(holder) == 2 && (holder[0].Kind != DeferCaptureReferenceHolder || holder[1].Kind != DeferCaptureValueUse) {
		t.Fatal("reference holder authority was conflated with a late value read")
	}
	if plans["Parameter"].Captures[0].Place.RootKind != PlaceRootParameter {
		t.Fatal("parameter capture lost its canonical root kind")
	}
}

// Queries must detach exact indices and nested types and must not repeat
// inference, allocate identities, publish transfers or change borrow/move facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§42–43,79.
func TestResolvedDeferPlansDetachedAndReadOnly(t *testing.T) {
	a, program := analyzeDeferPlanFixture(t, "defer_captures.sec")
	reference, _ := analyzeDeferPlanFixture(t, "defer_captures.sec")
	want := map[DeferID]ResolvedDeferPlan{}
	for _, plan := range reference.resolvedDeferPlans {
		want[plan.Identity.Defer] = plan
	}
	beforeTransfers := a.ResolvedTransferPlans()
	beforeMoves := copyMoved(a.moved)
	beforeBorrows := copyBorrows(a.borrows)
	beforeTypes, beforePlaces := len(a.expressionTypes), len(a.resolvedPlaces)
	beforeActions := len(a.cleanupIdentities.actions)
	for stmt, stored := range a.resolvedDeferPlans {
		plan, ok := a.ResolvedDeferPlanOf(stmt)
		if !ok || !reflect.DeepEqual(plan, stored) {
			t.Fatal("query changed capture facts")
		}
		for index := range plan.Captures {
			capture := &plan.Captures[index]
			capture.Place.Root = "changed"
			capture.Binding = 0
			capture.RequiredUntil = 0
			if capture.Place.Type.MinInteger != nil {
				capture.Place.Type.MinInteger.SetInt64(999)
			}
			if element := capture.Place.Type.Element; element != nil {
				element.Name = "changed"
				if len(element.Fields) != 0 {
					element.Fields[0].Name = "changed"
				}
			}
			for _, projection := range capture.Place.Projections {
				if projection.ConstantIndex != nil {
					projection.ConstantIndex.SetInt64(999)
				}
			}
		}
		again, _ := a.ResolvedDeferPlanOf(stmt)
		if !reflect.DeepEqual(again, want[stored.Identity.Defer]) {
			t.Fatal("query aliases nested capture data")
		}
	}
	if _, ok := a.ResolvedDeferPlanOf(&ast.DeferStatement{}); ok {
		t.Fatal("unknown defer acquired a plan")
	}
	if beforeTypes != len(a.expressionTypes) || beforePlaces != len(a.resolvedPlaces) || beforeActions != len(a.cleanupIdentities.actions) ||
		!reflect.DeepEqual(beforeTransfers, a.ResolvedTransferPlans()) || !reflect.DeepEqual(beforeMoves, a.moved) || !reflect.DeepEqual(beforeBorrows, a.borrows) {
		t.Fatal("query changed analysis state")
	}
	old := a.resolvedDeferPlans
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	if !reflect.DeepEqual(old, a.resolvedDeferPlans) {
		t.Fatal("reanalysis changed the resolved plans")
	}
}

// A rejected body has no consumable capture plan. Reanalysis also drops facts
// from the previous snapshot, even when that snapshot was valid.
// Rules: rules/control-flow/defer.md — §16;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§45,79.
func TestResolvedDeferPlansRejectedAndReset(t *testing.T) {
	a, _ := analyzeDeferPlanFixture(t, "defer_captures.sec")
	source, err := os.ReadFile("../../testdata/sema/cleanup_identities_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	if errors := a.Analyze(program); len(errors) == 0 || len(a.resolvedDeferPlans) != 0 {
		t.Fatal("invalid defer retained a capture plan")
	}
}

// Published lifetime requirements must match the canonical borrow checks:
// overlapping ownership end and retained-holder replacement are forbidden.
// The positive Fields fixture separately proves a disjoint sibling can move.
// Rules: rules/memory/lifetime_analysis.md — §17.1–§17.2;
// rules/memory/borrowing.md — §21(2), §21(4)–(5).
func TestResolvedDeferCaptureLifetimeEnforcement(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/defer_capture_lifetimes_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	if len(errors) != 2 || !strings.Contains(errors[0].Message, "cannot move pair.first while it is required by defer") ||
		!strings.Contains(errors[1].Message, "cannot replace view while it is required as a reference holder by defer") {
		t.Fatalf("unexpected lifetime diagnostics: %v", errors)
	}
	if len(a.resolvedDeferPlans) != 2 {
		t.Fatal("valid deferred bodies lost their lifetime plans")
	}
}

// analyzeDeferPlanFixture analyzes repository test sources with the real
// frontend before examining their immutable defer facts.
// Rules: rules/control-flow/defer.md — §29.
func analyzeDeferPlanFixture(t *testing.T, filename string) (*Analyzer, *ast.Program) {
	t.Helper()
	source, err := os.ReadFile("../../testdata/sema/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	return a, program
}
