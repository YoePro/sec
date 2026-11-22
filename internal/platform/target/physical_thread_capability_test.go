package target

import (
	"sec/internal/layout"
	"testing"
)

// TestPhysicalThreadTargetProjection requires explicit target evidence rather
// than guessing from the compiler host, OS family, architecture or task support.
// Rules: rules/concurrency/cancellation.md §24; rules/platform/platform_model.md §9.
func TestPhysicalThreadTargetProjection(t *testing.T) {
	for _, definition := range Definitions() {
		if definition.PointerWidthBits == 0 {
			continue
		}
		plan, err := definition.ScalarPlan()
		if err != nil {
			t.Fatal(err)
		}
		got, err := plan.PhysicalThreadCapability()
		if err != nil || got.Availability != layout.PhysicalThreadUnknown {
			t.Fatal(definition.OS, definition.Arch, got, err)
		}
	}
	definition, ok := Find(Target{OS: "linux", Arch: "armv7"})
	if !ok {
		t.Fatal("missing test target")
	}
	base, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	fact := layout.PhysicalThreadCapability{Availability: layout.PhysicalThreadUnsupported, Provenance: "test profile", Reason: "physical thread identity unavailable"}
	plan, err := base.WithPhysicalThreadCapability(fact)
	if err != nil {
		t.Fatal(err)
	}
	definition.PhysicalThreads = plan.PhysicalThreads
	projected, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := projected.PhysicalThreadCapability(); err != nil || got != fact {
		t.Fatal(got, err)
	}
	definition.OS = "other"
	if _, err := definition.ScalarPlan(); err == nil {
		t.Fatal("foreign target retained physical-thread evidence")
	}
}
