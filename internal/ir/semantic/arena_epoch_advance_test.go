package semantic

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
	"sec/internal/sema"
)

// testEpochAdvance is the checked 64-bit Reset advance Sema publishes today.
func testEpochAdvance() ArenaEpochAdvanceFacts {
	reason, _ := diagnostics.PanicReasonByID(diagnostics.PanicReasonArenaEpochExhaustion)
	return ArenaEpochAdvanceFacts{Form: string(sema.ArenaEpochAdvanceChecked), LogicalEpochBits: 64, Exhaustion: string(layout.ArenaEpochPanic), PanicReasonID: uint32(reason.ID), PanicReason: reason.Name}
}

// TestArenaResetEpochAdvanceVerifier: the Semantic IR verifier accepts the
// checked advance and the two proof-carrying elimination forms, and rejects
// a missing form, a checked advance with a proof, an elimination without
// proof, an unresolved width, a rekey or fallible outcome, another or a
// mismatched panic reason, and the fact on a non-Reset op (T43, T44).
// Rules: rules/compiler/semantic_ir.md — § 51(6)-(8); rules/errors/panic.md — § 13(8).
func TestArenaResetEpochAdvanceVerifier(t *testing.T) {
	valid := map[string]func(*ArenaEpochAdvanceFacts){
		"checked":       func(*ArenaEpochAdvanceFacts) {},
		"short checked": func(e *ArenaEpochAdvanceFacts) { e.LogicalEpochBits = 16 },
		"proven bounded": func(e *ArenaEpochAdvanceFacts) {
			e.Form, e.Proof = string(sema.ArenaEpochAdvanceProvenBounded), "bound:domain#5<=3"
		},
		"metadata eliminated": func(e *ArenaEpochAdvanceFacts) {
			e.Form, e.Proof = string(sema.ArenaEpochAdvanceMetadataEliminated), "no-runtime-handle"
		},
	}
	for name, change := range valid {
		m, op := arenaOperationFixture(OpArenaReset, 64)
		change(&op.Arena.EpochAdvance)
		if err := Verify(m); err != nil {
			t.Fatalf("%s rejected: %v", name, err)
		}
	}
	invalid := map[string]func(*ArenaEpochAdvanceFacts){
		"missing form":          func(e *ArenaEpochAdvanceFacts) { e.Form = "" },
		"unknown form":          func(e *ArenaEpochAdvanceFacts) { e.Form = "unchecked" },
		"checked with proof":    func(e *ArenaEpochAdvanceFacts) { e.Proof = "assumed" },
		"bounded without proof": func(e *ArenaEpochAdvanceFacts) { e.Form = string(sema.ArenaEpochAdvanceProvenBounded) },
		"eliminated no proof":   func(e *ArenaEpochAdvanceFacts) { e.Form = string(sema.ArenaEpochAdvanceMetadataEliminated) },
		"no width":              func(e *ArenaEpochAdvanceFacts) { e.LogicalEpochBits = 0 },
		"odd width":             func(e *ArenaEpochAdvanceFacts) { e.LogicalEpochBits = 48 },
		"rekey outcome":         func(e *ArenaEpochAdvanceFacts) { e.Exhaustion = "rekey" },
		"fallible outcome":      func(e *ArenaEpochAdvanceFacts) { e.Exhaustion = "fallible" },
		"stale reference reason": func(e *ArenaEpochAdvanceFacts) {
			e.PanicReasonID, e.PanicReason = uint32(diagnostics.PanicReasonInvalidReferenceGeneration), "InvalidReferenceGeneration"
		},
		"mismatched reason name": func(e *ArenaEpochAdvanceFacts) { e.PanicReason = "ArenaEpochExhaustion" },
	}
	for name, change := range invalid {
		m, op := arenaOperationFixture(OpArenaReset, 64)
		change(&op.Arena.EpochAdvance)
		if err := Verify(m); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	m, op := arenaOperationFixture(OpArenaNew, 64)
	op.Arena.EpochAdvance = testEpochAdvance()
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "epoch-advance") {
		t.Fatalf("epoch-advance fact on arena.new accepted: %v", err)
	}
}

const epochAdvanceSource = `module main

fn Cycle() void {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    arena.Reset()
}

fn main() int {
    Cycle()
    return 0
}
`

func buildEpochAdvanceSource(t *testing.T, references layout.ResolvedReferencePolicy) (*Module, error) {
	t.Helper()
	definition, _ := target.Find(target.Target{OS: "linux", Arch: "armv7"})
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	plan.References = references
	parsed := parser.New(lexer.NewWithFile(epochAdvanceSource, "epoch.sec")).Parse()
	a := sema.NewAnalyzerWithScalarPlan(plan)
	a.SetAllocationProfile("hosted")
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	return Build(parsed.Program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{"epoch.sec"}, MaxPackage: 19})
}

// TestArenaResetEpochAdvanceFromSource: a source Reset on a 32-bit target
// lowers with the checked 64-bit advance and the registered reason, printed
// on the op; an unresolved Arena epoch policy fails closed instead of an
// unchecked Reset (T07, T43).
// Rules: rules/compiler/semantic_ir.md — § 51(6)-(8); rules/memory/arena.md — § 39(9).
func TestArenaResetEpochAdvanceFromSource(t *testing.T) {
	module, err := buildEpochAdvanceSource(t, layout.ResolvedReferencePolicy{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpArenaReset {
					found = op.Arena.EpochAdvance == testEpochAdvance()
				}
			}
		}
	}
	if !found {
		t.Fatal("Reset lacks the checked 64-bit epoch advance")
	}
	if text := Format(module); !strings.Contains(text, "epoch-advance=checked width=64 exhaustion=panic reason=panic.arena-epoch-exhaustion#12") {
		t.Fatalf("printed Reset:\n%s", text)
	}
	if _, err := buildEpochAdvanceSource(t, layout.ResolvedReferencePolicy{LogicalEpochBits: 16, ShortWidthProof: "generic retirement only"}); err == nil || !strings.Contains(err.Error(), "Arena epoch policy") {
		t.Fatalf("unresolved epoch policy must fail closed: %v", err)
	}
}
