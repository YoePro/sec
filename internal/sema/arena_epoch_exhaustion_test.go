package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
)

const arenaEpochSource = `module main

fn Cycle() void {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    arena.Reset()
}

fn Plain(value: int32) int32 {
    return value
}

fn main() int {
    Cycle()
    return 0
}
`

func analyzeArenaEpoch(t *testing.T, source string, plan layout.ResolvedScalarPlan) (*Analyzer, *ast.Program, []Error) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "epoch.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzerWithScalarPlan(plan)
	return a, program, a.Analyze(program)
}

func arenaResetPlans(a *Analyzer) []ResolvedArenaOperationPlan {
	var plans []ResolvedArenaOperationPlan
	for call := range a.arenaOperationPlans {
		if plan, ok := a.ResolvedArenaResetPlanOf(call); ok {
			plans = append(plans, plan)
		}
	}
	return plans
}

// TestArenaResetEpochAdvancePlan: every reachable Reset publishes the checked
// epoch advance with the plan's logical width and the single panic outcome,
// 64 bits on 64- and 32-bit targets alike and the selected width on an
// approved short-width plan (T06-T08, T26); no proof form is invented.
// Rules: rules/memory/arena.md — §§ 38(8), 39(1), 39(5)-(9); MD-140 §§ 3, 5, 7.
func TestArenaResetEpochAdvancePlan(t *testing.T) {
	for _, tc := range []struct {
		name  string
		arch  string
		short bool
		bits  uint16
	}{
		{"64-bit target", "amd64", false, 64},
		{"32-bit target", "armv7", false, 64},
		{"approved short width", "armv7", true, 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definition, _ := target.Find(target.Target{OS: "linux", Arch: tc.arch})
			plan, err := definition.ScalarPlan()
			if err != nil {
				t.Fatal(err)
			}
			if tc.short {
				plan.References = layout.ResolvedReferencePolicy{LogicalEpochBits: 16, ShortWidthProof: "test-only checked advance", ArenaShortWidthSafety: layout.ArenaEpochCheckedAdvance}
			}
			a, _, errors := analyzeArenaEpoch(t, arenaEpochSource, plan)
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			plans := arenaResetPlans(a)
			if len(plans) != 1 {
				t.Fatalf("Reset plans: %d", len(plans))
			}
			want := ArenaEpochAdvancePlan{Form: ArenaEpochAdvanceChecked, LogicalEpochBits: tc.bits, Exhaustion: layout.ArenaEpochPanic, PanicReason: diagnostics.PanicReasonArenaEpochExhaustion}
			if plans[0].EpochAdvance != want {
				t.Fatalf("epoch advance %+v, want %+v", plans[0].EpochAdvance, want)
			}
			for _, missing := range plans[0].PhysicalUnresolved {
				if strings.Contains(missing, "rekey") {
					t.Fatalf("rekey strategy left open: %v", plans[0].PhysicalUnresolved)
				}
			}
		})
	}
}

// TestArenaResetExhaustionEffect: an unproven Reset contributes MayPanic with
// the registered panic.arena-epoch-exhaustion reason; a callable without one
// has no such cause; an unresolvable epoch policy leaves the plan unresolved
// and still keeps the potential panic.
// Rules: rules/analysis/effect_analysis.md — § 23(6); rules/memory/arena.md — § 39(8).
func TestArenaResetExhaustionEffect(t *testing.T) {
	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	plan, _ := definition.ScalarPlan()
	a, _, errors := analyzeArenaEpoch(t, arenaEpochSource, plan)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	graph := a.CallGraph()
	summary := graph.EffectSummary(callGraphNodeIDByName(t, graph, "Cycle"))
	found := false
	for _, effect := range summary.DirectEffects {
		if effect.Kind == EffectMayPanicArenaEpochExhaustion {
			found = len(effect.PanicReasonIDs) == 1 && effect.PanicReasonIDs[0] == diagnostics.PanicReasonArenaEpochExhaustion
		}
	}
	if !found || !summary.MayPanic {
		t.Fatalf("Cycle effects: %+v", summary)
	}
	if plain := graph.EffectSummary(callGraphNodeIDByName(t, graph, "Plain")); plain.MayPanic {
		t.Fatalf("Plain: %+v", plain)
	}
	if main := graph.EffectSummary(callGraphNodeIDByName(t, graph, "main")); !main.MayPanic {
		t.Fatal("the potential panic must propagate to callers")
	}

	unresolved := NewAnalyzerWithScalarPlan(plan)
	unresolved.referencePolicy = layout.ResolvedReferencePolicy{LogicalEpochBits: 16, ShortWidthProof: "generic retirement only"}
	p := parser.New(lexer.NewWithFile(arenaEpochSource, "epoch.sec"))
	if errors := unresolved.Analyze(p.ParseProgram()); len(errors) != 0 {
		t.Fatal(errors)
	}
	plans := arenaResetPlans(unresolved)
	if len(plans) != 1 || plans[0].EpochAdvance != (ArenaEpochAdvancePlan{}) || !strings.Contains(strings.Join(plans[0].Unresolved, ";"), "Arena epoch policy") {
		t.Fatalf("unresolved policy must stay named: %+v", plans)
	}
	if summary := unresolved.CallGraph().EffectSummary(callGraphNodeIDByName(t, unresolved.CallGraph(), "Cycle")); !summary.MayPanic {
		t.Fatal("an unresolved policy must not drop the potential panic")
	}
}

// TestArenaResetExhaustionSourceRules: @noPanic rejects an unproven Reset and
// explains it with the reason (T33), the 64-bit default is no proof, and try
// offers no recovery channel for Reset (T37).
// Rules: rules/analysis/effect_analysis.md — § 26(4); rules/errors/runtime_checks.md — § 11(5).
func TestArenaResetExhaustionSourceRules(t *testing.T) {
	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	plan, _ := definition.ScalarPlan()
	noPanic := strings.Replace(arenaEpochSource, "fn Cycle() void {", "@noPanic\nfn Cycle() void {", 1)
	_, _, errors := analyzeArenaEpoch(t, noPanic, plan)
	joined := ""
	for _, err := range errors {
		joined += err.Message + " " + err.Help + "\n"
	}
	if len(errors) == 0 || !strings.Contains(joined, "panic.arena-epoch-exhaustion") || !strings.Contains(joined, "64-bit width is not such a proof") {
		t.Fatalf("@noPanic must reject an unproven Reset with its reason: %v", errors)
	}
	recovery := strings.Replace(arenaEpochSource, "    arena.Reset()\n", "    try arena.Reset()\n", 1)
	if _, _, errors := analyzeArenaEpoch(t, recovery, plan); len(errors) == 0 {
		t.Fatal("try arena.Reset() must not be a recovery form")
	}
}
