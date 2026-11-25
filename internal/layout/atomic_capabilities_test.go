package layout

import "testing"

// TestAtomicCapabilitiesExactKeys prevents capability evidence from widening
// to another carrier, representation, layout, order or execution category;
// the frozen table is detached from producer input.
// Rules: rules/concurrency/atomics.md — §§24, 37(1–5), 37A(6), 37A(9), 39, 47(3–4).
func TestAtomicCapabilitiesExactKeys(t *testing.T) {
	request := atomicTestRequirement()
	supported := AtomicCapability{Key: request.Key, Availability: AtomicSupported, Provenance: "test-target-contract", Reason: "verified test implementation", Implementation: AtomicImplementation{Kind: AtomicCompilerIntrinsic, Identity: "test-intrinsic"}}
	input := []AtomicCapability{supported}
	plan, err := atomicTestPlan().WithAtomicCapabilities(input)
	if err != nil {
		t.Fatal(err)
	}
	// The producer must not retain mutable access after the plan is frozen.
	input[0].Availability = AtomicUnsupported
	input[0].Key.CarrierTypeID = "core:primitive:int64"
	got, err := plan.AtomicCapability(request)
	if err != nil || got != supported {
		t.Fatal(got, err)
	}
	got.Availability = AtomicUnsupported
	if again, err := plan.AtomicCapability(request); err != nil || again != supported {
		t.Fatal(again, err)
	}
	for name, change := range map[string]func(*AtomicCapabilityKey){
		// T11/T12: an equally sized raw pointer or bool carrier is not covered
		// by an integer entry.
		"raw-pointer carrier": func(k *AtomicCapabilityKey) { k.CarrierTypeID = "core:raw-pointer:default" },
		"bool carrier": func(k *AtomicCapabilityKey) {
			k.CarrierTypeID = "core:primitive:bool"
			k.Representation.WidthBits = 8
			k.SizeBytes = 1
			k.AlignmentBytes = 1
		},
		"signed carrier": func(k *AtomicCapabilityKey) { k.CarrierTypeID = "core:primitive:int64" },
		"width": func(k *AtomicCapabilityKey) {
			k.Representation.WidthBits = 128
			k.SizeBytes = 16
			k.AlignmentBytes = 16
		},
		"alignment":     func(k *AtomicCapabilityKey) { k.AlignmentBytes = 4 },
		"operation":     func(k *AtomicCapabilityKey) { k.Operation = "Swap"; k.FailureOrder = "" },
		"success order": func(k *AtomicCapabilityKey) { k.Order = "Acquire" },
		"failure order": func(k *AtomicCapabilityKey) { k.FailureOrder = "Acquire" },
		"category":      func(k *AtomicCapabilityKey) { k.Context = AtomicContextTask },
		"no block":      func(k *AtomicCapabilityKey) { k.NoBlocking = true },
		"interrupt":     func(k *AtomicCapabilityKey) { k.Context = AtomicContextInterrupt; k.InterruptSafe = true },
		"refinement":    func(k *AtomicCapabilityKey) { k.Refinement.Core = "core1" },
	} {
		t.Run(name, func(t *testing.T) {
			other := request
			change(&other.Key)
			fact, err := plan.AtomicCapability(other)
			if err != nil || fact.Availability != AtomicUnknown || fact.Key != other.Key || fact.Provenance != "" {
				t.Fatal("capability leaked to a different key", fact, err)
			}
		})
	}
}

