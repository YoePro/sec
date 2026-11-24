package layout

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
)

// Family and width requests remain independent, and both producer inputs and
// query/inspection results are detached. Missing facts never imply IPC support.
// Rules: rules/concurrency/ipc.md §§118–124.
func TestIPCCapabilitiesExactAndImmutable(t *testing.T) {
	base := atomicTestPlan()
	families := []IPCFamily{IPCPipes, IPCTypedMessages, IPCSharedMemory, IPCMutex, IPCSemaphore}
	var input []IPCCapability
	for _, family := range families {
		input = append(input, IPCCapability{Requirement: IPCRequirement{Family: family}, Availability: IPCSupported, Provenance: "test-complete-family-contract"})
	}
	input = append(input, IPCCapability{Requirement: IPCRequirement{Family: IPCAtomicRepresentation, WidthBits: 64}, Availability: IPCSupported, Provenance: "test-lock-free-process-shared-contract"})
	expected := append([]IPCCapability(nil), input...)
	plan, err := base.WithIPCCapabilities(input)
	if err != nil {
		t.Fatal(err)
	}
	input[0].Availability = IPCUnsupported
	input[0].Requirement.Family = "changed"
	for _, want := range expected {
		got, err := plan.IPCCapability(want.Requirement)
		if err != nil || got != want {
			t.Fatal(got, err, want)
		}
		got.Availability = IPCUnsupported
		if again, _ := plan.IPCCapability(want.Requirement); again != want {
			t.Fatal("query aliases frozen table")
		}
	}
	for _, width := range []uint16{1, 8, 16, 32, 128, 256} {
		req := IPCRequirement{Family: IPCAtomicRepresentation, WidthBits: width}
		got, err := plan.IPCCapability(req)
		if err != nil || got.Availability != IPCUnknown || got.Requirement != req || got.Provenance != "" {
			t.Fatalf("width %d inherited support: %+v %v", width, got, err)
		}
	}
	facts := plan.IPC.Facts()
	facts[0].Provenance = "changed"
	if reflect.DeepEqual(facts, plan.IPC.Facts()) {
		t.Fatal("inspection aliases snapshot")
	}
	// An explicit negative fact is distinguishable from missing information.
	denial := IPCCapability{Requirement: IPCRequirement{Family: IPCPipes}, Availability: IPCUnsupported, Provenance: "test-profile", Reason: "no complete pipe provider"}
	negative, err := base.WithIPCCapabilities([]IPCCapability{denial})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := negative.IPCCapability(denial.Requirement)
	if got != denial {
		t.Fatal(got)
	}
	other, _ := negative.IPCCapability(IPCRequirement{Family: IPCTypedMessages})
	if other.Availability != IPCUnknown {
		t.Fatal("family denial leaked")
	}
}

// Binding covers every scalar/profile fact, including address domains. Neither
// matching pointer width nor an ordinary atomic fact permits cross-plan reuse.
// Rules: rules/concurrency/ipc.md §§118,122–124; rules/platform/target_profiles.md §§29–31.
func TestIPCCapabilitiesPlanBinding(t *testing.T) {
	base := atomicTestPlan()
	base.AddressDomains = []AddressDomain{{Identity: "physical", AddressBits: 64, UnitBits: 8, Linear: true}}
	plan, err := base.WithIPCCapabilities([]IPCCapability{{Requirement: IPCRequirement{Family: IPCPipes}, Availability: IPCSupported, Provenance: "test-contract"}})
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*ResolvedScalarPlan){
		"os": func(p *ResolvedScalarPlan) { p.TargetOS = "other" }, "arch": func(p *ResolvedScalarPlan) { p.TargetArch = "other" },
		"profile": func(p *ResolvedScalarPlan) { p.Profile = "other" }, "abi": func(p *ResolvedScalarPlan) { p.ABI = "other" },
		"triple": func(p *ResolvedScalarPlan) { p.LLVMTriple = "other" }, "width": func(p *ResolvedScalarPlan) { p.PointerWidthBits = 32 },
		"endian": func(p *ResolvedScalarPlan) { p.Endianness = BigEndian }, "cabi": func(p *ResolvedScalarPlan) { p.CABI = CModelSysVX8664 },
		"reference": func(p *ResolvedScalarPlan) {
			p.References.LogicalEpochBits = 32
			p.References.ShortWidthProof = "test-proof"
		},
		"domains": func(p *ResolvedScalarPlan) {
			p.AddressDomains = []AddressDomain{{Identity: "other", AddressBits: 64, UnitBits: 8, Linear: true}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := plan
			change(&p)
			if p.Validate() == nil {
				t.Fatal("foreign plan accepted IPC snapshot")
			}
			if _, err := p.IPCCapability(IPCRequirement{Family: IPCPipes}); err == nil {
				t.Fatal("foreign snapshot queried")
			}
		})
	}
	// A shared input address-domain slice cannot rewrite the frozen binding.
	base.AddressDomains[0].AddressBits = 32
	if plan.Validate() == nil {
		t.Fatal("producer mutation silently rewrote frozen plan binding")
	}
}

