package target

import (
	"testing"

	"sec/internal/layout"
)

// Every registered target remains Unknown without IPC producer evidence,
// regardless of architecture, hosted profile or ordinary atomic facilities.
// Rules: rules/concurrency/ipc.md §§118,122–124.
func TestRegistryDoesNotGuessIPCCapabilities(t *testing.T) {
	requests := []layout.IPCRequirement{{Family: layout.IPCPipes}, {Family: layout.IPCTypedMessages}, {Family: layout.IPCSharedMemory}, {Family: layout.IPCMutex}, {Family: layout.IPCSemaphore}, {Family: layout.IPCAtomicRepresentation, WidthBits: 64}}
	for _, definition := range Definitions() {
		plan, err := definition.ScalarPlan()
		if definition.PointerWidthBits == 0 {
			if err == nil {
				t.Fatal("unresolved target produced plan")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, req := range requests {
			fact, err := plan.IPCCapability(req)
			if err != nil || fact.Availability != layout.IPCUnknown {
				t.Fatal(definition.OS, definition.Arch, fact, err)
			}
		}
	}
}

// The real registry-to-plan boundary preserves exact frozen evidence while
// rejecting a profile change and leaving the compiler's original registry alone.
// Rules: rules/concurrency/ipc.md §118; rules/platform/target_profiles.md §§29–31.
func TestDefinitionPreservesIPCCapabilities(t *testing.T) {
	definition, ok := Find(Target{OS: "linux", Arch: "armv7"})
	if !ok {
		t.Fatal("missing test target")
	}
	base, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	fact := layout.IPCCapability{Requirement: layout.IPCRequirement{Family: layout.IPCAtomicRepresentation, WidthBits: 64}, Availability: layout.IPCUnsupported, Provenance: "test-profile-contract", Reason: "no complete process-shared implementation"}
	plan, err := base.WithIPCCapabilities([]layout.IPCCapability{fact})
	if err != nil {
		t.Fatal(err)
	}
	definition.IPC = plan.IPC
	projected, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	got, err := projected.IPCCapability(fact.Requirement)
	if err != nil || got != fact {
		t.Fatal(got, err)
	}
	definition.Profile = "different"
	if _, err := definition.ScalarPlan(); err == nil {
		t.Fatal("IPC evidence crossed profile boundary")
	}
	original, _ := Find(Target{OS: "linux", Arch: "armv7"})
	fresh, err := original.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	got, err = fresh.IPCCapability(fact.Requirement)
	if err != nil || got.Availability != layout.IPCUnknown {
		t.Fatal("local copy mutated registry", got, err)
	}
}
