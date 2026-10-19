package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Slice spatial checks contribute canonical bounds panic; try removes only
// that operation's spatial effect and cannot suppress stale-reference panic.
// Proven/open ranges neither allocate nor block through borrowing/validation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§118–120.
func TestSliceRangeEffects(t *testing.T) {
	const file = "../../testdata/sema/slice_effects_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errs := a.Analyze(program)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	g := a.CallGraph()
	for _, name := range []string{"Ordinary", "UnsafeRange"} {
		summary := g.EffectSummary(callGraphNodeIDByName(t, g, name))
		if !summary.MayPanic || len(summary.DirectEffects) != 1 || summary.DirectEffects[0].Kind != EffectMayPanicBounds || len(summary.DirectEffects[0].PanicReasonIDs) != 1 || summary.DirectEffects[0].PanicReasonIDs[0] != diagnostics.PanicReasonBoundsFailure {
			t.Fatalf("%s: %+v", name, summary)
		}
	}
	for _, name := range []string{"Handled", "Proven", "Open", "Fallible", "Mixed", "Copy", "Move"} {
		if summary := g.EffectSummary(callGraphNodeIDByName(t, g, name)); summary.MayPanic {
			t.Fatalf("%s unexpectedly panics: %+v", name, summary)
		}
	}
	for _, name := range []string{"Ordinary", "Handled", "Proven", "Open", "Fallible", "Mixed", "Copy", "Move"} {
		id := callGraphNodeIDByName(t, g, name)
		alloc, block := g.ArenaSummary(id), g.BlockSummary(id)
		if alloc.MayAllocate || alloc.AllocationUnknown || block.MayBlock {
			t.Fatalf("slice operation invented allocation/blocking in %s: %+v %+v", name, alloc, block)
		}
	}
	if summary := g.EffectSummary(callGraphNodeIDByName(t, g, "Operand")); !summary.MayPanic || len(summary.PanicCauses) == 0 {
		t.Fatal("try removed operand-call panic")
	}
	for _, name := range []string{"Dynamic", "DynamicHandled", "DynamicLength", "DynamicGlobalLength", "DynamicIndex"} {
		found := false
		for _, site := range g.EffectSummary(callGraphNodeIDByName(t, g, name)).DirectEffects {
			if site.Kind == EffectMayPanicInvalidReference {
				found = true
			}
			if (name == "DynamicHandled" || name == "DynamicIndex") && site.Kind == EffectMayPanicBounds {
				t.Fatal("try retained its spatial panic")
			}
		}
		if !found {
			t.Fatalf("%s lost temporal validity panic", name)
		}
	}
}

// Attribute and ISR diagnostics consume the same canonical effects. Invalid
// propagation cannot turn an unhandled range into a false no-panic proof.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§118–120;
// rules/errors/panic.md — §§21–22,25.
func TestSliceEffectsAttributeAndISRDiagnostics(t *testing.T) {
	const file = "../../testdata/sema/slice_effects_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errs := a.Analyze(program)
	want := map[string]string{"Spatial": "BoundsFailure", "WrongChannel": "BoundsFailure", "Temporal": "InvalidReferenceGeneration", "ISRRange": "BoundsFailure"}
	wrongChannel := false
	for _, e := range errs {
		if strings.Contains(e.Message, "RangeError") && e.ID != diagnostics.NoPanicViolation {
			wrongChannel = true
		}
		if e.ID != diagnostics.NoPanicViolation {
			continue
		}
		for name, reason := range want {
			if strings.Contains(e.Message, "function "+name+" ") {
				if reason == "BoundsFailure" && !strings.Contains(e.Message, string(EffectMayPanicBounds)) || reason == "InvalidReferenceGeneration" && !strings.Contains(e.Help, reason) {
					t.Fatalf("wrong panic explanation: %v", e)
				}
				delete(want, name)
				break
			}
		}
		if strings.Contains(e.Message, "function ISRSafe ") {
			t.Fatal("handled ISR range still panics", e)
		}
	}
	if len(want) != 0 || !wrongChannel {
		t.Fatalf("missing canonical diagnostics %v, channel rejection=%v: %v", want, wrongChannel, errs)
	}
}
