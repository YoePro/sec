package layout

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMutexCapabilities freezes independent exact-mode evidence, distinguishes
// unknown from unsupported and prevents reuse across target/profile changes.
// Rules: rules/concurrency/mutex.md §§20–27,60,63; rules/platform/platform_model.md §23.
func TestMutexCapabilities(t *testing.T) {
	base := atomicTestPlan()
	input := []MutexCapability{{Acquisition: MutexTry, Availability: MutexSupported, Provenance: "verified-test-contract"}, {Acquisition: MutexBlocking, Availability: MutexUnsupported, Provenance: "test-profile", Reason: "no waiting primitive"}}
	plan, err := base.WithMutexCapabilities(input)
	if err != nil {
		t.Fatal(err)
	}
	input[0].Availability = MutexUnsupported
	facts := plan.Mutex.Facts()
	facts[0].Reason = "mutated"
	for _, mode := range []MutexAcquisition{MutexBlocking, MutexTry, MutexTimeout, MutexContext} {
		got, err := plan.MutexCapability(mode)
		want := MutexUnknown
		if mode == MutexTry {
			want = MutexSupported
		}
		if mode == MutexBlocking {
			want = MutexUnsupported
		}
		if err != nil || got.Availability != want || got.Reason == "mutated" {
			t.Fatal(got, err)
		}
	}
	encoded, err := json.Marshal(plan.Mutex)
	if err != nil || !strings.Contains(string(encoded), "verified-test-contract") {
		t.Fatal(string(encoded), err)
	}
	for _, change := range []func(*ResolvedScalarPlan){func(p *ResolvedScalarPlan) { p.Profile = "other" }, func(p *ResolvedScalarPlan) { p.TargetOS = "other" }, func(p *ResolvedScalarPlan) { p.PointerWidthBits = 32 }} {
		other := plan
		change(&other)
		if _, err := other.MutexCapability(MutexTry); err == nil {
			t.Fatal("foreign plan retained mutex proof")
		}
	}
	for _, bad := range [][]MutexCapability{
		{{Acquisition: "bad", Availability: MutexUnknown}},
		{{Acquisition: MutexTry, Availability: "probably"}},
		{{Acquisition: MutexTry, Availability: MutexSupported}},
		{{Acquisition: MutexTry, Availability: MutexUnsupported, Provenance: "p"}},
		{{Acquisition: MutexTry, Availability: MutexUnknown}, {Acquisition: MutexTry, Availability: MutexUnknown}},
	} {
		if _, err := base.WithMutexCapabilities(bad); err == nil {
			t.Fatal("accepted malformed evidence", bad)
		}
	}
	if _, err := plan.MutexCapability("bad"); err == nil {
		t.Fatal("unknown acquisition accepted")
	}
	fresh, _ := base.MutexCapability(MutexTry)
	if fresh.Availability != MutexUnknown {
		t.Fatal("producer mutation escaped")
	}
}

// TestMutexCapabilityFingerprints binds dependent memory/runtime/execution reuse
// to meaning, excluding provenance text and Unknown inventory placeholders.
// Rules: rules/platform/platform_model.md §23; rules/concurrency/mutex.md §60.
func TestMutexCapabilityFingerprints(t *testing.T) {
	base := atomicTestPlan()
	before := base.Fingerprints()
	fact := MutexCapability{Acquisition: MutexTry, Availability: MutexSupported, Provenance: "first"}
	selected, err := base.WithMutexCapabilities([]MutexCapability{fact})
	if err != nil {
		t.Fatal(err)
	}
	got := selected.Fingerprints()
	if got.Memory == before.Memory || got.Runtime == before.Runtime || got.Execution == before.Execution || got.Plan == before.Plan {
		t.Fatal("mutex dependency missing")
	}
	if got.Layout != before.Layout || got.ABI != before.ABI || got.Architecture != before.Architecture {
		t.Fatal("mutex changed unrelated scalar identity")
	}
	fact.Provenance = "different evidence"
	other, err := base.WithMutexCapabilities([]MutexCapability{fact})
	if err != nil || other.Fingerprints() != got {
		t.Fatal("provenance changed semantic reuse", err)
	}
	unknown, err := base.WithMutexCapabilities([]MutexCapability{{Acquisition: MutexTry, Availability: MutexUnknown}})
	if err != nil || unknown.Fingerprints() != before {
		t.Fatal("Unknown changed semantic reuse", err)
	}
	// Independent tables can be frozen in either order without nesting proofs.
	ipc, err := selected.WithIPCCapabilities([]IPCCapability{{Requirement: IPCRequirement{Family: IPCMutex}, Availability: IPCSupported, Provenance: "test IPC contract"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ipc.Validate(); err != nil {
		t.Fatal(err)
	}
}
