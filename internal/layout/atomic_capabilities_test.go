package layout

import "testing"

// TestAtomicCapabilitiesExactRequirements prevents capability evidence from
// widening to another nominal type, layout, order or execution contract.
// Rules: rules/concurrency/atomics.md — §§24,37(1–5),39,47(3–4).
func TestAtomicCapabilitiesExactRequirements(t *testing.T) {
	request := atomicTestRequirement()
	supported := AtomicCapability{Requirement: request, Availability: AtomicSupported, Provenance: "test-target-contract", Reason: "verified test implementation", Implementation: AtomicImplementation{Kind: AtomicCompilerIntrinsic, Identity: "test-intrinsic"}}
	input := []AtomicCapability{supported}
	plan, err := atomicTestPlan().WithAtomicCapabilities(input)
	if err != nil {
		t.Fatal(err)
	}
	// The producer must not retain mutable access after the plan is frozen.
	input[0].Availability = AtomicUnsupported
	input[0].Requirement.TypeID = "other::Count"
	got, err := plan.AtomicCapability(request)
	if err != nil || got != supported {
		t.Fatal(got, err)
	}
	got.Availability = AtomicUnsupported
	if again, err := plan.AtomicCapability(request); err != nil || again != supported {
		t.Fatal(again, err)
	}
	for name, change := range map[string]func(*AtomicRequirement){
		"nominal identity": func(r *AtomicRequirement) { r.TypeID = "other::Count" },
		"generic identity": func(r *AtomicRequirement) { r.TypeID = "core::RawPtr[other::Count]" },
		"size":             func(r *AtomicRequirement) { r.SizeBytes = 16 },
		"alignment":        func(r *AtomicRequirement) { r.AlignmentBytes = 4 },
		"operation":        func(r *AtomicRequirement) { r.Operation = "Swap"; r.FailureOrder = "" },
		"success order":    func(r *AtomicRequirement) { r.Order = "Acquire" },
		"failure order":    func(r *AtomicRequirement) { r.FailureOrder = "Acquire" },
		"context":          func(r *AtomicRequirement) { r.ContextID = "isr::timer" },
		"no block":         func(r *AtomicRequirement) { r.NoBlocking = true },
		"interrupt safety": func(r *AtomicRequirement) { r.InterruptSafe = true },
	} {
		t.Run(name, func(t *testing.T) {
			other := request
			change(&other)
			fact, err := plan.AtomicCapability(other)
			if err != nil || fact.Availability != AtomicUnknown || fact.Requirement != other || fact.Provenance != "" {
				t.Fatal("capability leaked to a different request", fact, err)
			}
		})
	}
}

// TestAtomicCapabilitiesPlanBinding rejects cross-target/profile reuse rather
// than deriving support from architecture, pointer width or compiler host.
// Rules: rules/concurrency/atomics.md — §37(1–5);
// rules/platform/target_profiles.md — §31.
func TestAtomicCapabilitiesPlanBinding(t *testing.T) {
	request := atomicTestRequirement()
	plan, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{{Requirement: request, Availability: AtomicUnsupported, Provenance: "test-profile", Reason: "operation unavailable"}})
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

// TestAtomicCapabilitiesUnknownAndFence keeps missing evidence distinct from
// denial and represents fences without inventing contained storage or defaults.
// Rules: rules/concurrency/atomics.md — §§24–26,37,40.
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
	fence := AtomicRequirement{Operation: "Fence", Order: "SeqCst", ContextID: "thread::main"}
	plan, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{{Requirement: fence, Availability: AtomicSupported, Provenance: "test-fence", Implementation: AtomicImplementation{Kind: AtomicNativeInstruction, Identity: "test-fence"}}})
	if err != nil {
		t.Fatal(err)
	}
	if fact, err := plan.AtomicCapability(fence); err != nil || fact.Availability != AtomicSupported {
		t.Fatal(fact, err)
	}
	fence.Order = "Acquire"
	if fact, err := plan.AtomicCapability(fence); err != nil || fact.Availability != AtomicUnknown {
		t.Fatal("invented strengthening", fact, err)
	}
	if fact, err := plan.AtomicCapability(request); err != nil || fact.Availability != AtomicUnknown {
		t.Fatal("fence capability supplied storage operation", fact, err)
	}
}

// TestAtomicCapabilitiesRejectIncompleteFacts prevents incomplete layout,
// unresolved orders and duplicate evidence from entering the frozen table.
// Rules: rules/concurrency/atomics.md — §§24,37;
// rules/memory/layout.md — §5(1–5).
func TestAtomicCapabilitiesRejectIncompleteFacts(t *testing.T) {
	valid := AtomicCapability{Requirement: atomicTestRequirement(), Availability: AtomicSupported, Provenance: "test-proof", Implementation: AtomicImplementation{Kind: AtomicPlatformPrimitive, Identity: "test-primitive"}}
	for name, change := range map[string]func(*AtomicCapability){
		"type":                func(f *AtomicCapability) { f.Requirement.TypeID = "" },
		"size":                func(f *AtomicCapability) { f.Requirement.SizeBytes = 0 },
		"alignment":           func(f *AtomicCapability) { f.Requirement.AlignmentBytes = 0 },
		"operation":           func(f *AtomicCapability) { f.Requirement.Operation = "compare_exchange" },
		"order":               func(f *AtomicCapability) { f.Requirement.Order = "" },
		"order spelling":      func(f *AtomicCapability) { f.Requirement.Order = "acquire" },
		"failure order":       func(f *AtomicCapability) { f.Requirement.FailureOrder = "" },
		"extra failure order": func(f *AtomicCapability) { f.Requirement.Operation = "Load" },
		"context":             func(f *AtomicCapability) { f.Requirement.ContextID = "" },
		"provenance":          func(f *AtomicCapability) { f.Provenance = "" },
		"availability":        func(f *AtomicCapability) { f.Availability = "enabled" },
		"fence storage":       func(f *AtomicCapability) { f.Requirement.Operation = "Fence"; f.Requirement.FailureOrder = "" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := valid
			change(&bad)
			if _, err := atomicTestPlan().WithAtomicCapabilities([]AtomicCapability{bad}); err == nil {
				t.Fatal("accepted incomplete fact")
			}
			if bad.Requirement != valid.Requirement {
				if _, err := atomicTestPlan().AtomicCapability(bad.Requirement); err == nil {
					t.Fatal("accepted incomplete request")
				}
			}
		})
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

func atomicTestRequirement() AtomicRequirement {
	return AtomicRequirement{TypeID: "test::Count", SizeBytes: 8, AlignmentBytes: 8, Operation: "CompareExchange", Order: "Relaxed", FailureOrder: "SeqCst", ContextID: "thread::main"}
}
