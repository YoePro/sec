package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestISRPanicUsesCanonicalSummaries verifies only the panic portion of the ISR
// contract. Unsupported ISR attributes remain rejected; no broader ISR proof or
// target binding is fabricated from a panic-free summary.
// Rules: rules/errors/panic.md — §§22,25,28(5);
// rules/platform/interrupts.md — §§24–29,43.
func TestISRPanicUsesCanonicalSummaries(t *testing.T) {
	const file = "../../testdata/sema/isr_panic_paths_invalid.sec"
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
	errors := a.Analyze(program)
	want := map[string]struct {
		root, cause int
		kind        EffectKind
	}{
		"Direct":      {5, 6, EffectMayPanicAssertion},
		"Transitive":  {7, 2, EffectMayPanicExplicit},
		"Helper":      {9, 2, EffectMayPanicExplicit},
		"UnsafeRoot":  {11, 2, EffectMayPanicExplicit},
		"Deferred":    {14, 2, EffectMayPanicExplicit},
		"Unknown":     {16, 17, EffectMayPanicUnknownCallee},
		"ForeignRoot": {18, 19, EffectMayPanicForeign},
	}
	unsupported := 0
	for _, e := range errors {
		if strings.Contains(e.Message, "not implemented yet") {
			unsupported++
		}
		if e.ID != diagnostics.NoPanicViolation {
			continue
		}
		found := false
		for name, expected := range want {
			if !strings.Contains(e.Message, "function "+name+" ") {
				continue
			}
			found = true
			if e.Line != expected.cause || e.PreviousLine != expected.root || e.File != file || e.PreviousFile != file || !strings.Contains(e.Message, string(expected.kind)) || !strings.Contains(e.Help, "unsafe does not waive") || strings.Contains(e.Help, "remove @noPanic") {
				t.Fatal(name, e)
			}
			g := a.CallGraph()
			summary := g.EffectSummary(callGraphNodeIDByName(t, g, name))
			if !summary.MayPanic || len(summary.PanicCauses) == 0 {
				t.Fatal("ISR has a parallel/incomplete panic summary", summary)
			}
			delete(want, name)
			break
		}
		if !found {
			t.Fatal("unexpected ISR panic violation", e)
		}
	}
	if len(want) != 0 || unsupported != 10 {
		t.Fatal("missing panic proof or unsupported-attribute guard", want, errors)
	}
	for _, name := range []string{"Proven", "Handled", "ContractProven"} {
		if summary := a.CallGraph().EffectSummary(callGraphNodeIDByName(t, a.CallGraph(), name)); summary.MayPanic {
			t.Fatal(name, summary)
		}
	}
}