// TestAtomicCapabilitiesShareCarrier queries by carrier, never by the nominal
// source type: uint64, two distinct named types over it, a multi-layer named
// type and generic instantiations share one target entry while keeping their
// semantic identities, and RawPtr to different pointees shares the raw-pointer
// carrier (T01-T04, T08, T10). No entry per user-defined type is needed.
// Rules: rules/concurrency/atomics.md — §§ 37A(4)-(6), 37A(9).
func TestAtomicCapabilitiesShareCarrier(t *testing.T) {
	integer := atomicTestRequirement().Key
	pointer := integer
	pointer.CarrierTypeID = "core:raw-pointer:default"
	plan, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{
		{Key: integer, Availability: AtomicSupported, Provenance: "test-proof", Implementation: AtomicImplementation{Kind: AtomicNativeInstruction, Identity: "test-cas64"}},
		{Key: pointer, Availability: AtomicUnsupported, Provenance: "test-proof", Reason: "pointer CAS unavailable"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, semantic := range []string{"core:primitive:uint64", "app:named:RequestCount", "app:named:UserID", "app:named:Outer", "app:named:Counter[app:named:A]", "app:named:Counter[app:named:B]"} {
		got, err := plan.AtomicCapability(AtomicRequirement{SemanticTypeID: semantic, Key: integer, Provenance: "source.sec:1:1"})
		if err != nil || got.Availability != AtomicSupported || got.Implementation.Identity != "test-cas64" {
			t.Fatalf("%s: carrier capability not shared: %v %v", semantic, got, err)
		}
	}
	for _, semantic := range []string{"core:raw-pointer[app:named:A]", "core:raw-pointer[app:named:B]"} {
		got, err := plan.AtomicCapability(AtomicRequirement{SemanticTypeID: semantic, Key: pointer, Provenance: "source.sec:1:1"})
		if err != nil || got.Availability != AtomicUnsupported {
			t.Fatalf("%s: raw-pointer carrier capability not shared: %v %v", semantic, got, err)
		}
	}
}

// TestAtomicCapabilitiesPlanBinding rejects cross-target/profile reuse rather
// than deriving support from architecture, pointer width or compiler host.
// Rules: rules/concurrency/atomics.md — §§ 37(1–5), 37A(14);
// rules/platform/target_profiles.md — §31.
func TestAtomicCapabilitiesPlanBinding(t *testing.T) {
	request := atomicTestRequirement()
	plan, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{{Key: request.Key, Availability: AtomicUnsupported, Provenance: "test-profile", Reason: "operation unavailable"}})
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*ResolvedScalarPlan){
		"OS":      func(p *ResolvedScalarPlan) { p.TargetOS = "different" },
		"arch":    func(p *ResolvedScalarPlan) { p.TargetArch = "other" },
		"triple":  func(p *ResolvedScalarPlan) { p.LLVMTriple = "other-triple" },
		"ABI":     func(p *ResolvedScalarPlan) { p.ABI = "other-abi" },
		"profile": func(p *ResolvedScalarPlan) { p.Profile = "noalloc" },
		"width":   func(p *ResolvedScalarPlan) { p.PointerWidthBits = 32 },
		"endian":  func(p *ResolvedScalarPlan) { p.Endianness = BigEndian },
		"C ABI":   func(p *ResolvedScalarPlan) { p.CABI = CModelSysVX8664 },
		"reference policy": func(p *ResolvedScalarPlan) {
			p.References.LogicalEpochBits = 32
			p.References.ShortWidthProof = "test-proof"
			p.References.ArenaShortWidthSafety = ArenaEpochCheckedAdvance
		},
	} {
		t.Run(name, func(t *testing.T) {
			other := plan
			change(&other)
			if err := other.Validate(); err == nil {
				t.Fatal("accepted foreign snapshot")
			}
			if _, err := other.AtomicCapability(request); err == nil {
				t.Fatal("queried foreign snapshot")
			}
		})
	}
	copy := plan
	if fact, err := copy.AtomicCapability(request); err != nil || fact.Availability != AtomicUnsupported || fact.Reason != "operation unavailable" {
		t.Fatal(fact, err)
	}
}

