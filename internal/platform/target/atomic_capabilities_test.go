package target

import (
	"testing"

	"sec/internal/layout"
)

// TestRegistryDoesNotGuessAtomicCapabilities keeps every registered target's
// missing concrete-operation facts unresolved, regardless of host or width.
// Rules: rules/concurrency/atomics.md — §37(1–5);
// rules/platform/platform_model.md — §9 "Architecture capability".
func TestRegistryDoesNotGuessAtomicCapabilities(t *testing.T) {
	request := layout.AtomicRequirement{SemanticTypeID: "core:primitive:uint64", Provenance: "test", Key: layout.AtomicCapabilityKey{CarrierTypeID: "core:primitive:uint64", Representation: layout.StorageRepresentation{Class: layout.IntegerStorage, WidthBits: 64}, SizeBytes: 8, AlignmentBytes: 8, Operation: "Load", Order: "SeqCst", Context: layout.AtomicContextThread}}
	for _, definition := range Definitions() {
		plan, err := definition.ScalarPlan()
		if definition.PointerWidthBits == 0 {
			// Placeholder targets have no concrete scalar plan at all and
			// therefore cannot provide any operation capability either.
			if err == nil {
				t.Fatal("unresolved target produced a plan", definition)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		fact, err := plan.AtomicCapability(request)
		if err != nil || fact.Availability != layout.AtomicUnknown {
			t.Fatal(definition.OS, definition.Arch, fact, err)
		}
	}
}

// TestDefinitionPreservesFrozenAtomicCapabilities carries exact target facts
// through the canonical registry-to-plan boundary and rejects foreign facts.
// Rules: rules/concurrency/atomics.md — §37(1–5);
// rules/platform/target_profiles.md — §§29–31.
func TestDefinitionPreservesFrozenAtomicCapabilities(t *testing.T) {
	definition, ok := Find(Target{OS: "linux", Arch: "armv7"})
	if !ok {
		t.Fatal("missing test target")
	}
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	request := layout.AtomicRequirement{SemanticTypeID: "core:primitive:uint64", Provenance: "test", Key: layout.AtomicCapabilityKey{CarrierTypeID: "core:primitive:uint64", Representation: layout.StorageRepresentation{Class: layout.IntegerStorage, WidthBits: 64}, SizeBytes: 8, AlignmentBytes: 8, Operation: "Load", Order: "Acquire", Context: layout.AtomicContextInterrupt, NoBlocking: true, InterruptSafe: true, Refinement: layout.AtomicContextRefinement{InterruptVector: "timer"}}}
	fact := layout.AtomicCapability{Key: request.Key, Availability: layout.AtomicUnsupported, Provenance: "test-target-contract", Reason: "test-only unavailable operation"}
	plan, err = plan.WithAtomicCapabilities([]layout.AtomicCapability{fact})
	if err != nil {
		t.Fatal(err)
	}
	definition.Atomics = plan.Atomics
	resolved, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := resolved.AtomicCapability(request); err != nil || got != fact {
		t.Fatal(got, err)
	}
	definition.Profile = "noalloc"
	if _, err := definition.ScalarPlan(); err == nil {
		t.Fatal("capability crossed profile boundary")
	}
	// Changing the local copy must not mutate the compiler-owned registry.
	original, _ := Find(Target{OS: "linux", Arch: "armv7"})
	fresh, err := original.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := fresh.AtomicCapability(request); err != nil || got.Availability != layout.AtomicUnknown {
		t.Fatal(got, err)
	}
}

// TestDefinitionPreservesAtomicRealization keeps explicit implementation and
// progress facts across registry projection, without equating emulation with
// blocking or native support with language eligibility.
// Rules: rules/concurrency/atomics.md — §§37–38;
// rules/platform/target_profiles.md — atomic capability separation.
func TestDefinitionPreservesAtomicRealization(t *testing.T) {
	definition, ok := Find(Target{OS: "linux", Arch: "armv7"})
	if !ok {
		t.Fatal("missing test target")
	}
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	request := layout.AtomicRequirement{SemanticTypeID: "core:primitive:uint64", Provenance: "test", Key: layout.AtomicCapabilityKey{CarrierTypeID: "core:primitive:uint64", Representation: layout.StorageRepresentation{Class: layout.IntegerStorage, WidthBits: 64}, SizeBytes: 8, AlignmentBytes: 8, Operation: "Swap", Order: "SeqCst", Context: layout.AtomicContextOrdinary}}
	fact := layout.AtomicCapability{
		Key: request.Key, Availability: layout.AtomicSupported, Provenance: "test-only-proof",
		Implementation: layout.AtomicImplementation{Kind: layout.AtomicEmulated, Identity: "test-lock-free-emulation", LockFree: layout.AtomicPropertyYes, Blocking: layout.AtomicPropertyNo, InterruptSafe: layout.AtomicPropertyUnknown},
	}
	plan, err = plan.WithAtomicCapabilities([]layout.AtomicCapability{fact})
	if err != nil {
		t.Fatal(err)
	}
	definition.Atomics = plan.Atomics
	projected, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := projected.AtomicCapability(request); err != nil || got != fact {
		t.Fatal(got, err)
	}
}

// TestDefinitionPreservesDerivedAtomicProofs preserves both concrete context
// evidence and independent profile permission across canonical plan projection.
// This synthetic fixture establishes no capability on a registered target.
// Rules: rules/concurrency/atomics.md — § 37A(10)-(11), (14).
func TestDefinitionPreservesDerivedAtomicProofs(t *testing.T) {
	definition, ok := Find(Target{OS: "linux", Arch: "armv7"})
	if !ok {
		t.Fatal("missing fixture target")
	}
	base, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	key := layout.AtomicCapabilityKey{CarrierTypeID: "core:primitive:uint64", Representation: layout.StorageRepresentation{Class: layout.IntegerStorage, WidthBits: 64}, SizeBytes: 8, AlignmentBytes: 8, Operation: "Load", Order: "Acquire", Context: layout.AtomicContextInterrupt, NoBlocking: true, InterruptSafe: true}
	fact := layout.AtomicCapability{Key: key, Availability: layout.AtomicSupported, Provenance: "synthetic:category-proof", Implementation: layout.AtomicImplementation{Kind: layout.AtomicPlatformPrimitive, Identity: "synthetic:load64", Blocking: layout.AtomicPropertyNo, InterruptSafe: layout.AtomicPropertyYes}}
	refinement := layout.AtomicContextRefinement{InterruptVector: "timer", Core: "0"}
	plan, err := base.WithAtomicCapabilityRules(layout.AtomicProfilePolicy{Activation: layout.AtomicDisabled, Provenance: "synthetic:profile", Reason: "profile prohibits atomics"}, []layout.AtomicCapabilityRule{{Fact: fact, CoveredRefinements: []layout.AtomicContextRefinement{refinement}}})
	if err != nil {
		t.Fatal(err)
	}
	definition.Atomics = plan.Atomics
	projected, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	request := layout.AtomicRequirement{SemanticTypeID: "test:named:Count", Key: key, Provenance: "test"}
	request.Key.Refinement = refinement
	got, err := projected.AtomicCapability(request)
	if err != nil || got.Availability != layout.AtomicSupported || got.Activation != layout.AtomicDisabled || got.Implementation != fact.Implementation || got.ActivationReason != "profile prohibits atomics" {
		t.Fatal(got, err)
	}
	if projected.Fingerprints() != plan.Fingerprints() {
		t.Fatal("projection changed derived proof meaning")
	}
	// Editing detached query results never enables the profile in the snapshot.
	got.Activation = layout.AtomicEnabled
	again, err := projected.AtomicCapability(request)
	if err != nil || again.Activation != layout.AtomicDisabled {
		t.Fatal(again, err)
	}
}
