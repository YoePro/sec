package layout

import "testing"

// TestAtomicRealizationFacts preserves declared progress independently of the
// chosen mechanism: native alone proves no progress guarantee, and emulation
// can be explicitly lock-free rather than an invented mutex fallback.
// Rules: rules/concurrency/atomics.md — §§37–39;
// rules/platform/platform_model.md — §9 "Architecture capability".
func TestAtomicRealizationFacts(t *testing.T) {
	for _, implementation := range []AtomicImplementation{
		{Kind: AtomicNativeInstruction, Identity: "test-native"},
		{Kind: AtomicNativeInstruction, Identity: "test-native-lock-free", LockFree: AtomicPropertyYes, Blocking: AtomicPropertyNo, InterruptSafe: AtomicPropertyYes},
		{Kind: AtomicCompilerIntrinsic, Identity: "test-intrinsic", LockFree: AtomicPropertyUnknown, Blocking: AtomicPropertyNo},
		{Kind: AtomicPlatformPrimitive, Identity: "test-platform", LockFree: AtomicPropertyNo, Blocking: AtomicPropertyNo, InterruptSafe: AtomicPropertyYes},
		{Kind: AtomicEmulated, Identity: "test-emulated-lock-free", LockFree: AtomicPropertyYes, Blocking: AtomicPropertyNo, InterruptSafe: AtomicPropertyYes},
		{Kind: AtomicEmulated, Identity: "test-emulated-blocking", LockFree: AtomicPropertyNo, Blocking: AtomicPropertyYes, InterruptSafe: AtomicPropertyNo},
	} {
		t.Run(implementation.Identity, func(t *testing.T) {
			request := atomicTestRequirement()
			fact := AtomicCapability{Requirement: request, Availability: AtomicSupported, Provenance: "test-target-proof", Implementation: implementation}
			inputs := []AtomicCapability{fact}
			plan, err := atomicTestPlan().WithAtomicCapabilities(inputs)
			if err != nil {
				t.Fatal(err)
			}
			inputs[0].Implementation.Kind = AtomicEmulated
			inputs[0].Implementation.LockFree = AtomicPropertyNo
			got, err := plan.AtomicCapability(request)
			if err != nil || got != fact {
				t.Fatal(got, err)
			}
			got.Implementation.Identity = "changed"
			got.Implementation.Blocking = AtomicPropertyYes
			if again, err := plan.AtomicCapability(request); err != nil || again != fact {
				t.Fatal(again, err)
			}
		})
	}
}

// TestAtomicRealizationRequiresEvidence rejects affirmative capability without
// a selected declared implementation and forbids attaching one to missing or
// negative support. Unknown property values remain explicit conservative facts.
// Rules: rules/concurrency/atomics.md — §§37(1),38(1–4);
// rules/platform/target_profiles.md — atomic capability separation.
func TestAtomicRealizationRequiresEvidence(t *testing.T) {
	valid := AtomicCapability{Requirement: atomicTestRequirement(), Availability: AtomicSupported, Provenance: "test-proof", Implementation: AtomicImplementation{Kind: AtomicEmulated, Identity: "test-emulation"}}
	for name, change := range map[string]func(*AtomicCapability){
		"missing implementation":           func(f *AtomicCapability) { f.Implementation = AtomicImplementation{} },
		"missing identity":                 func(f *AtomicCapability) { f.Implementation.Identity = "" },
		"unknown kind":                     func(f *AtomicCapability) { f.Implementation.Kind = "host-auto" },
		"invalid lock freedom":             func(f *AtomicCapability) { f.Implementation.LockFree = "probably" },
		"invalid blocking":                 func(f *AtomicCapability) { f.Implementation.Blocking = "probably" },
		"invalid interrupt safety":         func(f *AtomicCapability) { f.Implementation.InterruptSafe = "probably" },
		"denial selecting implementation":  func(f *AtomicCapability) { f.Availability = AtomicUnsupported },
		"unknown selecting implementation": func(f *AtomicCapability) { f.Availability = AtomicUnknown },
	} {
		t.Run(name, func(t *testing.T) {
			bad := valid
			change(&bad)
			if _, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{bad}); err == nil {
				t.Fatal("unproven realization accepted", bad)
			}
		})
	}
	for _, availability := range []AtomicAvailability{AtomicUnknown, AtomicUnsupported} {
		fact := valid
		fact.Availability = availability
		fact.Implementation = AtomicImplementation{}
		plan, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{fact})
		if err != nil {
			t.Fatal(err)
		}
		if got, err := plan.AtomicCapability(fact.Requirement); err != nil || got != fact {
			t.Fatal(got, err)
		}
	}
}