// TestAtomicCapabilitiesKeySchema rejects a snapshot built under another key
// schema, such as the former semantic-type-only table (T61), instead of
// reusing it.
// Rules: rules/concurrency/atomics.md — § 37A(14).
func TestAtomicCapabilitiesKeySchema(t *testing.T) {
	plan, err := atomicTestPlan().WithAtomicCapabilities(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{"sec.atomic-capability-key/1", "sec.atomic-capability-key/2"} {
		legacy := *plan.Atomics.snapshot
		legacy.schema = schema
		plan.Atomics = AtomicCapabilities{snapshot: &legacy}
		if err := plan.Validate(); err == nil {
			t.Fatal("legacy key schema accepted", schema)
		}
		if _, err := plan.AtomicCapability(atomicTestRequirement()); err == nil {
			t.Fatal("legacy key schema queried", schema)
		}
	}
}

// TestAtomicCapabilitiesFingerprints binds dependent proofs to the meaning of
// the selected facts: a changed availability, key or implementation guarantee
// changes the architecture and plan fingerprints, while provenance, reason,
// producer order and explicit Unknown entries do not (T60).
// Rules: rules/concurrency/atomics.md — § 37A(14);
// rules/platform/platform_model.md — §§ 21, 23.
func TestAtomicCapabilitiesFingerprints(t *testing.T) {
	key := atomicTestRequirement().Key
	other := key
	other.Operation, other.FailureOrder = "Load", ""
	supported := AtomicCapability{Key: key, Availability: AtomicSupported, Provenance: "a", Reason: "x", Implementation: AtomicImplementation{Kind: AtomicNativeInstruction, Identity: "cas", Blocking: AtomicPropertyNo}}
	load := AtomicCapability{Key: other, Availability: AtomicUnsupported, Provenance: "a"}
	freeze := func(facts ...AtomicCapability) PlanFingerprints {
		t.Helper()
		plan, err := atomicTestPlan().WithAtomicCapabilities(facts)
		if err != nil {
			t.Fatal(err)
		}
		return plan.Fingerprints()
	}
	base := freeze(supported, load)
	empty := freeze()
	if base.Architecture == empty.Architecture || base.Plan == empty.Plan {
		t.Fatal("atomic facts do not reach the plan fingerprints")
	}
	described := supported
	described.Provenance, described.Reason = "b", "y"
	unknown := AtomicCapability{Key: func() AtomicCapabilityKey { k := key; k.Order = "SeqCst"; return k }(), Availability: AtomicUnknown}
	if freeze(load, described, unknown) != base {
		t.Fatal("provenance, order of facts or Unknown entries changed the fingerprint")
	}
	guarantee := supported
	guarantee.Implementation.LockFree = AtomicPropertyYes
	denied := load
	denied.Availability = AtomicSupported
	denied.Implementation = AtomicImplementation{Kind: AtomicNativeInstruction, Identity: "load"}
	widened := supported
	widened.Key.AlignmentBytes = 16
	for name, facts := range map[string][]AtomicCapability{
		"guarantee":    {guarantee, load},
		"availability": {supported, denied},
		"key":          {widened, load},
	} {
		if freeze(facts...).Plan == base.Plan {
			t.Fatalf("%s change kept the plan fingerprint", name)
		}
	}
}

// TestAtomicCapabilitiesUnknownAndFence keeps missing evidence distinct from
// denial and represents fences without a carrier, representation or layout
// (T20, T50).
// Rules: rules/concurrency/atomics.md — §§24–26, 37, 37A(7), 37A(11), 40.
func TestAtomicCapabilitiesUnknownAndFence(t *testing.T) {
	request := atomicTestRequirement()
	for _, width := range []uint16{32, 64} {
		plan := atomicTestPlan()
		plan.PointerWidthBits = width
		fact, err := plan.AtomicCapability(request)
		if err != nil || fact.Availability != AtomicUnknown {
			t.Fatal(fact, err)
		}
	}
	fence := AtomicRequirement{Key: AtomicCapabilityKey{Operation: "Fence", Order: "SeqCst", Context: AtomicContextThread}, Provenance: "source.sec:2:1"}
	plan, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{{Key: fence.Key, Availability: AtomicSupported, Provenance: "test-fence", Implementation: AtomicImplementation{Kind: AtomicNativeInstruction, Identity: "test-fence"}}})
	if err != nil {
		t.Fatal(err)
	}
	if fact, err := plan.AtomicCapability(fence); err != nil || fact.Availability != AtomicSupported {
		t.Fatal(fact, err)
	}
	fence.Key.Order = "Acquire"
	if fact, err := plan.AtomicCapability(fence); err != nil || fact.Availability != AtomicUnknown {
		t.Fatal("invented strengthening", fact, err)
	}
	if fact, err := plan.AtomicCapability(request); err != nil || fact.Availability != AtomicUnknown {
		t.Fatal("fence capability supplied storage operation", fact, err)
	}
}

