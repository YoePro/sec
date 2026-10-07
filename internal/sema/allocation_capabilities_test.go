package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAllocationCapabilitiesDriveMaterialization checks canonical facts against
// actual runtime/folded string plans and mandatory diagnostics at every depth.
// Reanalysis preserves the explicitly selected profile, including unknown;
// unavailable is not confused with unsupported physical allocator capability.
// Rules: rules/memory/allocation.md — §§17(5)-(9),22,29(1),(5);
// rules/platform/target_profiles.md — §§3,31.
func TestAllocationCapabilitiesDriveMaterialization(t *testing.T) {
	file := "../../testdata/sema/allocation_context_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			a := NewAnalyzerWithDepth(depth)
			for _, profile := range []string{"hosted", "embedded-arena", "noalloc", "freestanding", "", "hosted"} {
				a.SetAllocationProfile(profile)
				want := layout.ResolveAllocationCapabilities(profile)
				if a.AllocationCapabilities() != want {
					t.Fatal("profile facts differ", a.AllocationCapabilities(), want)
				}
				p := parser.New(lexer.NewWithFile(string(data), file))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				errors := a.Analyze(program)
				expected := 0
				if !want.HasActiveArenaContext() {
					expected = 2
				}
				if len(errors) != expected {
					t.Fatal(profile, errors)
				}
				for _, diagnostic := range errors {
					if diagnostic.ID != "S1098" {
						t.Fatal(profile, diagnostic)
					}
					if !want.ProfileKnown && !strings.Contains(diagnostic.Message, "capabilities") {
						t.Fatal("unknown reported as known prohibition", diagnostic)
					}
				}
				sites := 0
				for _, fact := range a.AllocationContextFacts() {
					if !fact.ContextKnown {
						continue
					}
					if fact.Context.Profile != profile || fact.Context.Available != want.HasActiveArenaContext() {
						t.Fatal("materialization ignored selected facts", profile, fact)
					}
					sites++
				}
				if sites != 2 {
					t.Fatal("runtime/folded site classification changed", sites)
				}
			}
		})
	}
	// An explicitly incomplete plan must not fall back to unbound hosted analysis.
	a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{})
	if a.AllocationCapabilities().HasActiveArenaContext() {
		t.Fatal("empty explicit plan enabled allocation")
	}
	if !NewAnalyzer().AllocationCapabilities().HasActiveArenaContext() {
		t.Fatal("unbound source-analysis default changed")
	}
}

// TestAllocationProfilesPreserveStaticOperations rejects no ordinary copy,
// borrow, return or folded string merely because dynamic allocation is disabled
// or unresolved. Such source does not require an allocator/context.
// Rules: rules/memory/allocation.md — §§4,17(5),22(3),(6);
// rules/platform/target_profiles.md — §7.
func TestAllocationProfilesPreserveStaticOperations(t *testing.T) {
	file := "../../testdata/sema/allocation_profiles_static_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"hosted", "embedded-arena", "noalloc", "freestanding", ""} {
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithDepth(AnalysisInteractive)
		a.SetAllocationProfile(profile)
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(profile, errors)
		}
		if contexts := a.AllocationContextFacts(); len(contexts) != 0 {
			t.Fatal("static operations acquired contexts", profile, contexts)
		}
		for _, fact := range a.AllocationFacts() {
			if fact.Knowledge != AllocationFree {
				t.Fatal("static operations acquired effects", profile, fact)
			}
		}
	}
}