// Independent tables compose in either order and retain exact ordinary atomic
// requirements; one table never supplies missing evidence for the other.
// Rules: rules/concurrency/ipc.md §§7.3,118; rules/concurrency/atomics.md §37.
func TestIPCCapabilitiesComposeWithAtomics(t *testing.T) {
	base := atomicTestPlan()
	request := atomicTestRequirement()
	atomic := AtomicCapability{Key: request.Key, Availability: AtomicSupported, Provenance: "test-ordinary-atomic", Implementation: AtomicImplementation{Kind: AtomicCompilerIntrinsic, Identity: "ordinary-only-intrinsic", LockFree: AtomicPropertyYes, Blocking: AtomicPropertyNo}}
	ipc := IPCCapability{Requirement: IPCRequirement{Family: IPCAtomicRepresentation, WidthBits: 64}, Availability: IPCSupported, Provenance: "separate-process-shared-contract"}
	a, err := base.WithAtomicCapabilities([]AtomicCapability{atomic})
	if err != nil {
		t.Fatal(err)
	}
	absent, err := a.IPCCapability(ipc.Requirement)
	if err != nil || absent.Availability != IPCUnknown {
		t.Fatal("ordinary atomic implies IPC", absent, err)
	}
	a, err = a.WithIPCCapabilities([]IPCCapability{ipc})
	if err != nil {
		t.Fatal(err)
	}
	b, err := base.WithIPCCapabilities([]IPCCapability{ipc})
	if err != nil {
		t.Fatal(err)
	}
	missing, err := b.AtomicCapability(request)
	if err != nil || missing.Availability != AtomicUnknown {
		t.Fatal("IPC implies ordinary atomic", missing, err)
	}
	b, err = b.WithAtomicCapabilities([]AtomicCapability{atomic})
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []ResolvedScalarPlan{a, b} {
		if err := plan.Validate(); err != nil {
			t.Fatal(err)
		}
		got, err := plan.AtomicCapability(request)
		if err != nil || got != atomic {
			t.Fatal(got, err)
		}
		ip, err := plan.IPCCapability(ipc.Requirement)
		if err != nil || ip != ipc {
			t.Fatal(ip, err)
		}
	}
	if a.Fingerprints() != b.Fingerprints() {
		t.Fatal("table registration order changes plan meaning")
	}
}

// Reject missing/invalid qualifiers, duplicate keys, unsupported vocabulary and
// unsupported claims without provenance instead of silently accepting booleans.
// Rules: rules/concurrency/ipc.md §§118,121–124.
func TestIPCCapabilitiesInvalid(t *testing.T) {
	base := atomicTestPlan()
	for _, request := range []IPCRequirement{{}, {Family: "IPC"}, {Family: "SharedValue"}, {Family: IPCAtomicRepresentation}, {Family: IPCPipes, WidthBits: 64}} {
		if _, err := base.IPCCapability(request); err == nil {
			t.Fatalf("invalid request accepted: %+v", request)
		}
		if _, err := base.WithIPCCapabilities([]IPCCapability{{Requirement: request, Availability: IPCUnknown}}); err == nil {
			t.Fatal("invalid fact frozen")
		}
	}
	req := IPCRequirement{Family: IPCPipes}
	for _, fact := range []IPCCapability{
		{Requirement: req}, {Requirement: req, Availability: "native"},
		{Requirement: req, Availability: IPCSupported},
		{Requirement: req, Availability: IPCUnsupported, Provenance: "test"},
		{Requirement: req, Availability: IPCUnsupported, Reason: "no provider"},
	} {
		if _, err := base.WithIPCCapabilities([]IPCCapability{fact}); err == nil {
			t.Fatalf("invalid evidence accepted: %+v", fact)
		}
	}
	fact := IPCCapability{Requirement: req, Availability: IPCUnknown}
	if _, err := base.WithIPCCapabilities([]IPCCapability{fact, fact}); err == nil {
		t.Fatal("duplicate fact accepted")
	}
	base.Profile = ""
	if _, err := base.WithIPCCapabilities(nil); err == nil {
		t.Fatal("unresolved profile frozen")
	}
	if _, err := (ResolvedScalarPlan{}).IPCCapability(req); err == nil {
		t.Fatal("unresolved target queried")
	}
}

