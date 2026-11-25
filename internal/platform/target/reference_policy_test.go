package target

import (
	"sec/internal/layout"
	"testing"
)

// MD-140 T06/T07: registered target projection preserves the logical default
// rather than deriving epoch width from the host or target pointer size.
// Rules: rules/memory/reference_model.md §§33–34; rules/memory/arena.md §39(9).
func TestRegisteredArenaEpochPolicies(t *testing.T) {
	widths := map[uint16]bool{}
	for _, definition := range Definitions() {
		if definition.PointerWidthBits == 0 {
			continue
		}
		plan, err := definition.ScalarPlan()
		if err != nil {
			t.Fatal(err)
		}
		policy, err := plan.References.ResolveArenaEpochPolicy()
		if err != nil || policy.LogicalEpochBits != 64 || policy.Exhaustion != layout.ArenaEpochPanic {
			t.Fatalf("%s/%s: %+v, %v", definition.OS, definition.Arch, policy, err)
		}
		widths[plan.PointerWidthBits] = true
	}
	if !widths[32] || !widths[64] {
		t.Fatal("registered plan width coverage missing")
	}
}

// MD-140 T04/T08/T09: custom target policy survives projection; the selected
// profile cannot substitute another exhaustion outcome or omit short safety.
// Rules: rules/platform/target_profiles.md §13; rules/memory/arena.md §39(4),(7),(9).
func TestDefinitionPreservesArenaEpochPolicy(t *testing.T) {
	definition, ok := Find(Target{OS: "linux", Arch: "armv7"})
	if !ok {
		t.Fatal("missing target")
	}
	definition.References = layout.ResolvedReferencePolicy{LogicalEpochBits: 16, ShortWidthProof: "test-target checked exhaustion contract", ArenaShortWidthSafety: layout.ArenaEpochCheckedAdvance}
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan.References != definition.References {
		t.Fatal("definition lost reference policy")
	}
	resolver, err := definition.LayoutResolver()
	if err != nil {
		t.Fatal(err)
	}
	if resolver.Key().LogicalEpochBits != 16 {
		t.Fatal("layout used another epoch policy")
	}
	definition.References.ArenaExhaustion = "rekey"
	if _, err := definition.ScalarPlan(); err == nil {
		t.Fatal("registry accepted rekey")
	}
	definition.References.ArenaExhaustion = layout.ArenaEpochPanic
	definition.References.ArenaShortWidthSafety = ""
	if _, err := definition.ScalarPlan(); err == nil {
		t.Fatal("registry accepted unjustified short width")
	}
	original, _ := Find(Target{OS: "linux", Arch: "armv7"})
	fresh, err := original.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	policy, _ := fresh.References.ResolveArenaEpochPolicy()
	if policy.LogicalEpochBits != 64 {
		t.Fatal("local definition mutation changed registry")
	}
}
