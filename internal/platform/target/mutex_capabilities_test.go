package target

import (
	"sec/internal/layout"
	"testing"
)

// TestMutexTargetProjection never derives ordinary lock support from the host
// registry and preserves only explicit target/profile producer evidence.
// Rules: rules/concurrency/mutex.md §§27,60,63; rules/platform/platform_model.md §9.
func TestMutexTargetProjection(t *testing.T) {
	for _, definition := range Definitions() {
		if definition.PointerWidthBits == 0 {
			continue
		}
		plan, err := definition.ScalarPlan()
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []layout.MutexAcquisition{layout.MutexBlocking, layout.MutexTry, layout.MutexTimeout, layout.MutexContext} {
			got, err := plan.MutexCapability(mode)
			if err != nil || got.Availability != layout.MutexUnknown {
				t.Fatal(definition.OS, definition.Arch, got, err)
			}
		}
	}
	definition, ok := Find(Target{OS: "linux", Arch: "armv7"})
	if !ok {
		t.Fatal("missing target")
	}
	base, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	fact := layout.MutexCapability{Acquisition: layout.MutexContext, Availability: layout.MutexUnsupported, Provenance: "test target contract", Reason: "no conforming context wait"}
	selected, err := base.WithMutexCapabilities([]layout.MutexCapability{fact})
	if err != nil {
		t.Fatal(err)
	}
	definition.Mutex = selected.Mutex
	projected, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := projected.MutexCapability(layout.MutexContext); err != nil || got != fact {
		t.Fatal(got, err)
	}
	definition.Profile = "other"
	if _, err := definition.ScalarPlan(); err == nil {
		t.Fatal("foreign profile reused mutex contract")
	}
}
