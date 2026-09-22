package sema

import (
	"os"
	"strings"
	"testing"
)

// TestCheckedUnreachableTerminatesAndRecordsReachableEffect verifies that the
// statement satisfies return-path analysis and contributes MayPanic only on a
// reachable path.
//
// Rules:
//   - rules/errors/panic.md — § 16(2)–(6) "Checked unreachable"
func TestCheckedUnreachableTerminatesAndRecordsReachableEffect(t *testing.T) {
	path := "../../testdata/sema/checked_unreachable_valid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errors, []string{"unreachable statement at 9:9"})

	stop := analyzer.CallGraph().EffectSummary(callGraphNodeIDByName(t, analyzer.CallGraph(), "Stop"))
	if !stop.MayPanic || len(stop.DirectEffects) != 1 || stop.DirectEffects[0].Kind != EffectMayPanicUnreachable {
		t.Fatalf("Stop effects = %+v, want one checked-unreachable panic effect", stop)
	}
	dead := analyzer.CallGraph().EffectSummary(callGraphNodeIDByName(t, analyzer.CallGraph(), "DeadPath"))
	if dead.MayPanic || len(dead.DirectEffects) != 0 {
		t.Fatalf("DeadPath effects = %+v, want panic-free proven-dead statement", dead)
	}
}

// TestReachableCheckedUnreachableViolatesNoPanic verifies that @noPanic uses
// the compiler-owned checked-unreachable effect and reports its source.
//
// Rules:
//   - rules/errors/panic.md — § 16(6) "Checked unreachable"
//   - rules/errors/panic.md — § 21 "@noPanic"
func TestReachableCheckedUnreachableViolatesNoPanic(t *testing.T) {
	path := "../../testdata/sema/checked_unreachable_invalid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "function Reject does not satisfy @noPanic") ||
		!strings.Contains(errors[0].Message, string(EffectMayPanicUnreachable)) ||
		!strings.Contains(errors[0].Message, "effect introduced at") {
		t.Fatalf("errors = %+v, want source-aware checked-unreachable @noPanic violation", errors)
	}
}
