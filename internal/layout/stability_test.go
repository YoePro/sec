package layout

import (
	"math/big"
	"testing"
)

// Complete native aggregates are not stable just because their primitive
// fields are; wrappers/arrays retain dependency scope, and unsupported contracts
// and incomplete target facts are never positive stability evidence.
// Rules: rules/memory/layout.md §§33-35,39.
func TestResolvedLayoutStability(t *testing.T) {
	integer := scalar("int32", ScalarSigned, 32, false)
	aggregate := LayoutRequirement{TypeIdentity: "Pair", Kind: LayoutStruct, Fields: []FieldRequirement{{Name: "value", Requirement: integer}}}
	named := func(identity string, carrier LayoutRequirement) LayoutRequirement {
		return LayoutRequirement{TypeIdentity: identity, Kind: LayoutNamed, Carrier: &carrier}
	}
	array := func(identity string, element LayoutRequirement) LayoutRequirement {
		return LayoutRequirement{TypeIdentity: identity, Kind: LayoutArray, Element: &element, Count: big.NewInt(2)}
	}
	explicit := integer
	explicit.Contract = "requested-but-not-implemented"
	for _, test := range []struct {
		name        string
		requirement LayoutRequirement
		want        LayoutStability
	}{
		{"scalar", integer, LayoutTargetStable},
		{"decimal", scalar("decimal", ScalarDecimal, 64, false), LayoutTargetStable},
		{"native struct", aggregate, LayoutNativeUnstable},
		{"named scalar", named("Count", integer), LayoutTargetStable},
		{"named aggregate", named("WrappedPair", aggregate), LayoutNativeUnstable},
		{"scalar array", array("int32[2]", integer), LayoutTargetStable},
		{"native array", array("Pair[2]", aggregate), LayoutNativeUnstable},
		{"closed enum", LayoutRequirement{TypeIdentity: "Flag", Kind: LayoutEnum, Carrier: &integer, EnumMembers: []EnumMemberRequirement{{Name: "A", Value: big.NewInt(0)}}}, LayoutTargetStable},
		{"unsupported contract", explicit, LayoutStabilityUnknown},
		{"unsupported descriptor", LayoutRequirement{TypeIdentity: "string", Kind: LayoutDescriptor}, LayoutStabilityUnknown},
		{"invalid scalar", scalar("bad", ScalarSigned, 3, false), LayoutStabilityUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, err := NewResolver(layoutTestPlan(64), testAlignments())
			if err != nil {
				t.Fatal(err)
			}
			got := r.Resolve(test.requirement)
			if got.Stability != test.want || got.StabilityReason == "" || got.PlanKey != r.Key() {
				t.Fatalf("%+v", got)
			}
			if test.want != LayoutStabilityUnknown && !got.Complete() {
				t.Fatal(got)
			}
			if test.name == "unsupported contract" && got.Contract != explicit.Contract {
				t.Fatal("lost requested contract", got)
			}
		})
	}
	r, _ := NewResolver(layoutTestPlan(64), nil)
	if got := r.Resolve(integer); got.Stability != LayoutStabilityUnknown || got.Complete() {
		t.Fatal(got)
	}
}

// Stability and its scope survive detached cached snapshots, but identical
// sizes on two plans never imply cross-plan or nominal compatibility.
// Rules: rules/memory/layout.md §§26,33-35,39.
func TestLayoutStabilityCacheAndScope(t *testing.T) {
	cache := NewLayoutCache()
	integer := scalar("int32", ScalarSigned, 32, false)
	requirement := LayoutRequirement{TypeIdentity: "int32[2]", Kind: LayoutArray, Element: &integer, Count: big.NewInt(2)}
	var previous ResolvedLayout
	for _, width := range []uint16{32, 64} {
		plan := layoutTestPlan(width)
		for round := 0; round < 2; round++ {
			r, err := NewResolverWithCache(plan, testAlignments(), cache)
			if err != nil {
				t.Fatal(err)
			}
			got := r.Resolve(requirement)
			if got.Stability != LayoutTargetStable || got.StabilityReason == "" {
				t.Fatal(got)
			}
			if width == 64 && previous.PlanKey == got.PlanKey {
				t.Fatal("stability erased plan scope")
			}
			if width == 32 {
				previous = got
			}
			got.StabilityReason = "caller mutation"
			if again, _ := r.Layout(got.ID); again.StabilityReason == got.StabilityReason {
				t.Fatal("published evidence mutated")
			}
		}
	}
	if cache.Hits() != 2 {
		t.Fatal("expected plan-specific warm hits", cache.Hits())
	}
}