// Capability meaning participates in memory/runtime/execution fingerprints,
// excluding registration order, explanatory text and explicit unknown entries.
// Rules: rules/platform/platform_model.md §23; rules/concurrency/ipc.md §118.
func TestIPCCapabilityFingerprints(t *testing.T) {
	base := atomicTestPlan()
	original := base.Fingerprints()
	facts := []IPCCapability{
		{Requirement: IPCRequirement{Family: IPCPipes}, Availability: IPCSupported, Provenance: "contract-a"},
		{Requirement: IPCRequirement{Family: IPCAtomicRepresentation, WidthBits: 64}, Availability: IPCUnsupported, Provenance: "contract-b", Reason: "no provider"},
	}
	plan, err := base.WithIPCCapabilities(facts)
	if err != nil {
		t.Fatal(err)
	}
	first := plan.Fingerprints()
	if first.Plan == original.Plan || first.Memory == original.Memory || first.Runtime == original.Runtime || first.Execution == original.Execution {
		t.Fatal("IPC meaning absent from fingerprints")
	}
	if first.Architecture != original.Architecture || first.Layout != original.Layout || first.ABI != original.ABI || first.Link != original.Link {
		t.Fatal("unrelated domains changed")
	}
	facts[0], facts[1] = facts[1], facts[0]
	facts[0].Provenance = "different-evidence"
	facts[0].Reason = "different-explanation"
	again, err := base.WithIPCCapabilities(facts)
	if err != nil || again.Fingerprints() != first {
		t.Fatal("metadata/order changes semantic fingerprints", err)
	}
	a, _ := json.Marshal(plan.IPC)
	b, _ := json.Marshal(again.IPC)
	if len(a) == 0 || len(b) == 0 {
		t.Fatal("inspection empty")
	}
	facts[0].Requirement.WidthBits = 32
	changed, err := base.WithIPCCapabilities(facts)
	if err != nil || changed.Fingerprints().Plan == first.Plan {
		t.Fatal("width erased from fingerprint", err)
	}
	unknown, err := base.WithIPCCapabilities([]IPCCapability{{Requirement: IPCRequirement{Family: IPCMutex}, Availability: IPCUnknown, Provenance: "unresolved target", Reason: "missing evidence"}})
	if err != nil || unknown.Fingerprints() != original {
		t.Fatal("explicit unknown became semantic capability evidence", err)
	}
	// Inspection is deterministic and includes actual family/width evidence.
	repeated, _ := json.Marshal(plan.IPC)
	if string(repeated) != string(a) {
		t.Fatal("nondeterministic serialization")
	}
}

// Concurrent consumers may inspect and mutate their returned copies while the
// selected target/profile snapshot and its dependency identity remain frozen.
// Rules: rules/concurrency/ipc.md §118; rules/platform/target_profiles.md §23.
func TestIPCCapabilitiesConcurrentQueries(t *testing.T) {
	requirement := IPCRequirement{Family: IPCPipes}
	plan, err := atomicTestPlan().WithIPCCapabilities([]IPCCapability{{Requirement: requirement, Availability: IPCSupported, Provenance: "test-contract"}})
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := plan.Fingerprints()
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for round := 0; round < 20; round++ {
				fact, err := plan.IPCCapability(requirement)
				if err != nil || fact.Availability != IPCSupported {
					t.Errorf("concurrent query: %+v %v", fact, err)
					return
				}
				copies := plan.IPC.Facts()
				copies[0].Availability = IPCUnsupported
				if plan.Fingerprints() != fingerprint {
					t.Error("consumer mutated frozen semantics")
					return
				}
			}
		}()
	}
	workers.Wait()
}
