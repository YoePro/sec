package layout

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// MD-140 T04–T09: Arena's outcome is invariant across profiles, and selecting
// a smaller logical width needs explicit conforming safety evidence. A
// machine word size never replaces the canonical logical width.
// Rules: rules/memory/arena.md §§38(8),39(4)–(9);
// rules/platform/target_profiles.md §13; rules/memory/reference_model.md §§33–34.
func TestArenaEpochPolicyPlans(t *testing.T) {
	for _, pointerBits := range []uint16{32, 64} {
		for _, profile := range []string{"hosted", "rtos", "baremetal", "custom-derived"} {
			base := ResolvedScalarPlan{TargetOS: "test", TargetArch: "test", Profile: profile, PointerWidthBits: pointerBits, Endianness: LittleEndian}
			if err := base.Validate(); err != nil {
				t.Fatal(err)
			}
			got, err := base.References.ResolveArenaEpochPolicy()
			if err != nil || got.LogicalEpochBits != 64 || got.Exhaustion != ArenaEpochPanic || got.ShortWidthSafety != "" || got.ShortWidthJustification != "" {
				t.Fatalf("default %d/%s: %+v, %v", pointerBits, profile, got, err)
			}
			// Explicit defaults have the same effective policy and no check-elimination
			// proof is inferred from the default width or outcome selection.
			explicit := base
			explicit.References.LogicalEpochBits = 64
			explicit.References.ArenaExhaustion = ArenaEpochPanic
			resolved, err := explicit.References.ResolveArenaEpochPolicy()
			if err != nil || resolved != got {
				t.Fatalf("explicit default: %+v, %v", resolved, err)
			}
			for _, width := range []uint16{8, 16, 32} {
				short := base
				short.References = ResolvedReferencePolicy{LogicalEpochBits: width, ShortWidthProof: "verified producer: checked advancement before commit", ArenaShortWidthSafety: ArenaEpochCheckedAdvance}
				if err := short.Validate(); err != nil {
					t.Fatal(err)
				}
				policy, err := short.References.ResolveArenaEpochPolicy()
				if err != nil || policy.LogicalEpochBits != width || policy.Exhaustion != ArenaEpochPanic || policy.ShortWidthSafety != ArenaEpochCheckedAdvance || policy.ShortWidthJustification != short.References.ShortWidthProof {
					t.Fatalf("short policy: %+v, %v", policy, err)
				}
				if short.Fingerprints().Plan == base.Fingerprints().Plan || short.Fingerprints().Memory == base.Fingerprints().Memory {
					t.Fatal("reference constraints missing from plan identity")
				}
				encoded, err := json.Marshal(short)
				if err != nil {
					t.Fatal(err)
				}
				var restored ResolvedScalarPlan
				if err := json.Unmarshal(encoded, &restored); err != nil {
					t.Fatal(err)
				}
				if err := restored.Validate(); err != nil {
					t.Fatal(err)
				}
				roundTrip, _ := restored.References.ResolveArenaEpochPolicy()
				if roundTrip != policy {
					t.Fatal("plan transport lost Arena constraints")
				}
				policy.LogicalEpochBits = 8
				policy.ShortWidthJustification = "changed"
				again, _ := short.References.ResolveArenaEpochPolicy()
				if again.LogicalEpochBits != width || again.ShortWidthJustification != short.References.ShortWidthProof {
					t.Fatal("policy snapshot changed source")
				}
			}
			for _, forbidden := range []ArenaEpochExhaustion{"rekey", "domain-recreation", "retirement", "fallible", "trap", "unchecked", "unknown"} {
				other := base
				other.References.ArenaExhaustion = forbidden
				if err := other.Validate(); err == nil || !strings.Contains(err.Error(), "must panic") {
					t.Fatalf("accepted %s/%s policy: %v", profile, forbidden, err)
				}
				if got, err := other.References.ResolveArenaEpochPolicy(); err == nil || !reflect.DeepEqual(got, ResolvedArenaEpochPolicy{}) {
					t.Fatal("invalid policy published resolved facts")
				}
				if _, err := LayoutKeyOf(other, nil); err == nil {
					t.Fatal("invalid Arena policy received layout identity")
				}
			}
		}
	}
}

// MD-140 T09/T48: non-Arena slot retirement stays a valid general reference
// justification but cannot silently establish the Arena-specific guarantee.
// Rules: rules/memory/reference_model.md §§34,38–40; rules/memory/arena.md §39.
func TestArenaEpochPolicyRejectsMissingSafety(t *testing.T) {
	for _, policy := range []ResolvedReferencePolicy{
		{LogicalEpochBits: 8},
		{LogicalEpochBits: 16, ShortWidthProof: "retire slot"},
		{LogicalEpochBits: 32, ShortWidthProof: "generic proof"},
		{LogicalEpochBits: 32, ShortWidthProof: " \t", ArenaShortWidthSafety: ArenaEpochCheckedAdvance},
		{LogicalEpochBits: 32, ShortWidthProof: "proof", ArenaShortWidthSafety: "retire"},
		{LogicalEpochBits: 24, ShortWidthProof: "proof", ArenaShortWidthSafety: ArenaEpochCheckedAdvance},
		{LogicalEpochBits: 128, ShortWidthProof: "proof", ArenaShortWidthSafety: ArenaEpochCheckedAdvance},
	} {
		plan := ResolvedScalarPlan{TargetOS: "test", TargetArch: "test", PointerWidthBits: 64, Endianness: LittleEndian, References: policy}
		if err := plan.Validate(); err == nil {
			t.Fatalf("unsafe plan accepted: %+v", policy)
		}
	}
	generic := ResolvedReferencePolicy{LogicalEpochBits: 32, ShortWidthProof: "retire-on-exhaustion"}
	if err := generic.Validate(); err != nil {
		t.Fatal("non-Arena retirement changed", err)
	}
	if _, err := generic.ResolveArenaEpochPolicy(); err == nil {
		t.Fatal("slot retirement authorized Arena")
	}
}
