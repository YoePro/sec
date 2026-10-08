package temporal_test

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// TestClockReadsAreForbiddenInRequiredContractAndDefaultPositions exercises
// every modeled contract/default CTE position, projections and call arguments.
// Rules: rules/types/temporal.md — §3;
// rules/corrections/applied/temporal-now-correction-20260928.md — §7;
// rules/compiler/compile_time_evaluation.md — §§4(4),8(5).
func TestClockReadsAreForbiddenInRequiredContractAndDefaultPositions(t *testing.T) {
	template := fixture(t, "clock_positions.sec.in")
	for _, clock := range []string{"_now", "datetime.Now", "date.Today", "time.Now", "_now._epochDays", "Echo(datetime.Now)", "datetime.Now._epochDays", "1 + datetime.Now._epochDays", "-(_now._epochDays)"} {
		t.Run(clock, func(t *testing.T) {
			a, _, errors := analyzeTemporalCoreSource(t, strings.ReplaceAll(template, "__CLOCK__", clock))
			if len(errors) != 10 {
				t.Fatalf("errors = %v, want 10 forbidden positions", errors)
			}
			for _, err := range errors {
				if err.ID == diagnostics.SemanticCompileTimeExecutionUnavailable || !strings.Contains(err.Message, "current UTC wall-clock access is forbidden") || err.Help == "" {
					t.Errorf("clock classified as a permitted or unknown CTE operation: %+v", err)
				}
			}
			if !a.Types()["Defaulted"].InvalidExplicitDefault || sema.IsDefaultable(a.Types()["Defaulted"]) {
				t.Fatal("invalid clock default fell back to an implicit value")
			}
		})
	}
}

// TestClockRestrictionFollowsImmutableBindings covers the shared static/CTE
// boundary: binding a runtime clock value cannot make it a CTE constant.
// Rules: rules/types/temporal.md — §3;
// rules/corrections/applied/temporal-now-correction-20260928.md — §7.
func TestClockRestrictionFollowsImmutableBindings(t *testing.T) {
	_, _, errors := analyzeTemporalCoreSource(t, fixture(t, "clock_binding_invalid.sec"))
	if len(errors) != 3 {
		t.Fatalf("errors = %v, want a static and two default failures", errors)
	}
	clockErrors := 0
	for _, err := range errors {
		if strings.Contains(err.Message, "current UTC wall-clock access is forbidden") {
			clockErrors++
		}
	}
	if clockErrors != 2 {
		t.Fatalf("binding clock errors = %v", errors)
	}
}

// TestUserGetterNamesRemainLegalSemanticCTE preserves the separate missing
// executor outcome for deterministic user properties named Now or Today.
// Rules: rules/types/temporal.md — §3 (canonical intrinsic identities);
// rules/compiler/compile_time_evaluation.md — §§4(4),8(5).
func TestUserGetterNamesRemainLegalSemanticCTE(t *testing.T) {
	errors := analyzeSourceRaw(t, fixture(t, "deterministic_getters.sec"))
	if len(errors) != 3 {
		t.Fatalf("errors = %v, want three unavailable execution errors", errors)
	}
	for _, err := range errors {
		if err.ID != diagnostics.SemanticCompileTimeExecutionUnavailable {
			t.Errorf("user getter misclassified: %+v", err)
		}
	}
}
