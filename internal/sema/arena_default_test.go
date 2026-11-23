package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeArenaDefaults(t *testing.T, profile, source string) (*Analyzer, []string) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "arena_default.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	a.SetAllocationProfile(profile)
	var messages []string
	for _, err := range a.Analyze(program) {
		messages = append(messages, err.Error())
	}
	return a, messages
}

// MD-136 (arena.md § 131, default_values.md § 65B), T01–T04: both default
// forms type-check as the same empty default creation with a fresh domain
// each; they are legal under noalloc and in @noAlloc code; Reset and a
// zero-element Alloc need no backing; positive-storage New/Alloc on the
// default is a compile-time error where the frozen plan cannot grow it
// (noalloc, freestanding); under the interim hosted mapping the default
// freezes a growable policy pending MD-151; `Arena {}` takes no fields.
func TestArenaDefaultConstruction(t *testing.T) {
	valid := `module main

@noAlloc
fn Empty() int {
    let mut first: Arena
    first.Reset()
    return 0
}

fn Zero() Result[int64, AllocationError] {
    let mut second := Arena {}
    let none := try second.Alloc[int32](0)
    return Ok(0)
}

fn main() int {
    return 0
}
`
	for _, profile := range []string{"hosted", "noalloc"} {
		a, errs := analyzeArenaDefaults(t, profile, valid)
		if len(errs) != 0 {
			t.Fatalf("%s: %v", profile, errs)
		}
		domains := map[InvalidationDomainID]bool{}
		growth, provider := "fixed", "absent"
		if profile == "hosted" {
			growth, provider = "growable-nonrelocating", defaultArenaInterimProvider
		}
		for _, plan := range a.arenaDefaultPlans {
			if plan.Constructor != ArenaDefaultConstructor || plan.BackingKind != "none" || plan.GrowthKind != growth ||
				plan.CapacityPlan != "empty-default" || plan.ProviderPlan != provider || plan.StateAfter != 1 ||
				plan.Capacity.BackingBytes == nil || plan.Capacity.BackingBytes.Sign() != 0 || len(plan.Unresolved) != 0 {
				t.Fatalf("%s: default plan %+v", profile, plan)
			}
			domains[plan.InvalidationDomain] = true
		}
		if len(a.arenaDefaultPlans) != 2 || len(domains) != 2 {
			t.Fatalf("%s: %d default plans over %d domains, want 2 distinct", profile, len(a.arenaDefaultPlans), len(domains))
		}
	}
	for name, test := range map[string]struct{ profile, body, want string }{
		"New on noalloc default":   {"noalloc", "    let mut arena: Arena\n    let one := try arena.New[int64]()\n", "needs positive backing"},
		"Alloc on unknown default": {"freestanding", "    let mut arena := Arena {}\n    let values := try arena.Alloc[int32](4)\n", "needs positive backing"},
		"fields not permitted":     {"hosted", "    let mut arena := Arena { capacity: 4 }\n", "Arena {} takes no fields"},
	} {
		t.Run(name, func(t *testing.T) {
			source := "module main\n\nfn F() Result[int64, AllocationError] {\n" + test.body + "    return Ok(0)\n}\n\nfn main() int {\n    return 0\n}\n"
			_, errs := analyzeArenaDefaults(t, test.profile, source)
			if !strings.Contains(strings.Join(errs, "\n"), test.want) {
				t.Fatalf("errors %v, want %q", errs, test.want)
			}
		})
	}
}
