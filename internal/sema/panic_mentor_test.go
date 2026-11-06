package sema

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// Panic cleanup cannot be repaired by removing a guarantee or by returning an
// error from an invocation already exiting; diagnostics must explain local repair.
// Rules: rules/errors/panic.md — §§11,19,28; rules/control-flow/defer.md — §§14–18.
func TestPanicCleanupMentorDiagnostics(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/panic_mentor_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errs := analyzeSourceWithAnalyzer(t, string(data))
	count := 0
	counts := map[string]int{}
	for _, e := range errs {
		counts[e.ID]++
		if e.ID == "" || e.Help == "" {
			t.Errorf("anonymous diagnostic: %+v", e)
		}
		if e.ID == diagnostics.TryPropagationIncompatible || e.ID == diagnostics.TryResidualUnpropagatable {
			if !strings.Contains(e.Help, "already-exiting") || strings.Contains(e.Help, "or return a Result") {
				t.Errorf("illegal propagation advice: %+v", e)
			}
		}
		if e.ID == diagnostics.NoPanicViolation {
			count++
			if strings.Contains(e.Message, string(EffectMayPanicForeign)) && (!strings.Contains(e.Help, "trusted") || !strings.Contains(e.Help, "cannot abort or unwind")) {
				t.Errorf("missing foreign proof boundary: %+v", e)
			}
			if strings.Contains(e.Message, "Deferred") || strings.Contains(e.Message, "Resource.free") {
				if !strings.Contains(strings.ToLower(e.Help), "cleanup") || strings.Contains(e.Help, "remove @noPanic") {
					t.Errorf("unsafe cleanup advice: %+v", e)
				}
			}
		}
		if strings.Contains(e.Message, "not allowed inside defer") && e.Help == "" {
			t.Errorf("missing cleanup repair: %+v", e)
		}
	}
	if count != 7 {
		t.Fatalf("expected seven panic causes: %v", errs)
	}
	if counts[diagnostics.InvalidDeferredCleanup] != 8 || counts[diagnostics.TryPropagationIncompatible] != 2 || counts[diagnostics.TryResidualUnpropagatable] != 2 || len(errs) != 19 {
		t.Fatal("missing diagnostic groups", counts, errs)
	}
}

// Locally handled errors and independently returning nested invocations remain
// valid cleanup; trusted foreign proof stays distinct from unknown behavior.
// Rules: rules/errors/panic.md — §§11,19,28; rules/control-flow/defer.md — §§14,16.
func TestPanicCleanupMentorValid(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/panic_mentor_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errs := analyzeSourceWithAnalyzer(t, string(data))
	if len(errs) != 0 {
		t.Fatal(errs)
	}
}