// TestAtomicCapabilitiesRejectIncompleteFacts keeps incomplete layout,
// noncanonical contexts, unresolved orders and duplicate evidence out of the
// frozen table: no usable zero size or alignment (T45), no free-string
// context (T23), no interrupt category without interrupt safety and no
// semantic identity on a fence.
// Rules: rules/concurrency/atomics.md — §§24, 37, 37A(7)-(9);
// rules/memory/layout.md — § 47.6.
func TestAtomicCapabilitiesRejectIncompleteFacts(t *testing.T) {
	valid := AtomicCapability{Key: atomicTestRequirement().Key, Availability: AtomicSupported, Provenance: "test-proof", Implementation: AtomicImplementation{Kind: AtomicPlatformPrimitive, Identity: "test-primitive"}}
	for name, change := range map[string]func(*AtomicCapability){
		"carrier":        func(f *AtomicCapability) { f.Key.CarrierTypeID = "" },
		"representation": func(f *AtomicCapability) { f.Key.Representation = StorageRepresentation{} },
		"float representation": func(f *AtomicCapability) {
			f.Key.Representation = StorageRepresentation{Class: BinaryFloatStorage, WidthBits: 64}
		},
		"zero size":           func(f *AtomicCapability) { f.Key.SizeBytes = 0 },
		"size mismatch":       func(f *AtomicCapability) { f.Key.SizeBytes = 4 },
		"zero alignment":      func(f *AtomicCapability) { f.Key.AlignmentBytes = 0 },
		"odd alignment":       func(f *AtomicCapability) { f.Key.AlignmentBytes = 6 },
		"operation":           func(f *AtomicCapability) { f.Key.Operation = "compare_exchange" },
		"order":               func(f *AtomicCapability) { f.Key.Order = "" },
		"order spelling":      func(f *AtomicCapability) { f.Key.Order = "acquire" },
		"failure order":       func(f *AtomicCapability) { f.Key.FailureOrder = "" },
		"extra failure order": func(f *AtomicCapability) { f.Key.Operation = "Load" },
		"empty context":       func(f *AtomicCapability) { f.Key.Context = "" },
		"string context":      func(f *AtomicCapability) { f.Key.Context = "thread::main" },
		"isr string context":  func(f *AtomicCapability) { f.Key.Context = "isr::timer" },
		"interrupt unsafe":    func(f *AtomicCapability) { f.Key.Context = AtomicContextInterrupt },
		"vector outside ISR":  func(f *AtomicCapability) { f.Key.Refinement.InterruptVector = "timer" },
		"provenance":          func(f *AtomicCapability) { f.Provenance = "" },
		"availability":        func(f *AtomicCapability) { f.Availability = "enabled" },
		"fence storage":       func(f *AtomicCapability) { f.Key.Operation = "Fence"; f.Key.FailureOrder = "" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := valid
			change(&bad)
			if _, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{bad}); err == nil {
				t.Fatal("accepted incomplete fact")
			}
			if bad.Key != valid.Key {
				if _, err := atomicTestPlan().AtomicCapability(AtomicRequirement{SemanticTypeID: "core:primitive:uint64", Key: bad.Key, Provenance: "source.sec:1:1"}); err == nil {
					t.Fatal("accepted incomplete request")
				}
			}
		})
	}
	for name, request := range map[string]AtomicRequirement{
		"missing semantic type": {Key: valid.Key, Provenance: "source.sec:1:1"},
		"missing provenance":    {SemanticTypeID: "core:primitive:uint64", Key: valid.Key},
		"semantic type on fence": {SemanticTypeID: "core:primitive:uint64", Provenance: "source.sec:1:1",
			Key: AtomicCapabilityKey{Operation: "Fence", Order: "SeqCst", Context: AtomicContextOrdinary}},
	} {
		if _, err := atomicTestPlan().AtomicCapability(request); err == nil {
			t.Fatalf("%s: accepted request", name)
		}
	}
	if _, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{valid, valid}); err == nil {
		t.Fatal("duplicate fact accepted")
	}
	plan := atomicTestPlan()
	plan.Profile = ""
	if _, err := plan.WithAtomicCapabilities([]AtomicCapability{valid}); err == nil {
		t.Fatal("unresolved profile accepted")
	}
	if _, err := (ResolvedScalarPlan{}).WithAtomicCapabilities(nil); err == nil {
		t.Fatal("unresolved target accepted")
	}
}

func atomicTestPlan() ResolvedScalarPlan {
	return ResolvedScalarPlan{TargetOS: "test-os", TargetArch: "test-arch", LLVMTriple: "test-triple", ABI: "test-abi", Profile: "hosted", PointerWidthBits: 64, Endianness: LittleEndian}
}

// atomicTestRequirement is a 64-bit unsigned CompareExchange from a named
// source type in an ordinary thread. The carrier and semantic spellings are
// test fixtures, not normative identity spellings (atomics.md § 37A(9)).
func atomicTestRequirement() AtomicRequirement {
	return AtomicRequirement{
		SemanticTypeID: "test:named:Count",
		Key: AtomicCapabilityKey{CarrierTypeID: "core:primitive:uint64", Representation: StorageRepresentation{Class: IntegerStorage, WidthBits: 64},
			SizeBytes: 8, AlignmentBytes: 8, Operation: "CompareExchange", Order: "Relaxed", FailureOrder: "SeqCst", Context: AtomicContextThread},
		Provenance: "source.sec:1:1",
	}
}
