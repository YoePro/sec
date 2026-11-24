package layout

import "testing"

// TestAtomicProducerContextProofs covers MD-095 T26-T28 and T34-T35: only
// explicitly covered contexts obtain the category proof; a vector restriction
// and missing refinement evidence cannot be bypassed through its base category.
// Rules: rules/concurrency/atomics.md — § 37A(8), (10)-(11), (13).
func TestAtomicProducerContextProofs(t *testing.T) {
	base := atomicTestRequirement().Key
	base.Context, base.NoBlocking, base.InterruptSafe = AtomicContextInterrupt, true, true
	proof := AtomicCapability{Key: base, Availability: AtomicSupported, Provenance: "synthetic:verified-category-proof",
		Implementation: AtomicImplementation{Kind: AtomicPlatformPrimitive, Identity: "synthetic:isr-cas", Blocking: AtomicPropertyNo, InterruptSafe: AtomicPropertyYes}}
	timer := AtomicContextRefinement{InterruptVector: "timer", Priority: "low", MaskingDomain: "local", Core: "0", ExecutionDomain: "kernel"}
	uart := timer
	uart.InterruptVector = "uart"
	nmi := timer
	nmi.InterruptVector = "nmi"
	denied := AtomicCapability{Key: base, Availability: AtomicUnsupported, Provenance: "synthetic:vector-restriction", Reason: "not valid at NMI priority"}
	denied.Key.Refinement = nmi
	rules := []AtomicCapabilityRule{
		{Fact: proof, CoveredRefinements: []AtomicContextRefinement{timer, uart}},
		{Fact: denied},
	}
	policy := AtomicProfilePolicy{Activation: AtomicEnabled, Provenance: "synthetic:profile"}
	plan, err := atomicTestPlan().WithAtomicCapabilityRules(policy, rules)
	if err != nil {
		t.Fatal(err)
	}
	query := func(p ResolvedScalarPlan, refinement AtomicContextRefinement) AtomicCapability {
		t.Helper()
		request := AtomicRequirement{SemanticTypeID: "app:named:Counter", Provenance: "test", Key: base}
		request.Key.Refinement = refinement
		got, err := p.AtomicCapability(request)
		if err != nil {
			t.Fatal(err)
		}
		if got.Key != request.Key {
			t.Fatal("derived fact lost exact request", got)
		}
		return got
	}
	for _, refinement := range []AtomicContextRefinement{timer, uart} {
		got := query(plan, refinement)
		if got.Availability != AtomicSupported || got.Activation != AtomicEnabled || got.Provenance != proof.Provenance || got.Implementation != proof.Implementation {
			t.Fatal(got)
		}
	}
	if got := query(plan, nmi); got.Availability != AtomicUnsupported || got.Reason != denied.Reason {
		t.Fatal(got)
	}
	for _, missing := range []AtomicContextRefinement{{}, {InterruptVector: "timer"}, {InterruptVector: "other"}, func() AtomicContextRefinement { r := timer; r.Core = "1"; return r }()} {
		if got := query(plan, missing); got.Availability != AtomicUnknown || got.Activation != AtomicEnabled || got.Implementation != (AtomicImplementation{}) {
			t.Fatal("assumed permissive refinement", got)
		}
	}
	// Mutating input refinements, facts and policy never changes published proof.
	rules[0].CoveredRefinements[0].Core = "changed"
	rules[0].Fact.Implementation.Identity = "changed"
	policy.Activation = AtomicDisabled
	if got := query(plan, timer); got.Availability != AtomicSupported || got.Implementation != proof.Implementation || got.Activation != AtomicEnabled {
		t.Fatal(got)
	}
	// Global permission is independent of a verified implementation: no context
	// may enable a disabled profile, including a context without operation data.
	disabled, err := atomicTestPlan().WithAtomicCapabilityRules(AtomicProfilePolicy{Activation: AtomicDisabled, Provenance: "synthetic:profile", Reason: "disabled by profile"}, []AtomicCapabilityRule{{Fact: proof, CoveredRefinements: []AtomicContextRefinement{timer}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, refinement := range []AtomicContextRefinement{timer, nmi} {
		got := query(disabled, refinement)
		if got.Activation != AtomicDisabled || got.ActivationReason != "disabled by profile" {
			t.Fatal("context enabled profile", got)
		}
	}
	if got := query(disabled, timer); got.Availability != AtomicSupported {
		t.Fatal("activation erased physical availability", got)
	}
}

// TestAtomicProducerPlansAndOperations covers T36-T38, T48-T50 and T62:
// verified 64-bit operations on a 32-bit plan, independent target and operation
// evidence, optional misses, and rejection of unresolved required plan data.
// Rules: rules/concurrency/atomics.md — § 37A(6), (10)-(11), (14);
// rules/platform/platform_model.md — §§ 14, 21.
func TestAtomicProducerPlansAndOperations(t *testing.T) {
	key := atomicTestRequirement().Key
	key.Operation, key.Order, key.FailureOrder = "Load", "Acquire", ""
	proof := AtomicCapability{Key: key, Availability: AtomicSupported, Provenance: "synthetic:load64-proof", Implementation: AtomicImplementation{Kind: AtomicCompilerIntrinsic, Identity: "synthetic:load64"}}
	cas := proof
	cas.Key.Operation, cas.Key.FailureOrder = "CompareExchange", "Relaxed"
	cas.Availability, cas.Implementation = AtomicUnsupported, AtomicImplementation{}
	base := atomicTestPlan()
	base.PointerWidthBits = 32
	plan, err := base.WithAtomicCapabilityRules(AtomicProfilePolicy{}, []AtomicCapabilityRule{{Fact: proof}, {Fact: cas}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		key  AtomicCapabilityKey
		want AtomicAvailability
	}{{key, AtomicSupported}, {cas.Key, AtomicUnsupported}, {func() AtomicCapabilityKey { k := key; k.Operation = "Swap"; return k }(), AtomicUnknown}} {
		got, err := plan.AtomicCapability(AtomicRequirement{SemanticTypeID: "app:named:Count", Key: test.key, Provenance: "test"})
		if err != nil || got.Availability != test.want || got.Activation != AtomicActivationUnknown {
			t.Fatal(got, err)
		}
	}
	other := base
	other.TargetArch = "another-32-bit-target"
	fresh, err := other.WithAtomicCapabilityRules(AtomicProfilePolicy{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := fresh.AtomicCapability(AtomicRequirement{SemanticTypeID: "app:named:Count", Key: key, Provenance: "test"}); err != nil || got.Availability != AtomicUnknown {
		t.Fatal(got, err)
	}
	for _, bad := range []ResolvedScalarPlan{{}, func() ResolvedScalarPlan { p := base; p.PointerWidthBits = 0; return p }(), func() ResolvedScalarPlan { p := base; p.Profile = ""; return p }()} {
		if _, err := bad.WithAtomicCapabilityRules(AtomicProfilePolicy{}, nil); err == nil {
			t.Fatal("froze unresolved required plan facts")
		}
	}
}

// TestAtomicProducerValidation rejects contradictory derivations, incomplete
// keys/guarantees, and operation-local activation before a snapshot is published.
// Rules: rules/concurrency/atomics.md — § 37A(7)-(11), (13)-(14).
func TestAtomicProducerValidation(t *testing.T) {
	proof := AtomicCapability{Key: atomicTestRequirement().Key, Availability: AtomicSupported, Provenance: "synthetic:proof", Implementation: AtomicImplementation{Kind: AtomicNativeInstruction, Identity: "synthetic:cas"}}
	for name, rules := range map[string][]AtomicCapabilityRule{
		"duplicate derivation": {{Fact: proof, CoveredRefinements: []AtomicContextRefinement{{Core: "0"}, {Core: "0"}}}},
		"conflicting rules": {{Fact: proof}, {Fact: func() AtomicCapability {
			f := proof
			f.Availability, f.Implementation = AtomicUnsupported, AtomicImplementation{}
			return f
		}()}},
		"partial base":             {{Fact: func() AtomicCapability { f := proof; f.Key.Refinement.Core = "0"; return f }(), CoveredRefinements: []AtomicContextRefinement{{Core: "1"}}}},
		"vector outside interrupt": {{Fact: proof, CoveredRefinements: []AtomicContextRefinement{{InterruptVector: "timer"}}}},
		"missing context proof":    {{Fact: func() AtomicCapability { f := proof; f.Key.NoBlocking = true; return f }()}},
		"missing representation":   {{Fact: func() AtomicCapability { f := proof; f.Key.Representation = StorageRepresentation{}; return f }()}},
		"context-local activation": {{Fact: func() AtomicCapability { f := proof; f.Activation = AtomicEnabled; return f }()}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := atomicTestPlan().WithAtomicCapabilityRules(AtomicProfilePolicy{}, rules); err == nil {
				t.Fatal("accepted invalid producer data")
			}
		})
	}
	for _, policy := range []AtomicProfilePolicy{{Activation: "bogus"}, {Activation: AtomicEnabled}, {Activation: AtomicDisabled, Provenance: "test"}} {
		if _, err := atomicTestPlan().WithAtomicCapabilityRules(policy, nil); err == nil {
			t.Fatal("accepted invalid activation", policy)
		}
	}
}

// TestAtomicProducerFingerprints makes derivation independent of producer order
// and invalidates proofs for changed domain coverage or profile permission.
// Rules: rules/concurrency/atomics.md — § 37A(10), (14).
func TestAtomicProducerFingerprints(t *testing.T) {
	proof := AtomicCapability{Key: atomicTestRequirement().Key, Availability: AtomicUnsupported, Provenance: "synthetic:proof"}
	freeze := func(policy AtomicProfilePolicy, refs ...AtomicContextRefinement) ResolvedScalarPlan {
		t.Helper()
		p, err := atomicTestPlan().WithAtomicCapabilityRules(policy, []AtomicCapabilityRule{{Fact: proof, CoveredRefinements: refs}})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	policy := AtomicProfilePolicy{Activation: AtomicEnabled, Provenance: "test"}
	first, second := AtomicContextRefinement{Core: "0"}, AtomicContextRefinement{Core: "1"}
	base := freeze(policy, first, second)
	policy.Provenance, policy.Reason = "changed description", "description"
	if freeze(policy, second, first).Fingerprints() != base.Fingerprints() {
		t.Fatal("derivation order/provenance affected meaning")
	}
	if freeze(policy, first).Fingerprints().Plan == base.Fingerprints().Plan {
		t.Fatal("coverage change reused proof")
	}
	policy.Activation = AtomicDisabled
	if freeze(policy, first, second).Fingerprints().Plan == base.Fingerprints().Plan {
		t.Fatal("activation change reused proof")
	}
	other := base
	other.Profile = "different"
	if _, err := other.AtomicCapability(atomicTestRequirement()); err == nil {
		t.Fatal("derived snapshot crossed profile")
	}
}
